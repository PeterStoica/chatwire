package linkflow

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/PeterStoica/chatwire/internal/curve"
	"github.com/PeterStoica/chatwire/internal/device"
	"github.com/PeterStoica/chatwire/internal/frame"
	"github.com/PeterStoica/chatwire/internal/handshake"
	"github.com/PeterStoica/chatwire/internal/live"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/pairing"
	"github.com/PeterStoica/chatwire/internal/signon"
)

const (
	firstQRLifetime = 60 * time.Second
	nextQRLifetime  = 20 * time.Second
	tagStreamError  = "stream:error"
)

var (
	ErrQRExpired     = errors.New("linkflow: every QR code expired before it was scanned")
	ErrLoginRejected = errors.New("linkflow: WhatsApp rejected the login")
	ErrLoggedOut     = errors.New("linkflow: WhatsApp logged this device out")
	ErrReplaced      = errors.New("linkflow: another connection took over this linked device")
	ErrEnded         = errors.New("linkflow: connection ended before pairing finished")
	ErrBanned        = errors.New("linkflow: WhatsApp temporarily banned this account")
	ErrOutdated      = errors.New("linkflow: WhatsApp says this client version is outdated")
	ErrClient        = errors.New("linkflow: WhatsApp refused this client")
)

type Ban struct {
	Code int
	For  time.Duration
}

func (b Ban) Error() string {
	return fmt.Sprintf("%v (reason %d, for %s)", ErrBanned, b.Code, b.For)
}

func (b Ban) Is(target error) bool {
	return target == ErrBanned
}

type Config struct {
	Dial       func(ctx context.Context) (frame.MessageConn, error)
	Dictionary node.Dictionary
	Root       curve.PublicKey
	Version    func() signon.Version
	Random     io.Reader
	Now        func() time.Time
	Phone      pairing.Phone
	ShowQR     func(data string)
	ShowCode   func(code string)
	Save       func(Linked) error
	KeepAlive  time.Duration
}

type Linked struct {
	Identity  device.Stored
	AdvSecret []byte
	Account   pairing.Account
}

type link struct {
	cfg       Config
	identity  device.Identity
	companion pairing.Companion
	session   *live.Session
	code      *pairing.CodeRequest
	qr        *time.Timer
	refs      [][]byte
	shown     int
	qrTick    chan struct{}
}

func Link(ctx context.Context, cfg Config) (Linked, error) {
	identity, err := device.New(cfg.Random)
	if err != nil {
		return Linked{}, err
	}
	secret, err := pairing.NewAdvSecret(cfg.Random)
	if err != nil {
		return Linked{}, err
	}
	l := &link{
		cfg: cfg, identity: identity, qrTick: make(chan struct{}, 1),
		companion: pairing.Companion{Noise: identity.NoiseKey(), Identity: identity.IdentityKey(), AdvSecret: secret},
	}
	registration := signon.EncodeRegistration(cfg.Version(), identity.Registration(device.Props()))
	conn, err := connect(ctx, cfg, identity, registration)
	if err != nil {
		return Linked{}, err
	}
	l.session, err = live.Start(ctx, conn, cfg.Dictionary, cfg.Random)
	if err != nil {
		_ = conn.Close()
		return Linked{}, err
	}
	account, err := l.pair(ctx)
	_ = conn.Close()
	if err != nil {
		return Linked{}, err
	}
	linked := Linked{Identity: identity.Store(), AdvSecret: l.companion.AdvSecret, Account: account}
	if err := cfg.Save(linked); err != nil {
		return linked, fmt.Errorf("linkflow: save: %w", err)
	}
	return linked, Login(ctx, cfg, linked)
}

func connect(ctx context.Context, cfg Config, identity device.Identity, payload []byte) (*handshake.Conn, error) {
	messages, err := cfg.Dial(ctx)
	if err != nil {
		return nil, err
	}
	conn, err := handshake.Initiate(ctx, messages, handshake.Config{
		DictVersion: cfg.Dictionary.Version(), Root: cfg.Root, Static: identity.NoiseKey(),
		Payload: payload, Random: cfg.Random, Now: cfg.Now,
	})
	if err != nil {
		_ = messages.Close()
		return nil, err
	}
	return conn, nil
}

func (l *link) pair(ctx context.Context) (pairing.Account, error) {
	defer func() {
		if l.qr != nil {
			l.qr.Stop()
		}
	}()
	var account pairing.Account
	paired := false
	for {
		select {
		case <-l.qrTick:
			if !l.showNextQR() {
				return pairing.Account{}, ErrQRExpired
			}
		case <-ctx.Done():
			return pairing.Account{}, ctx.Err()
		case n, open := <-l.session.Events():
			if !open {
				if paired {
					return account, nil
				}
				return pairing.Account{}, fmt.Errorf("%w: %w", ErrEnded, l.session.Err())
			}
			done, err := l.handle(ctx, n, &account, &paired)
			if err != nil || done {
				return account, err
			}
		}
	}
}

func (l *link) handle(ctx context.Context, n node.Node, account *pairing.Account, paired *bool) (bool, error) {
	kind, _ := n.Attr("type").Text()
	switch {
	case n.Tag == tagStreamError:
		code, _ := n.Attr("code").Text()
		if *paired && code == "515" {
			return true, nil
		}
		return false, fmt.Errorf("linkflow: stream error %s", n)
	case n.Tag == "iq" && hasChild(n, "pair-device"):
		if err := l.session.Send(ctx, pairing.Ack(n)); err != nil {
			return false, err
		}
		return false, l.offer(ctx, pairing.Refs(n))
	case n.Tag == "notification" && kind == "link_code_companion_reg":
		if err := l.session.Send(ctx, live.Ack(n)); err != nil {
			return false, err
		}
		return false, l.finishCode(ctx, n)
	case n.Tag == "iq" && hasChild(n, "pair-success"):
		reply, linked, err := pairing.HandlePairSuccess(n, l.companion, l.cfg.Random)
		if sendErr := l.session.Send(ctx, reply); sendErr != nil {
			return false, errors.Join(err, sendErr)
		}
		if err != nil {
			return false, err
		}
		*account, *paired = linked, true
		return false, nil
	default:
		return false, nil
	}
}

func (l *link) offer(ctx context.Context, refs [][]byte) error {
	if l.cfg.Phone == "" {
		l.refs, l.shown = refs, 0
		if !l.showNextQR() {
			return ErrQRExpired
		}
		return nil
	}
	if l.code != nil {
		return nil
	}
	request, hello, err := pairing.StartCode(l.cfg.Random, l.cfg.Phone, l.companion, pairing.ClientOtherWeb, "Chatwire")
	if err != nil {
		return err
	}
	response, err := l.session.Query(ctx, hello)
	if err != nil {
		return err
	}
	if err := request.AcceptRef(response); err != nil {
		return err
	}
	l.code = request
	l.cfg.ShowCode(request.Display())
	return nil
}

func (l *link) showNextQR() bool {
	if l.shown >= len(l.refs) {
		return false
	}
	l.cfg.ShowQR(pairing.QRData(l.refs[l.shown], l.companion, pairing.ClientOtherWeb))
	lifetime := nextQRLifetime
	if l.shown == 0 {
		lifetime = firstQRLifetime
	}
	l.shown++
	if l.qr != nil {
		l.qr.Stop()
	}
	l.qr = time.AfterFunc(lifetime, func() {
		select {
		case l.qrTick <- struct{}{}:
		default:
		}
	})
	return true
}

func (l *link) finishCode(ctx context.Context, notification node.Node) error {
	if l.code == nil {
		return fmt.Errorf("linkflow: code notification without a pending code")
	}
	finish, secret, err := l.code.Finish(notification, l.companion, l.cfg.Random)
	if err != nil {
		return err
	}
	l.companion.AdvSecret = secret
	_, err = l.session.Query(ctx, finish)
	return err
}

type Online struct {
	Session *live.Session
	Success node.Node
	conn    *handshake.Conn
}

func (o *Online) Close() error {
	return o.conn.Close()
}

func Login(ctx context.Context, cfg Config, linked Linked) error {
	online, err := Open(ctx, cfg, linked)
	if err != nil {
		return err
	}
	return online.Close()
}

func Open(ctx context.Context, cfg Config, linked Linked) (*Online, error) {
	identity, err := device.Restore(linked.Identity)
	if err != nil {
		return nil, err
	}
	username, err := strconv.ParseUint(linked.Account.JID.User, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("linkflow: account user %q: %w", linked.Account.JID.User, err)
	}
	payload := signon.EncodeLogin(cfg.Version(), signon.Login{Username: username, Device: uint32(linked.Account.JID.Device), OS: device.Props().Version})
	conn, err := connect(ctx, cfg, identity, payload)
	if err != nil {
		return nil, err
	}
	session, err := live.Start(ctx, conn, cfg.Dictionary, cfg.Random)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	for n := range session.Events() {
		switch n.Tag {
		case "success":
			return &Online{Session: session, Success: n, conn: conn}, nil
		case "failure", tagStreamError:
			_ = conn.Close()
			if cause := Classify(n, nil); cause != nil {
				return nil, fmt.Errorf("%w: %w: %s", ErrLoginRejected, cause, n)
			}
			return nil, fmt.Errorf("%w: %s", ErrLoginRejected, n)
		}
	}
	_ = conn.Close()
	return nil, fmt.Errorf("%w: %w", ErrEnded, session.Err())
}

func Classify(n node.Node, otherwise error) error {
	code, _ := n.Attr("code").Text()
	reason, _ := n.Attr("reason").Text()
	conflict, _ := n.Child("conflict")
	kind, _ := conflict.Attr("type").Text()
	failure := n.Tag == "failure"
	switch {
	case failure && reason == "402":
		ban := Ban{}
		ban.Code, _ = strconv.Atoi(code)
		if raw, ok := n.Attr("expire").Text(); ok {
			seconds, _ := strconv.Atoi(raw)
			ban.For = time.Duration(max(seconds, 0)) * time.Second
		}
		return ban
	case failure && reason == "405":
		return ErrOutdated
	case failure && reason == "409":
		return ErrClient
	case failure && (reason == "401" || reason == "403" || reason == "406" || reason == "411"),
		n.Tag == tagStreamError && (code == "401" || kind == "device_removed"):
		return loggedOut(n)
	case n.Tag == tagStreamError && kind == "replaced":
		return ErrReplaced
	}
	return otherwise
}

func loggedOut(n node.Node) error {
	header, _ := n.Attr("logout_message_header").Text()
	subtext, _ := n.Attr("logout_message_subtext").Text()
	if message := strings.TrimSpace(header + " " + subtext); message != "" {
		return fmt.Errorf("%w: %s", ErrLoggedOut, message)
	}
	return ErrLoggedOut
}

func hasChild(n node.Node, tag string) bool {
	_, ok := n.Child(tag)
	return ok
}
