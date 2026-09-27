package linker

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/PeterStoica/chatwire/internal/linkflow"
	"github.com/PeterStoica/chatwire/internal/pairing"
)

const qrSessions = 6

type Phase uint8

const (
	Unlinked Phase = iota
	Starting
	ShowingCode
	ShowingQR
	Linked
	Expired
	Failed
	LoggedOut
)

func (p Phase) String() string {
	switch p {
	case Starting:
		return "starting"
	case ShowingCode:
		return "waiting for the code to be entered on the phone"
	case ShowingQR:
		return "waiting for the QR code to be scanned"
	case Linked:
		return "linked"
	case Expired:
		return "expired"
	case Failed:
		return "failed"
	case LoggedOut:
		return "logged out"
	default:
		return "not linked"
	}
}

func (p Phase) InFlight() bool {
	return p == Starting || p == ShowingCode || p == ShowingQR
}

type Status struct {
	Phase   Phase
	Phone   pairing.Phone
	Code    string
	Renewed bool
	QR      string
	Account pairing.Account
	Err     error
}

func (s Status) settled() bool {
	return s.Phase != Starting
}

type Linker struct {
	cfg      linkflow.Config
	onLinked func(linkflow.Linked)
	keepQR   func() bool

	starting sync.Mutex
	mu       sync.Mutex
	status   Status
	changed  chan struct{}
	cancel   context.CancelFunc
	running  chan struct{}
}

func New(cfg linkflow.Config, linked *pairing.Account) *Linker {
	l := &Linker{cfg: cfg, changed: make(chan struct{})}
	if linked != nil {
		l.status = Status{Phase: Linked, Account: *linked}
	}
	return l
}

func (l *Linker) WhenLinked(fn func(linkflow.Linked)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.onLinked = fn
}

func (l *Linker) KeepQRWhile(fn func() bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.keepQR = fn
}

func (l *Linker) keepingQR() bool {
	l.mu.Lock()
	keep := l.keepQR
	l.mu.Unlock()
	return keep != nil && keep()
}

func (l *Linker) LoggedOut(err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.setLocked(Status{Phase: LoggedOut, Err: err})
}

func (l *Linker) Status() Status {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.status
}

func (l *Linker) Start(ctx context.Context, phone string) (Status, error) {
	parsed, err := parse(phone)
	if err != nil {
		return l.Status(), err
	}
	l.starting.Lock()
	defer l.starting.Unlock()
	if status := l.Status(); status.Phase.InFlight() && status.Phone == parsed {
		return l.Await(ctx, Status.settled)
	}
	l.Close()
	if status := l.Status(); status.Phase == Linked {
		return status, nil
	}
	l.begin(ctx, parsed)
	return l.Await(ctx, Status.settled)
}

func parse(phone string) (pairing.Phone, error) {
	if phone == "" {
		return "", nil
	}
	return pairing.ParsePhone(phone)
}

func (l *Linker) begin(parent context.Context, phone pairing.Phone) {
	ctx, cancel := context.WithCancel(context.WithoutCancel(parent))
	running := make(chan struct{})
	l.mu.Lock()
	l.cancel, l.running = cancel, running
	l.setLocked(Status{Phase: Starting, Phone: phone})
	l.mu.Unlock()
	cfg := l.cfg
	cfg.Phone = phone
	cfg.ShowCode = func(code string) { l.show(ctx, Status{Phase: ShowingCode, Phone: phone, Code: code}) }
	cfg.ShowQR = func(data string) { l.show(ctx, Status{Phase: ShowingQR, Phone: phone, QR: data}) }
	go func() {
		defer close(running)
		defer cancel()
		for session := 1; ; session++ {
			linked, err := linkflow.Link(ctx, cfg)
			if err != nil && ctx.Err() != nil {
				return
			}
			if errors.Is(err, linkflow.ErrQRExpired) && session < qrSessions && l.keepingQR() {
				l.show(ctx, Status{Phase: Starting, Phone: phone})
				continue
			}
			l.finish(phone, linked, err)
			return
		}
	}()
}

func (l *Linker) show(ctx context.Context, status Status) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if ctx.Err() == nil {
		status.Renewed = status.Code != "" && l.status.Phase == ShowingCode && l.status.Code != status.Code
		l.setLocked(status)
	}
}

func (l *Linker) finish(phone pairing.Phone, linked linkflow.Linked, err error) {
	status := Status{Phase: Linked, Phone: phone, Account: linked.Account}
	switch {
	case errors.Is(err, linkflow.ErrQRExpired), errors.Is(err, linkflow.ErrCodeExpired), errors.Is(err, linkflow.ErrEnded):
		status = Status{Phase: Expired, Phone: phone, Err: err}
	case err != nil:
		status = Status{Phase: Failed, Phone: phone, Err: err}
	}
	l.mu.Lock()
	l.setLocked(status)
	onLinked := l.onLinked
	l.mu.Unlock()
	if status.Phase == Linked && onLinked != nil {
		onLinked(linked)
	}
}

func (l *Linker) setLocked(status Status) {
	l.status = status
	close(l.changed)
	l.changed = make(chan struct{})
}

func (l *Linker) Await(ctx context.Context, done func(Status) bool) (Status, error) {
	for {
		l.mu.Lock()
		status, changed := l.status, l.changed
		l.mu.Unlock()
		if done(status) {
			return status, nil
		}
		select {
		case <-changed:
		case <-ctx.Done():
			return status, fmt.Errorf("linker: %w", ctx.Err())
		}
	}
}

func (l *Linker) Close() {
	l.mu.Lock()
	cancel, running := l.cancel, l.running
	l.mu.Unlock()
	if cancel != nil {
		cancel()
		<-running
	}
}
