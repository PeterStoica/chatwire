package linkflow_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/PeterStoica/chatwire/internal/fakephone"
	"github.com/PeterStoica/chatwire/internal/fakeworld"
	"github.com/PeterStoica/chatwire/internal/frame"
	"github.com/PeterStoica/chatwire/internal/linkflow"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/pairing"
)

func world(t *testing.T) *fakeworld.World {
	t.Helper()
	w, err := fakeworld.New(11)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

type disk struct {
	saved []linkflow.Linked
	err   error
}

func (d *disk) save(linked linkflow.Linked) error {
	d.saved = append(d.saved, linked)
	return d.err
}

func config(w *fakeworld.World, phone pairing.Phone, d *disk) linkflow.Config {
	return linkflow.Config{
		Save: d.save,
		Dial: w.Dial, Dictionary: w.Dictionary, Root: w.Authority.Root(), Version: w.Version,
		Random: w.Random, Now: time.Now, Phone: phone, ShowQR: w.ShowQR,
		ShowCode: func(code string) {
			w.ShowCode(code)
			w.Type(code)
		},
	}
}

func TestLinkByQR(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := world(t)
		w.Script(w.QRPairing(10*time.Second), w.Login(fakeworld.Success()))
		d := &disk{}
		linked, err := linkflow.Link(t.Context(), config(w, "", d))
		if err != nil {
			t.Fatal(err)
		}
		if len(d.saved) != 1 || !reflect.DeepEqual(d.saved[0], linked) {
			t.Fatalf("saved %d times, want once with the linked device", len(d.saved))
		}
		if linked.Account.JID != w.Phone.JID || linked.Account.LID != w.Phone.LID || linked.Account.KeyIndex != 2 {
			t.Fatalf("linked account = %+v", linked.Account)
		}
		if shown := w.ShownQR(); len(shown) != 1 || !strings.Contains(shown[0], "#2@ref-1,") {
			t.Fatalf("shown QR codes = %v", shown)
		}
	})
}

func TestQRRotatesThenExpires(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := world(t)
		w.Script(w.Unscanned())
		start := time.Now()
		d := &disk{}
		_, err := linkflow.Link(t.Context(), config(w, "", d))
		if !errors.Is(err, linkflow.ErrQRExpired) {
			t.Fatalf("Link() error = %v, want %v", err, linkflow.ErrQRExpired)
		}
		if waited := time.Since(start); waited != 60*time.Second+5*20*time.Second {
			t.Fatalf("gave up after %s, want 60s + 5 x 20s", waited)
		}
		if shown := w.ShownQR(); len(shown) != 6 || !strings.Contains(shown[5], "#2@ref-6,") {
			t.Fatalf("shown QR codes = %v", shown)
		}
		if len(d.saved) != 0 {
			t.Fatalf("saved %d devices without pairing", len(d.saved))
		}
	})
}

func TestLinkByPhoneNumberCode(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := world(t)
		w.Script(w.CodePairing(), w.Login(fakeworld.Success()))
		linked, err := linkflow.Link(t.Context(), config(w, "40700000000", &disk{}))
		if err != nil {
			t.Fatal(err)
		}
		if linked.Account.JID != w.Phone.JID || len(w.ShownQR()) != 0 || len(w.ShownCodes()) != 1 {
			t.Fatalf("linked %+v, QR shown %d times, codes %v", linked.Account, len(w.ShownQR()), w.ShownCodes())
		}
	})
}

func TestLoginRejectedAfterPairing(t *testing.T) {
	for _, tt := range []struct {
		reason    string
		loggedOut bool
	}{
		{reason: "401", loggedOut: true},
		{reason: "500", loggedOut: false},
	} {
		t.Run(tt.reason, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				w := world(t)
				w.Script(w.QRPairing(time.Second), w.Login(fakeworld.Failure(tt.reason)))
				d := &disk{}
				linked, err := linkflow.Link(t.Context(), config(w, "", d))
				if !errors.Is(err, linkflow.ErrLoginRejected) || errors.Is(err, linkflow.ErrLoggedOut) != tt.loggedOut || strings.Contains(err.Error(), "%!") {
					t.Fatalf("Link() error = %v, want %v (logged out: %v)", err, linkflow.ErrLoginRejected, tt.loggedOut)
				}
				if len(d.saved) != 1 || !reflect.DeepEqual(d.saved[0], linked) || linked.Account.JID != w.Phone.JID {
					t.Fatalf("the paired device was saved %d times before the rejected login, want once", len(d.saved))
				}
			})
		})
	}
}

func TestSaveFailureStopsBeforeLogin(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := world(t)
		w.Script(w.QRPairing(time.Second), w.Login(fakeworld.Success()))
		errDisk := errors.New("disk full")
		_, err := linkflow.Link(t.Context(), config(w, "", &disk{err: errDisk}))
		if !errors.Is(err, errDisk) {
			t.Fatalf("Link() error = %v, want %v", err, errDisk)
		}
		if dials := w.Dials(); dials != 1 {
			t.Fatalf("dialled %d times, want only the pairing connection", dials)
		}
	})
}

func TestTamperedPairSuccessIsRefused(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := world(t)
		w.Script(func(c *fakeworld.Conn) {
			c.Send(fakephone.PairDevice("p1", 6))
			_ = c.Receive()
			synctest.Sleep(time.Second)
			shown := w.ShownQR()
			scan, err := fakephone.ParseQR(shown[len(shown)-1])
			if err != nil {
				panic(err)
			}
			success, err := w.Phone.PairSuccess("s1", scan.Identity, make([]byte, 32))
			if err != nil {
				panic(err)
			}
			c.Send(success)
			reply := c.Receive()
			if kind, _ := reply.Attr("type").Text(); kind != "error" {
				panic("expected an error reply, got " + reply.String())
			}
			c.WaitForHangUp()
		})
		_, err := linkflow.Link(t.Context(), config(w, "", &disk{}))
		if !errors.Is(err, pairing.ErrHMAC) {
			t.Fatalf("Link() error = %v, want %v", err, pairing.ErrHMAC)
		}
	})
}

type refusing struct {
	closed bool
}

func (r *refusing) ReadMessage(context.Context) ([]byte, error) { return nil, errors.New("refused") }

func (r *refusing) WriteMessage(context.Context, []byte) error { return nil }

func (r *refusing) Close() error {
	r.closed = true
	return nil
}

func TestFailedHandshakeClosesTheConnection(t *testing.T) {
	w := world(t)
	conn := &refusing{}
	cfg := config(w, "", &disk{})
	cfg.Dial = func(context.Context) (frame.MessageConn, error) { return conn, nil }
	if _, err := linkflow.Link(t.Context(), cfg); err == nil || !conn.closed {
		t.Fatalf("Link() = %v, connection closed %v", err, conn.closed)
	}
}

func TestClassifyingHowWhatsAppEndsASession(t *testing.T) {
	t.Parallel()
	other := errors.New("other")
	conflict := func(kind string) []node.Node {
		return []node.Node{{Tag: "conflict", Attrs: []node.Attr{{Key: "type", Value: node.Text(kind)}}}}
	}
	attr := func(key, value string) []node.Attr { return []node.Attr{{Key: key, Value: node.Text(value)}} }
	for _, tt := range []struct {
		name string
		n    node.Node
		want error
	}{
		{name: "device removed", n: node.Node{Tag: "stream:error", Attrs: attr("code", "401"), Children: conflict("device_removed")}, want: linkflow.ErrLoggedOut},
		{name: "401 alone", n: node.Node{Tag: "stream:error", Attrs: attr("code", "401")}, want: linkflow.ErrLoggedOut},
		{name: "replaced", n: node.Node{Tag: "stream:error", Children: conflict("replaced")}, want: linkflow.ErrReplaced},
		{name: "restart", n: node.Node{Tag: "stream:error", Attrs: attr("code", "515")}, want: other},
		{name: "unavailable", n: node.Node{Tag: "stream:error", Attrs: attr("code", "503")}, want: other},
		{name: "logged out at login", n: node.Node{Tag: "failure", Attrs: attr("reason", "401")}, want: linkflow.ErrLoggedOut},
		{name: "main device gone", n: node.Node{Tag: "failure", Attrs: attr("reason", "403")}, want: linkflow.ErrLoggedOut},
		{name: "unknown logout", n: node.Node{Tag: "failure", Attrs: attr("reason", "406")}, want: linkflow.ErrLoggedOut},
		{name: "temporarily banned", n: node.Node{Tag: "failure", Attrs: attr("reason", "402")}, want: other},
		{name: "outdated client", n: node.Node{Tag: "failure", Attrs: attr("reason", "405")}, want: other},
		{name: "a failure with a code attribute", n: node.Node{Tag: "failure", Attrs: attr("code", "401")}, want: other},
		{name: "a stream error with a reason attribute", n: node.Node{Tag: "stream:error", Attrs: attr("reason", "401")}, want: other},
		{name: "a failure with a conflict", n: node.Node{Tag: "failure", Children: conflict("replaced")}, want: other},
	} {
		if got := linkflow.Classify(tt.n, other); !errors.Is(got, tt.want) {
			t.Errorf("%s: Classify() = %v, want %v", tt.name, got, tt.want)
		}
	}
}
