package linkpage

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"rsc.io/qr"

	"github.com/PeterStoica/chatwire/internal/linker"
)

const (
	readHeaderTimeout = 5 * time.Second
	watchedFor        = 5 * time.Second
	tokenBytes        = 16
)

var ErrToken = errors.New("linkpage: no randomness for the page address")

type Linker interface {
	Status() linker.Status
	Start(ctx context.Context, phone string) (linker.Status, error)
}

type Page struct {
	URL    string
	l      Linker
	life   context.Context
	server *http.Server
	host   string
	mu     sync.Mutex
	qr     string
	grid   []byte
	polled time.Time
	seen   time.Time
}

type State struct {
	Phase   string `json:"phase"`
	QR      string `json:"qr,omitempty"`
	Code    string `json:"code,omitempty"`
	Account string `json:"account,omitempty"`
	Problem string `json:"problem,omitempty"`
}

func Start(ctx context.Context, l Linker, random io.Reader) (*Page, error) {
	token := make([]byte, tokenBytes)
	if _, err := io.ReadFull(random, token); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrToken, err)
	}
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("linkpage: listen: %w", err)
	}
	base := "/" + hex.EncodeToString(token) + "/"
	p := &Page{l: l, life: context.WithoutCancel(ctx), host: listener.Addr().String()}
	p.URL = "http://" + p.host + base
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+base+"{$}", p.index)
	mux.HandleFunc("GET "+base+"state", p.state)
	mux.HandleFunc("GET "+base+"qr.json", p.modules)
	mux.HandleFunc("POST "+base+"again", p.again)
	p.server = &http.Server{Handler: p.local(mux), ReadHeaderTimeout: readHeaderTimeout}
	go func() { _ = p.server.Serve(listener) }()
	return p, nil
}

func (p *Page) Close() error {
	return p.server.Close()
}

func (p *Page) Open() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return time.Since(p.polled) < watchedFor
}

func (p *Page) Seen() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return time.Since(p.seen) < watchedFor
}

func (p *Page) local(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != p.host {
			http.Error(w, "this page only answers on "+p.host, http.StatusMisdirectedRequest)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func (p *Page) State() State {
	st := p.l.Status()
	out := State{Phase: phase(st.Phase)}
	switch st.Phase {
	case linker.ShowingQR:
		out.QR = qrID(st.QR)
	case linker.ShowingCode:
		out.Code = st.Code
	case linker.Linked:
		out.Account = "+" + st.Account.JID.User
	case linker.Failed, linker.LoggedOut:
		if st.Err != nil {
			out.Problem = st.Err.Error()
		}
	default:
	}
	return out
}

func phase(p linker.Phase) string {
	switch p {
	case linker.Starting:
		return "starting"
	case linker.ShowingQR:
		return "qr"
	case linker.ShowingCode:
		return "code"
	case linker.Linked:
		return "linked"
	case linker.Expired:
		return "expired"
	case linker.Failed:
		return "failed"
	case linker.LoggedOut:
		return "logged_out"
	default:
		return "idle"
	}
}

func (p *Page) state(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	p.mu.Lock()
	p.polled = now
	if r.URL.Query().Get("visible") == "1" {
		p.seen = now
	}
	p.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.MarshalWrite(w, p.State())
}

type Grid struct {
	ID   string   `json:"id"`
	Rows []string `json:"rows"`
}

func (p *Page) modules(w http.ResponseWriter, _ *http.Request) {
	st := p.l.Status()
	if st.Phase != linker.ShowingQR || st.QR == "" {
		http.NotFound(w, nil)
		return
	}
	grid, err := p.render(st.QR)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(grid)
}

func (p *Page) render(data string) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if data == p.qr {
		return p.grid, nil
	}
	code, err := qr.Encode(data, qr.M)
	if err != nil {
		return nil, fmt.Errorf("linkpage: qr: %w", err)
	}
	rows := make([]string, code.Size)
	row := make([]byte, code.Size)
	for y := range code.Size {
		for x := range code.Size {
			row[x] = '0'
			if code.Black(x, y) {
				row[x] = '1'
			}
		}
		rows[y] = string(row)
	}
	grid, err := json.Marshal(Grid{ID: qrID(data), Rows: rows})
	if err != nil {
		return nil, fmt.Errorf("linkpage: qr: %w", err)
	}
	p.qr, p.grid = data, grid
	return p.grid, nil
}

func qrID(data string) string {
	sum := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sum[:6])
}

func (p *Page) again(w http.ResponseWriter, _ *http.Request) {
	if st := p.l.Status(); st.Phase == linker.Linked || st.Phase.InFlight() {
		w.WriteHeader(http.StatusConflict)
		return
	}
	go func() { _, _ = p.l.Start(p.life, "") }()
	w.WriteHeader(http.StatusAccepted)
}

func (p *Page) index(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'")
	_, _ = io.WriteString(w, page)
}

//go:embed page.html
var page string
