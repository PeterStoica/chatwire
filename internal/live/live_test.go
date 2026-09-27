package live_test

import (
	"bytes"
	"crypto/rand"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/PeterStoica/chatwire/internal/curve"
	"github.com/PeterStoica/chatwire/internal/handshake"
	"github.com/PeterStoica/chatwire/internal/live"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/testkit/fakeworld"
)

func TestStanzaIDsFollowWhatsAppWebsForm(t *testing.T) {
	ids, err := live.NewIDs(bytes.NewReader([]byte{0x12, 0x34, 0xab, 0xcd}))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"4660.43981-1", "4660.43981-2", "4660.43981-3"} {
		if got := ids.Next(); got != want {
			t.Fatalf("Next() = %q, want %q", got, want)
		}
	}
}

func TestStanzaIDsNeedFourRandomBytes(t *testing.T) {
	if _, err := live.NewIDs(bytes.NewReader([]byte{1, 2, 3})); err == nil {
		t.Fatal("NewIDs accepted three random bytes")
	}
}

func TestConcurrentSendsAllArriveDecryptable(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, err := fakeworld.New(71)
		if err != nil {
			t.Fatal(err)
		}
		const senders, each = 20, 10
		var received atomic.Int32
		w.Script(func(c *fakeworld.Conn) {
			for range senders * each {
				c.Receive()
				received.Add(1)
			}
		})
		session := dialLive(t, w)
		var wg sync.WaitGroup
		for range senders {
			wg.Go(func() {
				for range each {
					if err := session.Send(t.Context(), node.Node{Tag: "presence", Attrs: []node.Attr{{Key: "id", Value: node.Text(session.NewID())}}}); err != nil {
						t.Error(err)
					}
				}
			})
		}
		wg.Wait()
		synctest.Wait()
		if got := received.Load(); got != senders*each {
			t.Fatalf("the server decrypted %d of %d concurrent sends", got, senders*each)
		}
	})
}

func dialLive(t *testing.T, w *fakeworld.World) *live.Session {
	t.Helper()
	messages, err := w.Dial(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	static, err := curve.NewKeyPair(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := handshake.Initiate(t.Context(), messages, handshake.Config{
		DictVersion: w.Dictionary.Version(), Root: w.Authority.Root(), Static: static, Payload: []byte{1}, Random: rand.Reader, Now: time.Now,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	session, err := live.Start(t.Context(), conn, w.Dictionary, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func TestEventsQueriesAndHangUp(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, err := fakeworld.New(72)
		if err != nil {
			t.Fatal(err)
		}
		w.Script(func(c *fakeworld.Conn) {
			c.Send(node.Node{Tag: "notification", Attrs: []node.Attr{{Key: "id", Value: node.Text("n1")}}})
			first := c.Receive()
			c.Send(node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "type", Value: node.Text("result")}, {Key: "id", Value: first.Attr("id")}}})
			second := c.Receive()
			c.Send(node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "type", Value: node.Text("error")}, {Key: "id", Value: second.Attr("id")}}})
			c.Receive()
		})
		session := dialLive(t, w)
		if event := <-session.Events(); event.Tag != "notification" {
			t.Fatalf("event = %s", event)
		}
		query := node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "id", Value: node.Value{}}, {Key: "type", Value: node.Text("get")}}}
		if reply, err := session.Query(t.Context(), query); err != nil || reply.Attr("type").String() != "result" {
			t.Fatalf("Query() = %s, %v", reply, err)
		}
		if reply, err := session.Query(t.Context(), query); err == nil || reply.Attr("type").String() != "error" {
			t.Fatalf("Query() of a failing iq = %s, %v", reply, err)
		}
		if _, err := session.Query(t.Context(), query); !errors.Is(err, live.ErrClosed) {
			t.Fatalf("Query() after the server hung up = %v, want %v", err, live.ErrClosed)
		}
		if _, open := <-session.Events(); open {
			t.Fatal("events still open after the server hung up")
		}
		if session.Err() == nil {
			t.Fatal("Err() is nil after the connection ended")
		}
	})
}
