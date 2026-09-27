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
	tokenBytes        = 16
	qrScale           = 8
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
	png    []byte
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
	mux.HandleFunc("GET "+base+"qr.png", p.image)
	mux.HandleFunc("POST "+base+"again", p.again)
	p.server = &http.Server{Handler: p.local(mux), ReadHeaderTimeout: readHeaderTimeout}
	go func() { _ = p.server.Serve(listener) }()
	return p, nil
}

func (p *Page) Close() error {
	return p.server.Close()
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
		sum := sha256.Sum256([]byte(st.QR))
		out.QR = hex.EncodeToString(sum[:6])
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

func (p *Page) state(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.MarshalWrite(w, p.State())
}

func (p *Page) image(w http.ResponseWriter, _ *http.Request) {
	st := p.l.Status()
	if st.Phase != linker.ShowingQR || st.QR == "" {
		http.NotFound(w, nil)
		return
	}
	png, err := p.render(st.QR)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	_, _ = w.Write(png)
}

func (p *Page) render(data string) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if data == p.qr {
		return p.png, nil
	}
	code, err := qr.Encode(data, qr.M)
	if err != nil {
		return nil, fmt.Errorf("linkpage: qr: %w", err)
	}
	code.Scale = qrScale
	p.qr, p.png = data, code.PNG()
	return p.png, nil
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
	w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'")
	_, _ = io.WriteString(w, page)
}

//go:embed page.html
var page string
