package frame_test

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/synctest"
	"time"

	"github.com/coder/websocket"

	"github.com/PeterStoica/chatwire/internal/frame"
)

const (
	socketURL = "ws://web.whatsapp.test/ws/chat"
	origin    = "https://web.whatsapp.test"
)

func serve(t *testing.T, handler http.HandlerFunc) *http.Client {
	t.Helper()
	return httptest.NewTestServer(t, handler).Client()
}

func accept(w http.ResponseWriter, r *http.Request) (*websocket.Conn, bool) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	return conn, err == nil
}

func TestWebSocketSkipsTextAndEchoesBinary(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := serve(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Origin") != origin {
				http.Error(w, "bad origin", http.StatusForbidden)
				return
			}
			conn, ok := accept(w, r)
			if !ok {
				return
			}
			defer func() { _ = conn.CloseNow() }()
			kind, data, err := conn.Read(r.Context())
			if err != nil || kind != websocket.MessageBinary {
				return
			}
			_ = conn.Write(r.Context(), websocket.MessageText, []byte("ignored"))
			_ = conn.Write(r.Context(), websocket.MessageBinary, data)
			_, _, _ = conn.Read(r.Context())
		})
		socket, err := frame.DialWebSocket(t.Context(), client, socketURL, origin)
		if err != nil {
			t.Fatal(err)
		}
		if err := socket.WriteMessage(t.Context(), []byte{1, 2, 3}); err != nil {
			t.Fatal(err)
		}
		got, err := socket.ReadMessage(t.Context())
		if err != nil || string(got) != "\x01\x02\x03" {
			t.Fatalf("ReadMessage() = %x, %v", got, err)
		}
		if err := socket.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := socket.ReadMessage(t.Context()); err == nil {
			t.Fatal("read after close succeeded")
		}
	})
}

func TestDialWebSocketRejectsRefusal(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := serve(t, http.NotFoundHandler().ServeHTTP)
		if _, err := frame.DialWebSocket(t.Context(), client, socketURL, origin); err == nil {
			t.Fatal("dial to a non-websocket endpoint succeeded")
		}
	})
}

func TestDialGivesUpExactlyAtTheDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client := serve(t, func(_ http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		})
		ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
		defer cancel()
		start := time.Now()
		if _, err := frame.DialWebSocket(ctx, client, socketURL, origin); err == nil {
			t.Fatal("dial to a silent server succeeded")
		}
		if waited := time.Since(start); waited != 20*time.Second {
			t.Fatalf("gave up after %s, want exactly 20s", waited)
		}
	})
}

func TestWebSocketReadsLargestFrame(t *testing.T) {
	const largest = frame.MaxSize + 3
	synctest.Test(t, func(t *testing.T) {
		client := serve(t, func(w http.ResponseWriter, r *http.Request) {
			conn, ok := accept(w, r)
			if !ok {
				return
			}
			defer func() { _ = conn.CloseNow() }()
			_ = conn.Write(r.Context(), websocket.MessageBinary, make([]byte, largest))
			_, _, _ = conn.Read(r.Context())
		})
		socket, err := frame.DialWebSocket(t.Context(), client, socketURL, origin)
		if err != nil {
			t.Fatal(err)
		}
		defer socket.Close()
		got, err := socket.ReadMessage(t.Context())
		if err != nil || len(got) != largest {
			t.Fatalf("ReadMessage() = %d bytes, %v; want %d", len(got), err, largest)
		}
	})
}

func TestCloseIsCleanWhenThePeerHangsUpInsteadOfAnswering(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		got := make(chan byte, 1)
		client := serve(t, func(w http.ResponseWriter, r *http.Request) {
			sum := sha1.Sum([]byte(r.Header.Get("Sec-WebSocket-Key") + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
			conn, rw, err := http.NewResponseController(w).Hijack()
			if err != nil {
				return
			}
			defer func() { _ = conn.Close() }()
			if !upgraded(rw, base64.StdEncoding.EncodeToString(sum[:])) {
				return
			}
			first, err := rw.ReadByte()
			if err == nil {
				got <- first
			}
		})
		socket, err := frame.DialWebSocket(t.Context(), client, socketURL, origin)
		if err != nil {
			t.Fatal(err)
		}
		if err := socket.Close(); err != nil {
			t.Fatalf("Close() after the peer hung up = %v", err)
		}
		if first := <-got; first != 0x88 {
			t.Fatalf("the peer saw %#x first, want a close frame", first)
		}
	})
}

func upgraded(rw *bufio.ReadWriter, accept string) bool {
	_, err := fmt.Fprintf(rw, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", accept)
	return err == nil && rw.Flush() == nil
}
