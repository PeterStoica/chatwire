package qrpage

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"sync"
	"time"

	"rsc.io/qr"
)

const readHeaderTimeout = 5 * time.Second

type Page struct {
	mu      sync.Mutex
	qr      []byte
	code    string
	status  string
	version int
	server  *http.Server
	URL     string
}

func Start(ctx context.Context) (*Page, error) {
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("qrpage: listen: %w", err)
	}
	p := &Page{URL: "http://" + listener.Addr().String() + "/", status: "Waiting for WhatsApp..."}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", p.index)
	mux.HandleFunc("GET /qr.png", p.image)
	mux.HandleFunc("GET /state", p.state)
	p.server = &http.Server{Handler: mux, ReadHeaderTimeout: readHeaderTimeout}
	go func() {
		if err := p.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			p.SetStatus("page server stopped: " + err.Error())
		}
	}()
	return p, nil
}

func (p *Page) Close() error {
	return p.server.Close()
}

func (p *Page) ShowQR(data string) {
	code, err := qr.Encode(data, qr.M)
	p.mu.Lock()
	defer p.mu.Unlock()
	if err != nil {
		p.status = "cannot render QR: " + err.Error()
		return
	}
	code.Scale = 8
	p.qr, p.code, p.version = code.PNG(), "", p.version+1
	p.status = "Open WhatsApp on your phone: Settings > Linked devices > Link a device, and scan this code."
}

func (p *Page) ShowCode(linkingCode string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.qr, p.code, p.version = nil, linkingCode, p.version+1
	p.status = "On your phone: WhatsApp > Settings > Linked devices > Link a device > Link with phone number instead, and type this code."
}

func (p *Page) SetStatus(status string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.status, p.version = status, p.version+1
}

func (p *Page) state(w http.ResponseWriter, _ *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"version":%d}`, p.version)
}

func (p *Page) image(w http.ResponseWriter, _ *http.Request) {
	p.mu.Lock()
	image := p.qr
	p.mu.Unlock()
	if image == nil {
		http.NotFound(w, nil)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(image)
}

func (p *Page) index(w http.ResponseWriter, _ *http.Request) {
	p.mu.Lock()
	status, code, hasQR, version := p.status, p.code, p.qr != nil, p.version
	p.mu.Unlock()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	body := `<p class="status">` + html.EscapeString(status) + `</p>`
	if hasQR {
		body += fmt.Sprintf(`<img alt="WhatsApp linking QR code" src="/qr.png?v=%d">`, version)
	}
	if code != "" {
		body += `<p class="code">` + html.EscapeString(code) + `</p>`
	}
	fmt.Fprintf(w, `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Link WhatsApp</title>
<style>body{font-family:system-ui,sans-serif;display:flex;flex-direction:column;align-items:center;justify-content:center;min-height:100vh;margin:0;background:#fff;color:#111}
img{width:min(80vw,360px);image-rendering:pixelated}.status{max-width:36em;text-align:center;font-size:1.1rem;padding:0 16px}.code{font-size:3rem;letter-spacing:.2em;font-family:ui-monospace,monospace}</style></head>
<body>%s<script>let v=%d;setInterval(async()=>{try{const r=await fetch('/state');const s=await r.json();if(s.version!==v)location.reload()}catch(e){}},1000)</script></body></html>`, body, version)
}
