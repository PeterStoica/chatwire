package dial_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/synctest"

	"github.com/coder/websocket"

	"github.com/PeterStoica/chatwire/internal/dial"
)

func TestRequestsLookLikeTheBrowser(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		seen := make(chan http.Header, 2)
		server := httptest.NewTestServer(t, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			seen <- r.Header.Clone()
		}))
		client := dial.Client(server.Client().Transport)
		for _, language := range []string{"", "ro-RO"} {
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, dial.Page, nil)
			if err != nil {
				t.Fatal(err)
			}
			if language != "" {
				req.Header.Set("Accept-Language", language)
			}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			_ = resp.Body.Close()
			if req.Header.Get("User-Agent") != "" {
				t.Fatal("the caller's request was modified")
			}
			header := <-seen
			want := language
			if want == "" {
				want = dial.Language
			}
			if header.Get("User-Agent") != dial.UserAgent || header.Get("Accept-Language") != want {
				t.Fatalf("sent User-Agent %q, Accept-Language %q", header.Get("User-Agent"), header.Get("Accept-Language"))
			}
		}
	})
}

func TestDialerOpensTheChatSocketAsTheBrowser(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		server := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/ws/chat" || r.Header.Get("Origin") != dial.Origin || r.Header.Get("User-Agent") != dial.UserAgent {
				http.Error(w, "not the browser", http.StatusForbidden)
				return
			}
			conn, err := websocket.Accept(w, r, nil)
			if err != nil {
				return
			}
			defer func() { _ = conn.CloseNow() }()
			if kind, data, err := conn.Read(r.Context()); err == nil && kind == websocket.MessageBinary {
				_ = conn.Write(r.Context(), websocket.MessageBinary, data)
			}
			_, _, _ = conn.Read(r.Context())
		}))
		socket, err := dial.Dialer(dial.Client(server.Client().Transport))(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = socket.Close() }()
		if err := socket.WriteMessage(t.Context(), []byte("hi")); err != nil {
			t.Fatal(err)
		}
		if got, err := socket.ReadMessage(t.Context()); err != nil || string(got) != "hi" {
			t.Fatalf("ReadMessage() = %q, %v", got, err)
		}
	})
}
