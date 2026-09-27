package dial

import (
	"context"
	"net/http"

	"github.com/PeterStoica/chatwire/internal/frame"
)

const (
	Origin    = "https://web.whatsapp.com"
	Page      = Origin + "/"
	SocketURL = "wss://web.whatsapp.com/ws/chat"
	UserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36"
	Language  = "en-US,en;q=0.9"
)

type browser struct {
	next http.RoundTripper
}

func Client(next http.RoundTripper) *http.Client {
	return &http.Client{Transport: browser{next: next}}
}

func (b browser) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("User-Agent", UserAgent)
	if req.Header.Get("Accept-Language") == "" {
		req.Header.Set("Accept-Language", Language)
	}
	return b.next.RoundTrip(req)
}

func Dialer(client *http.Client) func(context.Context) (frame.MessageConn, error) {
	return func(ctx context.Context) (frame.MessageConn, error) {
		return frame.DialWebSocket(ctx, client, SocketURL, Origin)
	}
}
