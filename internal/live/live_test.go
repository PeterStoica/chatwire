package live_test

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
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

func TestQueriesAreAnsweredWhileEventsPileUp(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w, err := fakeworld.New(73)
		if err != nil {
			t.Fatal(err)
		}
		const flood = 500
		w.Script(func(c *fakeworld.Conn) {
			query := c.Receive()
			for i := range flood {
				c.Send(node.Node{Tag: "message", Attrs: []node.Attr{{Key: "id", Value: node.Text(fmt.Sprint("m", i))}}})
			}
			c.Send(node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "type", Value: node.Text("result")}, {Key: "id", Value: query.Attr("id")}}})
		})
		session := dialLive(t, w)
		query := node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "id", Value: node.Value{}}, {Key: "type", Value: node.Text("get")}}}
		if _, err := session.Query(t.Context(), query); err != nil {
			t.Fatalf("Query() behind %d unread events: %v", flood, err)
		}
		for i := range flood {
			if event := <-session.Events(); event.Attr("id").String() != fmt.Sprint("m", i) {
				t.Fatalf("event %d = %s", i, event)
			}
		}
		if _, open := <-session.Events(); open {
			t.Fatal("events still open after the server hung up")
		}
	})
}

func TestAcksAndRefusals(t *testing.T) {
	text := func(k, v string) node.Attr { return node.Attr{Key: k, Value: node.Text(v)} }
	from := func(j string) node.Attr { return node.Attr{Key: "from", Value: node.Text(j)} }
	render := func(n node.Node) string {
		out := n.Tag
		for _, a := range n.Attrs {
			out += " " + a.Key + "=" + a.Value.String()
		}
		return out
	}
	for _, tt := range []struct {
		name   string
		stanza node.Node
		ack    string
		nack   string
	}{
		{
			name:   "a message",
			stanza: node.Node{Tag: "message", Attrs: []node.Attr{from("40722222222@s.whatsapp.net"), text("id", "M1"), text("type", "text")}},
			ack:    "ack to=40722222222@s.whatsapp.net id=M1 class=message",
			nack:   "ack to=40722222222@s.whatsapp.net id=M1 class=message type=text error=487",
		},
		{
			name:   "a group receipt",
			stanza: node.Node{Tag: "receipt", Attrs: []node.Attr{from("120363000000000000@g.us"), text("id", "R1"), text("type", "read"), text("participant", "40722222222:3@s.whatsapp.net")}},
			ack:    "ack to=120363000000000000@g.us id=R1 class=receipt type=read participant=40722222222:3@s.whatsapp.net",
			nack:   "ack to=120363000000000000@g.us id=R1 class=receipt type=read participant=40722222222:3@s.whatsapp.net error=487",
		},
		{
			name:   "a receipt whose participant is the sender",
			stanza: node.Node{Tag: "receipt", Attrs: []node.Attr{from("40722222222@s.whatsapp.net"), text("id", "R2"), text("participant", "40722222222@s.whatsapp.net")}},
			ack:    "ack to=40722222222@s.whatsapp.net id=R2 class=receipt",
			nack:   "ack to=40722222222@s.whatsapp.net id=R2 class=receipt participant=40722222222@s.whatsapp.net error=487",
		},
		{
			name:   "a call",
			stanza: node.Node{Tag: "call", Attrs: []node.Attr{from("40722222222@s.whatsapp.net"), text("id", "C1")}},
			ack:    "ack to=40722222222@s.whatsapp.net id=C1 class=call",
			nack:   "ack to=40722222222@s.whatsapp.net id=C1 class=call error=487",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := render(live.Ack(tt.stanza)); got != tt.ack {
				t.Errorf("Ack\n got %s\nwant %s", got, tt.ack)
			}
			if got := render(live.Nack(tt.stanza, live.Unparsable)); got != tt.nack {
				t.Errorf("Nack\n got %s\nwant %s", got, tt.nack)
			}
		})
	}
}

func TestPingsGetAPongInTheirOwnShape(t *testing.T) {
	t.Parallel()
	server := node.Address(node.JID{Server: node.ServerUser})
	ping := node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "from", Value: server}, {Key: "type", Value: node.Text("get")}, {Key: "t", Value: node.Text("1790528734")}, {Key: "xmlns", Value: node.Text("urn:xmpp:ping")}}}
	pong, ok := live.Pong(ping)
	if !ok || strings.TrimSpace(pong.String()) != `<iq type="result" to="s.whatsapp.net"/>` {
		t.Fatalf("Pong(%s) = %s, %v", ping, pong, ok)
	}
	pong, ok = live.Pong(ping.With("id", node.Text("ping-7")))
	if !ok || strings.TrimSpace(pong.String()) != `<iq id="ping-7" type="result" to="s.whatsapp.net"/>` {
		t.Fatalf("Pong with an id = %s, %v", pong, ok)
	}
	if _, ok := live.Pong(ping.With("xmlns", node.Text("w:other"))); ok {
		t.Fatal("answered an iq that is not a ping")
	}
}
