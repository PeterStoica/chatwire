package client_test

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"go.uber.org/goleak"
	"google.golang.org/protobuf/proto"

	"github.com/PeterStoica/chatwire/internal/appstate"
	"github.com/PeterStoica/chatwire/internal/client"
	"github.com/PeterStoica/chatwire/internal/dial"
	"github.com/PeterStoica/chatwire/internal/groups"
	"github.com/PeterStoica/chatwire/internal/history"
	"github.com/PeterStoica/chatwire/internal/linkflow"
	"github.com/PeterStoica/chatwire/internal/media"
	"github.com/PeterStoica/chatwire/internal/mediaretry"
	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/prekeys"
	"github.com/PeterStoica/chatwire/internal/privacy"
	"github.com/PeterStoica/chatwire/internal/signal"
	"github.com/PeterStoica/chatwire/internal/testkit/fakeappstate"
	"github.com/PeterStoica/chatwire/internal/testkit/fakecdn"
	"github.com/PeterStoica/chatwire/internal/testkit/fakedevice"
	"github.com/PeterStoica/chatwire/internal/testkit/fakegroups"
	"github.com/PeterStoica/chatwire/internal/testkit/fakekeys"
	"github.com/PeterStoica/chatwire/internal/testkit/fakerelay"
	"github.com/PeterStoica/chatwire/internal/testkit/fakeusync"
	"github.com/PeterStoica/chatwire/internal/testkit/fakeworld"
	"github.com/PeterStoica/chatwire/internal/wire"
	"sync/atomic"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

type rig struct {
	keepAlive time.Duration
	t         *testing.T
	world     *fakeworld.World
	keys      *fakekeys.Server
	devices   *fakeusync.Server
	server    *fakeworld.Server
	account   node.JID
	ourPhone  *fakedevice.Device
	bob       node.JID
	bobPhone  *fakedevice.Device
	mu        sync.Mutex
	delivered map[node.JID][]node.Node
	sent      []node.Node
	persisted []client.State
	received  chan client.Received
	history   chan history.Chunk
	appState  client.AppStateStore
	receipts  chan message.Receipt
	http      *http.Client
	lids      map[node.JID]node.JID
	now       func() time.Time
	receive   func(client.Received)
	seen      func(chat node.JID, id string) bool
	tokenOf   func(node.JID) privacy.Token
	saved     []privacy.Token
	problems  []error
}

func newRig(t *testing.T) *rig {
	t.Helper()
	w, err := fakeworld.New(51)
	if err != nil {
		t.Fatal(err)
	}
	r := &rig{t: t, world: w, keys: fakekeys.New(), devices: fakeusync.New(), delivered: map[node.JID][]node.Node{}, received: make(chan client.Received, 4), keepAlive: -1}
	r.account = node.JID{User: w.Phone.JID.User, Server: w.Phone.JID.Server}
	r.bob = node.JID{User: "40722222222", Server: node.ServerUser}
	for _, d := range []struct {
		target **fakedevice.Device
		jid    node.JID
	}{{&r.ourPhone, r.account}, {&r.bobPhone, r.bob}} {
		if *d.target, err = fakedevice.New(rand.Reader, d.jid); err != nil {
			t.Fatal(err)
		}
		if err := (*d.target).Upload(r.keys, 3); err != nil {
			t.Fatal(err)
		}
	}
	r.devices.Set(r.account, fakeusync.Device{ID: 0}, fakeusync.Device{ID: w.Phone.JID.Device})
	r.devices.Set(r.bob, fakeusync.Device{ID: 0})
	r.server = &fakeworld.Server{
		Keys: r.keys, Devices: r.devices, PushName: "Me", Inbox: make(chan node.Node, 8),
		Deliver: func(device node.JID, stanza node.Node) {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.delivered[device] = append(r.delivered[device], stanza)
		},
		Received: func(n node.Node) {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.sent = append(r.sent, n)
		},
	}
	return r
}

func (r *rig) onReceipt() func(message.Receipt) {
	if r.receipts == nil {
		return nil
	}
	return func(rc message.Receipt) { r.receipts <- rc }
}

func (r *rig) connect() *client.Client {
	r.t.Helper()
	w := r.world
	w.Script(w.QRPairing(time.Second), w.Login(fakeworld.Success()), w.Serve(r.server))
	now := r.now
	if now == nil {
		now = time.Now
	}
	cfg := linkflow.Config{
		Dial: w.Dial, Dictionary: w.Dictionary, Root: w.Authority.Root(), Version: w.Version, Random: rand.Reader, Now: now,
		ShowQR: w.ShowQR, Save: func(linkflow.Linked) error { return nil }, KeepAlive: r.keepAlive,
	}
	linked, err := linkflow.Link(r.t.Context(), cfg)
	if err != nil {
		r.t.Fatal(err)
	}
	var onHistory func(history.Chunk)
	if r.history != nil {
		onHistory = func(chunk history.Chunk) { r.history <- chunk }
	}
	c, err := client.Connect(r.t.Context(), client.Config{
		LIDs: r.lids, Link: cfg, HTTP: r.http, History: onHistory, AppState: r.appState, Receipt: r.onReceipt(),
		Persist: func(s client.State) error {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.persisted = append(r.persisted, s)
			return nil
		},
		Receive: func(m client.Received) {
			if r.receive != nil {
				r.receive(m)
			}
			r.received <- m
		},
		Seen: func(_ context.Context, chat node.JID, id string) bool { return r.seen != nil && r.seen(chat, id) },
		TokenOf: func(_ context.Context, contact node.JID) privacy.Token {
			if r.tokenOf == nil {
				return privacy.Token{}
			}
			return r.tokenOf(contact)
		},
		Tokens: func(tokens []privacy.Token) {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.saved = append(r.saved, tokens...)
		},
		Problem: func(err error) {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.problems = append(r.problems, err)
		},
	}, client.State{Linked: linked})
	if err != nil {
		r.t.Fatal(err)
	}
	r.t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestClientUploadsKeysSendsAndReceives(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		c := r.connect()
		self := r.world.Phone.JID
		if held := r.keys.Keys(self); held != prekeys.Batch {
			t.Fatalf("WhatsApp holds %d of our prekeys, want %d", held, prekeys.Batch)
		}
		if len(r.persisted) == 0 || len(r.persisted[0].PreKeys) != prekeys.Batch {
			t.Fatal("prekeys were not kept before the upload")
		}

		id, err := c.Send(t.Context(), r.bob, &wire.Message{Conversation: new("hello bob")})
		if err != nil {
			t.Fatal(err)
		}
		r.mu.Lock()
		toBob, toOurPhone := r.delivered[r.bob], r.delivered[r.account]
		r.mu.Unlock()
		if len(toBob) != 1 || len(toOurPhone) != 1 {
			t.Fatalf("delivered %d to bob and %d to our phone", len(toBob), len(toOurPhone))
		}
		in, m, err := r.bobPhone.Receive(toBob[0])
		if err != nil || m.GetConversation() != "hello bob" || in.Author != self || in.Chat != r.account || in.ID != id {
			t.Fatalf("bob read %v from %v in %v: %v", m, in.Author, in.Chat, err)
		}
		in, m, err = r.ourPhone.Receive(toOurPhone[0])
		if err != nil || m.GetDeviceSentMessage().GetMessage().GetConversation() != "hello bob" || in.Chat != r.bob {
			t.Fatalf("our phone read %v in %v: %v", m, in.Chat, err)
		}

		reply, err := r.bobPhone.Send(r.keys, r.devices, r.account, &wire.Message{Conversation: new("hi, it's bob")})
		if err != nil {
			t.Fatal(err)
		}
		deliveries := fakerelay.Deliver(r.bob, "Bob", time.Now(), reply)
		r.server.Inbox <- deliveries[self]
		got := <-r.received
		if got.Message.GetConversation() != "hi, it's bob" || got.Chat != r.bob || got.Author != r.bob || got.Name != "Bob" {
			t.Fatalf("received %+v", got)
		}
		r.mu.Lock()
		kept := len(r.persisted[len(r.persisted)-1].PreKeys)
		r.mu.Unlock()
		if kept != prekeys.Batch {
			t.Fatalf("%d prekeys kept, but bob replied on the session we started", kept)
		}
		synctest.Wait()
		replyID, _ := reply.Attr("id").Text()
		r.mu.Lock()
		defer r.mu.Unlock()
		for _, n := range r.sent {
			if id, _ := n.Attr("id").Text(); n.Tag == "receipt" && id == replyID {
				return
			}
		}
		t.Fatal("no delivery receipt for bob's reply")
	})
}

func TestStreamErrorEndsTheSession(t *testing.T) {
	t.Parallel()
	conflict := func(kind string) []node.Node {
		return []node.Node{{Tag: "conflict", Attrs: []node.Attr{{Key: "type", Value: node.Text(kind)}}}}
	}
	for _, tt := range []struct {
		name    string
		ending  node.Node
		wantErr error
		shown   string
	}{
		{name: "device removed", ending: node.Node{Tag: "stream:error", Attrs: []node.Attr{{Key: "code", Value: node.Text("401")}}, Children: conflict("device_removed")}, wantErr: linkflow.ErrLoggedOut, shown: `code="401"`},
		{name: "another connection took over", ending: node.Node{Tag: "stream:error", Children: conflict("replaced")}, wantErr: linkflow.ErrReplaced, shown: "replaced"},
		{name: "restart requested", ending: node.Node{Tag: "stream:error", Attrs: []node.Attr{{Key: "code", Value: node.Text("515")}}}, wantErr: client.ErrClosed, shown: `code="515"`},
		{name: "failure", ending: node.Node{Tag: "failure", Attrs: []node.Attr{{Key: "reason", Value: node.Text("403")}}}, wantErr: linkflow.ErrLoggedOut, shown: `reason="403"`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				r := newRig(t)
				c := r.connect()
				r.server.Inbox <- tt.ending
				<-c.Done()
				if err := c.Err(); !errors.Is(err, tt.wantErr) || !strings.Contains(err.Error(), tt.shown) {
					t.Fatalf("Err() = %v, want %v", err, tt.wantErr)
				}
				if _, err := c.Send(t.Context(), r.bob, &wire.Message{Conversation: new("too late")}); err == nil {
					t.Fatal("sent on a closed session")
				}
			})
		})
	}
}

func TestRefusedPreKeyUploadFailsTheConnection(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		r.server.Override = func(n node.Node) (node.Node, bool) {
			if _, upload := n.Child("list"); !upload {
				return node.Node{}, false
			}
			return node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "type", Value: node.Text("error")}, {Key: "id", Value: n.Attr("id")}}, Children: []node.Node{
				{Tag: "error", Attrs: []node.Attr{{Key: "code", Value: node.Text("406")}, {Key: "text", Value: node.Text("not-acceptable")}}},
			}}, true
		}
		w := r.world
		w.Script(w.QRPairing(time.Second), w.Login(fakeworld.Success()), w.Serve(r.server))
		cfg := linkflow.Config{Dial: w.Dial, Dictionary: w.Dictionary, Root: w.Authority.Root(), Version: w.Version, Random: rand.Reader, Now: time.Now, ShowQR: w.ShowQR, Save: func(linkflow.Linked) error { return nil }, KeepAlive: -1}
		linked, err := linkflow.Link(t.Context(), cfg)
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.Connect(t.Context(), client.Config{Link: cfg, Persist: func(client.State) error { return nil }}, client.State{Linked: linked})
		if !errors.Is(err, prekeys.ErrRejected) {
			t.Fatalf("Connect() = %v, want %v", err, prekeys.ErrRejected)
		}
	})
}

func TestOurOwnDevicesAreKnownByNumberAndByLID(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		r.connect()
		lid := r.world.Phone.LID
		for _, tt := range []struct {
			from node.JID
			want string
		}{
			{node.JID{User: lid.User, Device: 5, Server: lid.Server}, "receipt"},
			{node.JID{User: r.account.User, Device: 5, Server: r.account.Server}, "receipt"},
			{node.JID{User: "555555555555555", Device: 1, Server: node.ServerLID}, "ack"},
		} {
			id := "3EB0" + tt.from.User
			r.server.Inbox <- node.Node{Tag: "message", Attrs: []node.Attr{
				{Key: "from", Value: node.Address(tt.from)}, {Key: "type", Value: node.Text("text")}, {Key: "id", Value: node.Text(id)},
				{Key: "t", Value: node.Text("1790000000")}, {Key: "recipient", Value: node.Address(r.bob)},
			}, Children: []node.Node{{Tag: "enc", Attrs: []node.Attr{{Key: "v", Value: node.Text("2")}, {Key: "type", Value: node.Text("msg")}}, Bytes: []byte{0x33, 1, 2, 3, 4, 5, 6, 7, 8, 9}}}}
			synctest.Wait()
			r.mu.Lock()
			answer := r.sent[len(r.sent)-1]
			r.mu.Unlock()
			answerID, _ := answer.Attr("id").Text()
			recipient, _ := answer.Attr("recipient").JID()
			if answer.Tag != tt.want || answerID != id || tt.want == "receipt" && recipient != r.bob {
				t.Fatalf("message from %v answered with %s", tt.from, answer)
			}
		}
	})
}

func TestUndecryptableMessagesGetARetryReceipt(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		r.connect()
		self := r.world.Phone.JID
		r.server.Inbox <- node.Node{Tag: "message", Attrs: []node.Attr{
			{Key: "from", Value: node.Address(r.bob)}, {Key: "type", Value: node.Text("text")},
			{Key: "id", Value: node.Text("3EB0BAD")}, {Key: "t", Value: node.Text("1790000000")},
		}, Children: []node.Node{{Tag: "enc", Attrs: []node.Attr{{Key: "v", Value: node.Text("2")}, {Key: "type", Value: node.Text("msg")}}, Bytes: []byte{0x33, 1, 2, 3, 4, 5, 6, 7, 8, 9}}}}
		synctest.Wait()
		r.mu.Lock()
		defer r.mu.Unlock()
		for _, n := range r.sent {
			if kind, _ := n.Attr("type").Text(); n.Tag == "receipt" && kind == "retry" {
				retry, _ := n.Child("retry")
				if count, _ := retry.Attr("count").Text(); count != "1" {
					t.Fatalf("retry count %s", count)
				}
				if len(r.persisted) == 0 || len(r.persisted[len(r.persisted)-1].PreKeys) != prekeys.Batch+1 {
					t.Fatal("the fresh prekey for the retry was not kept")
				}
				_ = self
				return
			}
		}
		t.Fatal("no retry receipt")
	})
}

func TestGroupMessagesDecryptWithSenderKeys(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		c := r.connect()
		group := node.JID{User: "120363000000000001", Server: node.ServerGroup}
		key, err := signal.NewSenderKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		distribution, err := key.Distribution()
		if err != nil {
			t.Fatal(err)
		}
		skdm, err := r.bobPhone.EncryptFor(r.keys, c.Self(), &wire.Message{SenderKeyDistributionMessage: &wire.Message_SenderKeyDistributionMessage{
			GroupId: new(group.String()), AxolotlSenderKeyDistributionMessage: distribution,
		}})
		if err != nil {
			t.Fatal(err)
		}
		encNode := func(kind string, raw []byte) node.Node {
			return node.Node{Tag: "enc", Attrs: []node.Attr{{Key: "v", Value: node.Text("2")}, {Key: "type", Value: node.Text(kind)}}, Bytes: raw}
		}
		groupMessage := func(id, text string, sender node.JID, withDistribution bool) node.Node {
			t.Helper()
			padded, err := message.Encode(rand.Reader, &wire.Message{Conversation: new(text)})
			if err != nil {
				t.Fatal(err)
			}
			ciphertext, err := key.Encrypt(rand.Reader, padded)
			if err != nil {
				t.Fatal(err)
			}
			children := []node.Node{encNode("skmsg", ciphertext)}
			if withDistribution {
				children = append(children, encNode("pkmsg", skdm.Ciphertext.Bytes), node.Node{Tag: "device-identity", Bytes: []byte("bob")})
			}
			return node.Node{Tag: "message", Attrs: []node.Attr{
				{Key: "from", Value: node.Address(group)}, {Key: "participant", Value: node.Address(sender)}, {Key: "type", Value: node.Text("text")},
				{Key: "id", Value: node.Text(id)}, {Key: "t", Value: node.Text("1790000000")}, {Key: "notify", Value: node.Text("Bob")},
			}, Children: children}
		}
		for i, text := range []string{"first in the group", "second, same key"} {
			r.server.Inbox <- groupMessage(fmt.Sprint("3EB0G", i), text, r.bob, i == 0)
			got := <-r.received
			if got.Chat != group || got.Author != r.bob || got.Message.GetConversation() != text {
				t.Fatalf("message %d: %+v", i, got)
			}
		}
		r.mu.Lock()
		kept := len(r.persisted[len(r.persisted)-1].PreKeys)
		r.mu.Unlock()
		if kept != prekeys.Batch-1 {
			t.Fatalf("%d prekeys kept after bob started a session with one, want %d", kept, prekeys.Batch-1)
		}
		synctest.Wait()
		select {
		case extra := <-r.received:
			t.Fatalf("the key distribution surfaced as a message: %+v", extra)
		default:
		}
		stranger := node.JID{User: "40799999999", Server: node.ServerUser}
		r.server.Inbox <- groupMessage("3EB0NOKEY", "no key for this", stranger, false)
		synctest.Wait()
		r.mu.Lock()
		defer r.mu.Unlock()
		last := r.sent[len(r.sent)-1]
		participant, _ := last.Attr("participant").JID()
		if kind, _ := last.Attr("type").Text(); last.Tag != "receipt" || kind != "retry" || participant != stranger {
			t.Fatalf("answer to a message without a sender key = %s", last)
		}
	})
}

func TestSendingToAGroup(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		family := groups.Group{
			JID: node.JID{User: "120363000000000009", Server: node.ServerGroup}, Subject: "Family", Created: time.Unix(1700000000, 0), AddressingMode: "pn",
			Participants: []groups.Participant{{JID: r.account, Admin: true}, {JID: r.bob}},
		}
		r.server.Groups = fakegroups.New(family)
		r.server.Members = func(node.JID) []node.JID {
			return []node.JID{r.bob, r.account, r.world.Phone.JID}
		}
		c := r.connect()
		listed, err := c.Groups(t.Context())
		if err != nil || len(listed) != 1 || listed[0].Subject != "Family" || len(listed[0].Participants) != 2 {
			t.Fatalf("Groups() = %+v, %v", listed, err)
		}
		for round, text := range []string{"hello family", "still here"} {
			id, err := c.SendGroup(t.Context(), listed[0], &wire.Message{Conversation: new(text)})
			if err != nil {
				t.Fatal(err)
			}
			r.mu.Lock()
			toBob, toOurPhone := r.delivered[r.bob], r.delivered[r.account]
			r.mu.Unlock()
			if len(toBob) != round+1 || len(toOurPhone) != round+1 {
				t.Fatalf("round %d: %d deliveries to bob, %d to our phone", round, len(toBob), len(toOurPhone))
			}
			for _, d := range []struct {
				device  *fakedevice.Device
				stanzas []node.Node
			}{{r.bobPhone, toBob}, {r.ourPhone, toOurPhone}} {
				stanza := d.stanzas[round]
				in, m, err := d.device.Receive(stanza)
				if err != nil || m.GetConversation() != text || in.Chat != family.JID || in.Author != r.world.Phone.JID || in.ID != id {
					t.Fatalf("round %d: %v read %v in %v from %v: %v", round, d.device.JID, m, in.Chat, in.Author, err)
				}
				if withKey := len(in.Encs) == 2; withKey != (round == 0) {
					t.Fatalf("round %d: %v got %d encs; only the first message carries our sender key", round, d.device.JID, len(in.Encs))
				}
			}
		}
	})
}

func TestConversationsSurviveARestart(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		family := groups.Group{JID: node.JID{User: "120363000000000031", Server: node.ServerGroup}, Subject: "Family", Created: time.Unix(1700000000, 0),
			Participants: []groups.Participant{{JID: r.account}, {JID: r.bob}}}
		r.server.Groups = fakegroups.New(family)
		r.server.Members = func(node.JID) []node.JID { return []node.JID{r.bob, r.account, r.world.Phone.JID} }
		w := r.world
		w.Script(w.QRPairing(time.Second), w.Login(fakeworld.Success()), w.Serve(r.server), w.Serve(r.server))
		cfg := linkflow.Config{Dial: w.Dial, Dictionary: w.Dictionary, Root: w.Authority.Root(), Version: w.Version, Random: rand.Reader, Now: time.Now, ShowQR: w.ShowQR, Save: func(linkflow.Linked) error { return nil }, KeepAlive: -1}
		linked, err := linkflow.Link(t.Context(), cfg)
		if err != nil {
			t.Fatal(err)
		}
		open := func(state client.State) *client.Client {
			t.Helper()
			c, err := client.Connect(t.Context(), client.Config{Link: cfg, Receive: func(m client.Received) { r.received <- m },
				Persist: func(s client.State) error {
					r.mu.Lock()
					defer r.mu.Unlock()
					r.persisted = append(r.persisted, s)
					return nil
				}}, state)
			if err != nil {
				t.Fatal(err)
			}
			return c
		}
		latest := func() []node.Node {
			r.mu.Lock()
			defer r.mu.Unlock()
			return r.delivered[r.bob]
		}
		bobSays := func(text string) {
			t.Helper()
			out, err := r.bobPhone.Send(r.keys, r.devices, r.account, &wire.Message{Conversation: new(text)})
			if err != nil {
				t.Fatal(err)
			}
			r.server.Inbox <- fakerelay.Deliver(r.bob, "Bob", time.Now(), out)[w.Phone.JID]
			if got := <-r.received; got.Message.GetConversation() != text {
				t.Fatalf("received %+v, want %q", got, text)
			}
		}

		bobKey, err := signal.NewSenderKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		bobInGroup := func(text string, withKey bool) {
			t.Helper()
			distribution, err := bobKey.Distribution()
			if err != nil {
				t.Fatal(err)
			}
			padded, err := message.Encode(rand.Reader, &wire.Message{Conversation: new(text)})
			if err != nil {
				t.Fatal(err)
			}
			ciphertext, err := bobKey.Encrypt(rand.Reader, padded)
			if err != nil {
				t.Fatal(err)
			}
			enc := func(kind string, raw []byte) node.Node {
				return node.Node{Tag: "enc", Attrs: []node.Attr{{Key: "v", Value: node.Text("2")}, {Key: "type", Value: node.Text(kind)}}, Bytes: raw}
			}
			children := []node.Node{enc("skmsg", ciphertext)}
			if withKey {
				part, err := r.bobPhone.EncryptFor(r.keys, w.Phone.JID, message.SenderKeyDistribution(family.JID, distribution))
				if err != nil {
					t.Fatal(err)
				}
				children = append(children, enc(map[signal.MessageType]string{signal.TypeMessage: "msg", signal.TypePreKeyMessage: "pkmsg"}[part.Ciphertext.Type], part.Ciphertext.Bytes))
			}
			r.server.Inbox <- node.Node{Tag: "message", Attrs: []node.Attr{
				{Key: "from", Value: node.Address(family.JID)}, {Key: "participant", Value: node.Address(r.bob)}, {Key: "type", Value: node.Text("text")},
				{Key: "id", Value: node.Text("3EB0" + text)}, {Key: "t", Value: node.Text("1790000000")},
			}, Children: children}
			if got := <-r.received; got.Message.GetConversation() != text || got.Chat != family.JID {
				t.Fatalf("received %+v, want %q in the group", got, text)
			}
		}

		first := open(client.State{Linked: linked})
		if _, err := first.Send(t.Context(), r.bob, &wire.Message{Conversation: new("before the restart")}); err != nil {
			t.Fatal(err)
		}
		if _, _, err := r.bobPhone.Receive(latest()[0]); err != nil {
			t.Fatal(err)
		}
		bobSays("bob, before the restart")
		bobInGroup("bob in the group, before", true)
		if _, err := first.SendGroup(t.Context(), family, &wire.Message{Conversation: new("group, before")}); err != nil {
			t.Fatal(err)
		}
		if _, _, err := r.bobPhone.Receive(latest()[1]); err != nil {
			t.Fatal(err)
		}
		if err := first.Close(); err != nil {
			t.Fatal(err)
		}
		r.mu.Lock()
		saved := r.persisted[len(r.persisted)-1]
		r.mu.Unlock()
		if len(saved.Sessions) == 0 || len(saved.OwnSenderKeys) != 1 || len(saved.OwnSenderKeys[0].Holders) == 0 {
			t.Fatalf("saved %d sessions and %d own sender keys", len(saved.Sessions), len(saved.OwnSenderKeys))
		}

		second := open(saved)
		defer func() { _ = second.Close() }()
		bobSays("bob, after the restart")
		bobInGroup("bob in the group, after", false)
		if _, err := second.Send(t.Context(), r.bob, &wire.Message{Conversation: new("after the restart")}); err != nil {
			t.Fatal(err)
		}
		in, m, err := r.bobPhone.Receive(latest()[2])
		if err != nil || m.GetConversation() != "after the restart" || in.Encs[0].Type != "msg" {
			t.Fatalf("bob read %v (%v): %v; the old session should carry on", m, in.Encs, err)
		}
		if _, err := second.SendGroup(t.Context(), family, &wire.Message{Conversation: new("group, after")}); err != nil {
			t.Fatal(err)
		}
		in, m, err = r.bobPhone.Receive(latest()[3])
		if err != nil || m.GetConversation() != "group, after" || len(in.Encs) != 1 {
			t.Fatalf("bob read %v with %d encs: %v; our sender key should carry on", m, len(in.Encs), err)
		}
	})
}

func TestDownloadingMedia(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		var mu sync.Mutex
		files := map[string][]byte{}
		primaryDown := false
		var asked []string
		cdn := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			mu.Lock()
			defer mu.Unlock()
			asked = append(asked, req.Host+" "+req.URL.RawQuery)
			if req.Header.Get("Origin") != dial.Origin {
				http.Error(w, "no origin", http.StatusForbidden)
				return
			}
			if primaryDown && req.Host == "mmg.whatsapp.net" {
				http.Error(w, "busy", http.StatusServiceUnavailable)
				return
			}
			file, ok := files[req.URL.Path]
			if !ok {
				http.NotFound(w, req)
				return
			}
			_, _ = w.Write(file)
		}))
		r.http = cdn.Client()
		c := r.connect()
		photo := bytes.Repeat([]byte("jpeg"), 5000)
		mediaKey := make([]byte, 32)
		if _, err := rand.Read(mediaKey); err != nil {
			t.Fatal(err)
		}
		sealed, err := media.Encrypt(mediaKey, media.Image, photo)
		if err != nil {
			t.Fatal(err)
		}
		mu.Lock()
		files["/v/t62.7118-24/photo.enc"] = sealed.File
		mu.Unlock()
		out, err := r.bobPhone.Send(r.keys, r.devices, r.account, &wire.Message{ImageMessage: &wire.Message_ImageMessage{
			DirectPath: new("/v/t62.7118-24/photo.enc?ccb=11-4&oh=01AB&oe=6700"), MediaKey: mediaKey,
			FileSha256: sealed.FileSHA256[:], FileEncSha256: sealed.FileEncSHA256[:], Mimetype: new("image/jpeg"), Caption: new("look at this"),
		}})
		if err != nil {
			t.Fatal(err)
		}
		r.server.Inbox <- fakerelay.Deliver(r.bob, "Bob", time.Now(), out)[r.world.Phone.JID]
		got := <-r.received
		ref, ok := media.ReferenceOf(got.Message)
		if !ok || ref.Caption != "look at this" || ref.Mimetype != "image/jpeg" || ref.Type != media.Image {
			t.Fatalf("reference = %+v, %v", ref, ok)
		}
		data, err := c.Download(t.Context(), ref)
		if err != nil || !bytes.Equal(data, photo) {
			t.Fatalf("Download() = %d bytes, %v", len(data), err)
		}
		mu.Lock()
		first := asked[0]
		mu.Unlock()
		for _, part := range []string{"mmg.whatsapp.net ", "ccb=11-4", "oh=01AB", "oe=6700", "mms-type=image", "__wa-mms=", "hash="} {
			if !strings.Contains(first, part) {
				t.Fatalf("request %q lacks %q", first, part)
			}
		}
		mu.Lock()
		primaryDown = true
		mu.Unlock()
		if data, err := c.Download(t.Context(), ref); err != nil || !bytes.Equal(data, photo) {
			t.Fatalf("with the primary host down: %d bytes, %v", len(data), err)
		}
		mu.Lock()
		last := asked[len(asked)-1]
		mu.Unlock()
		if !strings.HasPrefix(last, "media-fallback.fna.whatsapp.net ") {
			t.Fatalf("fallback not used: %q", last)
		}
		evil := ref
		evil.DirectPath = "https://attacker.example/steal"
		if _, err := c.Download(t.Context(), evil); !errors.Is(err, media.ErrPath) {
			t.Fatalf("a direct path to another host: %v", err)
		}
		forged := ref
		forged.FileSHA256 = make([]byte, 32)
		if _, err := c.Download(t.Context(), forged); !errors.Is(err, media.ErrHash) {
			t.Fatalf("a wrong plaintext hash: %v", err)
		}
		missing := ref
		missing.DirectPath = "/v/gone.enc"
		if _, err := c.Download(t.Context(), missing); !errors.Is(err, client.ErrDownload) {
			t.Fatalf("a file no host has: %v", err)
		}
		huge := make([]byte, client.MaxDownload+1)
		mu.Lock()
		files["/v/edge.enc"], files["/v/over.enc"] = huge[:client.MaxDownload], huge
		mu.Unlock()
		edge, over := ref, ref
		edge.DirectPath, over.DirectPath = "/v/edge.enc", "/v/over.enc"
		if _, err := c.Download(t.Context(), edge); err == nil || errors.Is(err, client.ErrDownload) {
			t.Fatalf("a file of exactly the cap must reach decryption: %v", err)
		}
		if _, err := c.Download(t.Context(), over); !errors.Is(err, client.ErrDownload) || !strings.Contains(err.Error(), "more than") {
			t.Fatalf("a file over the cap: %v", err)
		}
	})
}

func historyNotice(t *testing.T, syncType wire.Message_HistorySyncType, conversation string, texts ...string) (*wire.Message_HistorySyncNotification, []byte) {
	t.Helper()
	conv := &wire.Conversation{Id: new(conversation), Name: new("Chat " + conversation)}
	for i, text := range texts {
		conv.Messages = append(conv.Messages, &wire.HistorySyncMsg{Message: &wire.MessageInfo{
			Key:     &wire.MessageKey{RemoteJid: new(conversation), FromMe: new(i%2 == 1), Id: new(fmt.Sprintf("3EB0H%d", i))},
			Message: &wire.Message{Conversation: new(text)}, MessageTimestamp: new(uint64(1790000000 + i)),
		}})
	}
	raw, err := proto.Marshal(&wire.HistorySync{SyncType: wire.HistorySync_HistorySyncType(syncType).Enum(), Conversations: []*wire.Conversation{conv}})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	if _, err := w.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return &wire.Message_HistorySyncNotification{SyncType: syncType.Enum()}, buf.Bytes()
}

func protocolNotice(n *wire.Message_HistorySyncNotification) *wire.Message {
	return &wire.Message{ProtocolMessage: &wire.Message_ProtocolMessage{Type: wire.Message_ProtocolMessage_HISTORY_SYNC_NOTIFICATION.Enum(), HistorySyncNotification: n}}
}

func (r *rig) receiptsFor(id string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var kinds []string
	for _, n := range r.sent {
		if got, _ := n.Attr("id").Text(); n.Tag == "receipt" && got == id {
			kind, _ := n.Attr("type").Text()
			kinds = append(kinds, kind)
		}
	}
	return kinds
}

func TestHistoryFromOurOwnPhone(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		r.history = make(chan history.Chunk, 4)
		var mu sync.Mutex
		files := map[string][]byte{}
		cdn := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			mu.Lock()
			defer mu.Unlock()
			if file, ok := files[req.URL.Path]; ok {
				_, _ = w.Write(file)
				return
			}
			http.NotFound(w, req)
		}))
		r.http = cdn.Client()
		c := r.connect()
		companion := r.world.Phone.JID
		peer := func(from *fakedevice.Device, m *wire.Message) string {
			t.Helper()
			out, err := from.SendPeer(r.keys, companion, m)
			if err != nil {
				t.Fatal(err)
			}
			r.server.Inbox <- fakerelay.DeliverPeer(from.JID, time.Now(), out)
			id, _ := out.Attr("id").Text()
			return id
		}

		bootstrap, inline := historyNotice(t, wire.Message_INITIAL_BOOTSTRAP, r.bob.String(), "old hello", "old reply")
		bootstrap.InitialHistBootstrapInlinePayload = inline
		first := peer(r.ourPhone, protocolNotice(bootstrap))
		chunk := <-r.history
		synctest.Wait()
		if chunk.Type != wire.HistorySync_INITIAL_BOOTSTRAP || len(chunk.Chats) != 1 || chunk.Chats[0].JID != r.bob || len(chunk.Messages) != 2 ||
			chunk.Messages[0].Author != r.bob || chunk.Messages[1].Author != c.Self() || !chunk.Messages[1].FromMe {
			t.Fatalf("bootstrap chunk = %+v", chunk)
		}
		if got := r.receiptsFor(first); !slices.Equal(got, []string{"peer_msg", "hist_sync"}) {
			t.Fatalf("receipts for the inline chunk = %q", got)
		}

		recent, blob := historyNotice(t, wire.Message_RECENT, "120363000000000021@g.us")
		mediaKey := bytes.Repeat([]byte{5}, media.KeySize)
		sealed, err := media.Encrypt(mediaKey, media.History, blob)
		if err != nil {
			t.Fatal(err)
		}
		mu.Lock()
		files["/v/t62.hist/recent.enc"] = sealed.File
		mu.Unlock()
		recent.DirectPath, recent.MediaKey = new("/v/t62.hist/recent.enc"), mediaKey
		recent.FileSha256, recent.FileEncSha256 = sealed.FileSHA256[:], sealed.FileEncSHA256[:]
		second := peer(r.ourPhone, protocolNotice(recent))
		chunk = <-r.history
		synctest.Wait()
		if chunk.Type != wire.HistorySync_RECENT || len(chunk.Chats) != 1 || chunk.Chats[0].Name != "Chat 120363000000000021@g.us" {
			t.Fatalf("downloaded chunk = %+v", chunk)
		}
		if got := r.receiptsFor(second); !slices.Equal(got, []string{"peer_msg", "hist_sync"}) {
			t.Fatalf("receipts for the downloaded chunk = %q", got)
		}

		gone, _ := historyNotice(t, wire.Message_RECENT, r.bob.String())
		gone.DirectPath, gone.MediaKey = new("/v/t62.hist/expired.enc"), mediaKey
		third := peer(r.ourPhone, protocolNotice(gone))
		synctest.Wait()
		if got := r.receiptsFor(third); !slices.Equal(got, []string{"server-error"}) {
			t.Fatalf("a chunk gone from the media servers must be asked for again, not acknowledged: %q", got)
		}
		r.mu.Lock()
		var ask node.Node
		for _, n := range r.sent {
			if id, _ := n.Attr("id").Text(); n.Tag == "receipt" && id == third {
				ask = n
			}
		}
		r.mu.Unlock()
		to, _ := ask.Attr("to").JID()
		category, _ := ask.Attr("category").Text()
		encrypted, _ := ask.Child("encrypt")
		var request wire.ServerErrorReceipt
		if to != r.account || category != "peer" || len(encrypted.Children) != 2 ||
			mediaretry.Open(mediaKey, third, encrypted.Children[0].Bytes, encrypted.Children[1].Bytes, &request) != nil || request.GetStanzaId() != third {
			t.Fatalf("the request to upload the chunk again = %s", ask)
		}

		forged, forgedInline := historyNotice(t, wire.Message_INITIAL_BOOTSTRAP, r.bob.String(), "fake history")
		forged.InitialHistBootstrapInlinePayload = forgedInline
		out, err := r.bobPhone.Send(r.keys, r.devices, r.account, protocolNotice(forged))
		if err != nil {
			t.Fatal(err)
		}
		r.server.Inbox <- fakerelay.Deliver(r.bob, "Bob", time.Now(), out)[companion]
		synctest.Wait()
		forgedID, _ := out.Attr("id").Text()
		if got := r.receiptsFor(forgedID); !slices.Equal(got, []string{""}) {
			t.Fatalf("receipts for Bob's history notice = %q, want a plain delivery receipt", got)
		}
		select {
		case chunk := <-r.history:
			t.Fatalf("history from Bob was accepted: %+v", chunk)
		case m := <-r.received:
			t.Fatalf("a protocol message surfaced as a chat message: %v", m)
		default:
		}
	})
}

func TestHistoryIsAcknowledgedWhenNobodyWantsIt(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		r.connect()
		notice, inline := historyNotice(t, wire.Message_INITIAL_BOOTSTRAP, r.bob.String(), "old")
		notice.InitialHistBootstrapInlinePayload = inline
		out, err := r.ourPhone.SendPeer(r.keys, r.world.Phone.JID, protocolNotice(notice))
		if err != nil {
			t.Fatal(err)
		}
		r.server.Inbox <- fakerelay.DeliverPeer(r.ourPhone.JID, time.Now(), out)
		synctest.Wait()
		id, _ := out.Attr("id").Text()
		if got := r.receiptsFor(id); !slices.Equal(got, []string{"peer_msg"}) {
			t.Fatalf("receipts = %q", got)
		}
	})
}

type syncStore struct {
	mu        sync.Mutex
	states    map[string]appstate.State
	mutations map[string][]appstate.Mutation
	saves     int
}

func (s *syncStore) SyncState(_ context.Context, collection string) (appstate.State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.states[collection], nil
}

func (s *syncStore) SaveSync(_ context.Context, collection string, state appstate.State, mutations []appstate.Mutation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.states[collection] = state
	s.mutations[collection] = append(s.mutations[collection], mutations...)
	s.saves++
	return nil
}

func (s *syncStore) names(collection string) map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]string{}
	for _, m := range s.mutations[collection] {
		if m.Operation == wire.SyncdMutation_REMOVE {
			delete(out, m.Index[1])
			continue
		}
		out[m.Index[1]] = m.Value.GetContactAction().GetFullName()
	}
	return out
}

func keyShare(id, data []byte) *wire.Message {
	return &wire.Message{ProtocolMessage: &wire.Message_ProtocolMessage{
		Type: wire.Message_ProtocolMessage_APP_STATE_SYNC_KEY_SHARE.Enum(),
		AppStateSyncKeyShare: &wire.Message_AppStateSyncKeyShare{Keys: []*wire.Message_AppStateSyncKey{{
			KeyId: &wire.Message_AppStateSyncKeyId{KeyId: id}, KeyData: &wire.Message_AppStateSyncKeyData{KeyData: data, Timestamp: new(int64(1790000000))},
		}}},
	}}
}

func TestContactsFromTheAddressBook(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		var mu sync.Mutex
		files := map[string][]byte{}
		cdn := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			mu.Lock()
			defer mu.Unlock()
			if file, ok := files[req.URL.Path]; ok {
				_, _ = w.Write(file)
				return
			}
			http.NotFound(w, req)
		}))
		r.http = cdn.Client()
		host := func(name string, plain []byte) (*wire.ExternalBlobReference, error) {
			mediaKey := bytes.Repeat([]byte{byte(len(files) + 1)}, media.KeySize)
			sealed, err := media.Encrypt(mediaKey, media.AppState, plain)
			if err != nil {
				return nil, err
			}
			mu.Lock()
			defer mu.Unlock()
			path := fmt.Sprintf("/v/appstate/%s-%d.enc", name, len(files))
			files[path] = sealed.File
			return &wire.ExternalBlobReference{MediaKey: mediaKey, DirectPath: new(path), FileSha256: sealed.FileSHA256[:], FileEncSha256: sealed.FileEncSHA256[:]}, nil
		}
		keyID, keyData := []byte{0, 0, 0, 1}, bytes.Repeat([]byte{7}, 32)
		book, err := fakeappstate.New(keyID, keyData, host)
		if err != nil {
			t.Fatal(err)
		}
		person := func(n string) *wire.SyncActionValue {
			return &wire.SyncActionValue{ContactAction: &wire.SyncActionValue_ContactAction{FullName: new(n)}}
		}
		for _, c := range []fakeappstate.Change{
			fakeappstate.Set(person("Mama"), "contact", "40744444444@s.whatsapp.net"),
			fakeappstate.Set(person("Bob Builder"), "contact", r.bob.String()),
			fakeappstate.Set(person("Old Friend"), "contact", "40755555555@s.whatsapp.net"),
		} {
			if _, err := book.Patch(appstate.CriticalUnblockLow, c); err != nil {
				t.Fatal(err)
			}
		}
		r.server.AppState = book
		kept := &syncStore{states: map[string]appstate.State{}, mutations: map[string][]appstate.Mutation{}}
		r.appState = kept
		r.connect()
		synctest.Wait()
		if book.Requests != 0 {
			t.Fatal("synced before any key was shared")
		}
		companion := r.world.Phone.JID
		deliverPeer := func(m *wire.Message) {
			t.Helper()
			out, err := r.ourPhone.SendPeer(r.keys, companion, m)
			if err != nil {
				t.Fatal(err)
			}
			r.server.Inbox <- fakerelay.DeliverPeer(r.ourPhone.JID, time.Now(), out)
		}
		out, err := r.bobPhone.Send(r.keys, r.devices, r.account, keyShare([]byte{6, 6}, bytes.Repeat([]byte{6}, 32)))
		if err != nil {
			t.Fatal(err)
		}
		r.server.Inbox <- fakerelay.Deliver(r.bob, "Bob", time.Now(), out)[companion]
		synctest.Wait()
		if book.Requests != 0 {
			t.Fatal("a key shared by Bob started a sync")
		}

		deliverPeer(keyShare(keyID, keyData))
		synctest.Wait()
		contacts := kept.names(appstate.CriticalUnblockLow)
		if len(contacts) != 3 || contacts["40744444444@s.whatsapp.net"] != "Mama" || kept.states[appstate.CriticalUnblockLow].Version != 3 {
			t.Fatalf("after the first sync: %v at v%d", contacts, kept.states[appstate.CriticalUnblockLow].Version)
		}
		for _, name := range appstate.Collections() {
			if _, ok := kept.states[name]; !ok {
				t.Errorf("collection %s was not synced", name)
			}
		}
		r.mu.Lock()
		last := r.persisted[len(r.persisted)-1]
		r.mu.Unlock()
		if len(last.SyncKeys) != 1 || !bytes.Equal(last.SyncKeys[0].Data, keyData) {
			t.Fatalf("sync keys were not kept: %+v", last.SyncKeys)
		}

		if _, err := book.Patch(appstate.CriticalUnblockLow,
			fakeappstate.Set(person("Carol Mihai"), "contact", "40733333333@s.whatsapp.net"),
			fakeappstate.Remove("contact", "40755555555@s.whatsapp.net"),
		); err != nil {
			t.Fatal(err)
		}
		requests := book.Requests
		r.server.Inbox <- node.Node{Tag: "notification", Attrs: []node.Attr{{Key: "from", Value: node.Address(node.JID{Server: node.ServerUser})}, {Key: "type", Value: node.Text("server_sync")}, {Key: "id", Value: node.Text("n1")}},
			Children: []node.Node{{Tag: "collection", Attrs: []node.Attr{{Key: "name", Value: node.Text(appstate.CriticalUnblockLow)}, {Key: "version", Value: node.Text("4")}}}}}
		synctest.Wait()
		contacts = kept.names(appstate.CriticalUnblockLow)
		if book.Requests != requests+1 || len(contacts) != 3 || contacts["40733333333@s.whatsapp.net"] != "Carol Mihai" || contacts["40755555555@s.whatsapp.net"] != "" || kept.states[appstate.CriticalUnblockLow].Version != 4 {
			t.Fatalf("after the notification: %v at v%d", contacts, kept.states[appstate.CriticalUnblockLow].Version)
		}

		kept.mu.Lock()
		broken := kept.states[appstate.CriticalUnblockLow]
		broken.Hash[0] ^= 1
		kept.states[appstate.CriticalUnblockLow] = broken
		kept.mutations[appstate.CriticalUnblockLow] = nil
		kept.mu.Unlock()
		if _, err := book.Patch(appstate.CriticalUnblockLow, fakeappstate.Set(person("Dan"), "contact", "40766666666@s.whatsapp.net")); err != nil {
			t.Fatal(err)
		}
		r.server.Inbox <- node.Node{Tag: "notification", Attrs: []node.Attr{{Key: "type", Value: node.Text("server_sync")}, {Key: "id", Value: node.Text("n2")}},
			Children: []node.Node{{Tag: "collection", Attrs: []node.Attr{{Key: "name", Value: node.Text(appstate.CriticalUnblockLow)}}}}}
		synctest.Wait()
		contacts = kept.names(appstate.CriticalUnblockLow)
		if len(contacts) != 4 || contacts["40766666666@s.whatsapp.net"] != "Dan" || kept.states[appstate.CriticalUnblockLow].Version != 5 {
			t.Fatalf("a corrupted local state must be rebuilt from a snapshot: %v at v%d", contacts, kept.states[appstate.CriticalUnblockLow].Version)
		}

		stranger, err := fakeappstate.New([]byte{9, 9}, bytes.Repeat([]byte{8}, 32), host)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := stranger.Patch(appstate.Regular, fakeappstate.Set(person("X"), "contact", "1@s.whatsapp.net")); err != nil {
			t.Fatal(err)
		}
		r.server.AppState = stranger
		saves := kept.saves
		r.server.Inbox <- node.Node{Tag: "notification", Attrs: []node.Attr{{Key: "type", Value: node.Text("server_sync")}, {Key: "id", Value: node.Text("n3")}},
			Children: []node.Node{{Tag: "collection", Attrs: []node.Attr{{Key: "name", Value: node.Text(appstate.Regular)}}}, {Tag: "other"}}}
		synctest.Wait()
		if kept.saves != saves {
			t.Fatal("a collection under a key we were never given was saved")
		}
		asked := r.deliveredTo(r.account)
		if len(asked) != 1 {
			t.Fatalf("%d messages to our phone, want one key request", len(asked))
		}
		_, request, err := r.ourPhone.Receive(asked[0])
		if ids := request.GetProtocolMessage().GetAppStateSyncKeyRequest().GetKeyIds(); err != nil || len(ids) != 1 || !bytes.Equal(ids[0].GetKeyId(), []byte{9, 9}) {
			t.Fatalf("our phone got %v: %v", request, err)
		}
		deliverPeer(keyShare([]byte{9, 9}, bytes.Repeat([]byte{8}, 32)))
		synctest.Wait()
		if got := kept.names(appstate.Regular); got["1@s.whatsapp.net"] != "X" {
			t.Fatalf("after our phone shared the missing key: %v", got)
		}
		before := stranger.Requests
		r.server.Inbox <- node.Node{Tag: "notification", Attrs: []node.Attr{{Key: "type", Value: node.Text("devices")}, {Key: "id", Value: node.Text("n4")}},
			Children: []node.Node{{Tag: "collection", Attrs: []node.Attr{{Key: "name", Value: node.Text(appstate.Regular)}}}}}
		synctest.Wait()
		if stranger.Requests != before {
			t.Fatalf("%d sync requests; only server_sync notifications start one", stranger.Requests-before)
		}
	})
}

func TestAppStateSyncStopsAndFollowsUp(t *testing.T) {
	for _, tt := range []struct {
		name           string
		more, conflict int
		requests       int
	}{
		{name: "a server that always has more is asked eight times", more: 1000, requests: 8},
		{name: "a conflict with more patches is followed up once", more: 1, conflict: 1, requests: 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				r := newRig(t)
				cdn := fakecdn.New()
				r.http = httptest.NewTestServer(t, cdn).Client()
				host := func(name string, plain []byte) (*wire.ExternalBlobReference, error) {
					mediaKey := bytes.Repeat([]byte{3}, media.KeySize)
					sealed, err := media.Encrypt(mediaKey, media.AppState, plain)
					if err != nil {
						return nil, err
					}
					path := "/v/appstate/" + name + ".enc"
					cdn.Put(path, sealed.File)
					return &wire.ExternalBlobReference{MediaKey: mediaKey, DirectPath: new(path), FileSha256: sealed.FileSHA256[:], FileEncSha256: sealed.FileEncSHA256[:]}, nil
				}
				keyID, keyData := []byte{1}, bytes.Repeat([]byte{2}, 32)
				book, err := fakeappstate.New(keyID, keyData, host)
				if err != nil {
					t.Fatal(err)
				}
				book.MoreFor, book.ConflictFor = tt.more, tt.conflict
				r.server.AppState = book
				kept := &syncStore{states: map[string]appstate.State{}, mutations: map[string][]appstate.Mutation{}}
				r.appState = kept
				r.connect()
				out, err := r.ourPhone.SendPeer(r.keys, r.world.Phone.JID, keyShare(keyID, keyData))
				if err != nil {
					t.Fatal(err)
				}
				r.server.Inbox <- fakerelay.DeliverPeer(r.ourPhone.JID, time.Now(), out)
				synctest.Wait()
				if book.Requests != tt.requests {
					t.Fatalf("%d sync requests, want %d", book.Requests, tt.requests)
				}
				r.server.Inbox <- node.Node{Tag: "notification", Attrs: []node.Attr{{Key: "type", Value: node.Text("server_sync")}, {Key: "id", Value: node.Text("empty")}}}
				synctest.Wait()
				if book.Requests != tt.requests {
					t.Fatal("a server_sync naming no collection started a sync")
				}
			})
		})
	}
}

func TestUploadingMedia(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		cdn := fakecdn.New()
		r.http = httptest.NewTestServer(t, cdn).Client()
		c := r.connect()
		photo := bytes.Repeat([]byte("jpeg!"), 9000)
		up, err := c.Upload(t.Context(), media.Image, photo)
		if err != nil {
			t.Fatal(err)
		}
		if len(cdn.Uploads) != 1 || cdn.Uploads[0].Type != "image" || cdn.Uploads[0].Refused != "" || !strings.HasPrefix(up.URL, "https://mmg.whatsapp.net/") ||
			len(up.MediaKey) != media.KeySize || up.FileLength != uint64(len(photo)) {
			t.Fatalf("uploaded %+v, cdn saw %+v", up, cdn.Uploads)
		}
		ref := media.Reference{Type: media.Image, DirectPath: up.DirectPath, MediaKey: up.MediaKey, FileSHA256: up.FileSHA256, FileEncSHA256: up.FileEncSHA256}
		if back, err := c.Download(t.Context(), ref); err != nil || !bytes.Equal(back, photo) {
			t.Fatalf("downloading what was uploaded: %d bytes, %v", len(back), err)
		}
		again, err := c.Upload(t.Context(), media.Image, photo)
		if err != nil || bytes.Equal(again.MediaKey, up.MediaKey) || bytes.Equal(again.FileEncSHA256, up.FileEncSHA256) {
			t.Fatalf("every upload needs a fresh media key: %v", err)
		}
		cdn.SetDown("mmg.whatsapp.net", true)
		voice := []byte("OggS voice")
		fallback, err := c.Upload(t.Context(), media.Voice, voice)
		if err != nil || !strings.HasPrefix(fallback.URL, "https://media-fallback.fna.whatsapp.net/") || cdn.Uploads[len(cdn.Uploads)-1].Type != "ptt" {
			t.Fatalf("with the primary host down: %+v, %v", fallback, err)
		}
		cdn.SetDown("media-fallback.fna.whatsapp.net", true)
		if _, err := c.Upload(t.Context(), media.Document, []byte("x")); !errors.Is(err, client.ErrUpload) {
			t.Fatalf("with every host down: %v", err)
		}
		cdn.SetDown("mmg.whatsapp.net", false)
		cdn.SetDown("media-fallback.fna.whatsapp.net", false)
		cdn.ExpectAuth("rotated")
		if _, err := c.Upload(t.Context(), media.Document, []byte("x")); !errors.Is(err, client.ErrUpload) || !strings.Contains(err.Error(), "401") {
			t.Fatalf("with a stale media auth: %v", err)
		}
		if _, err := c.Upload(t.Context(), media.Document, make([]byte, client.MaxUpload+1)); !errors.Is(err, client.ErrTooLarge) {
			t.Fatalf("a file over the cap: %v", err)
		}
		if edge, err := c.Upload(t.Context(), media.Document, make([]byte, client.MaxUpload)); !errors.Is(err, client.ErrUpload) || edge.URL != "" {
			t.Fatalf("a file of exactly the cap must be tried: %v", err)
		}
	})
}

func (r *rig) mediaConns() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, n := range r.sent {
		if xmlns, _ := n.Attr("xmlns").Text(); n.Tag == "iq" && xmlns == "w:m" {
			count++
		}
	}
	return count
}

func TestExpiredMediaIsUploadedAgainByThePhone(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		cdn := fakecdn.New()
		r.http = httptest.NewTestServer(t, cdn).Client()
		photo := bytes.Repeat([]byte("old photo "), 500)
		mediaKey := bytes.Repeat([]byte{4}, media.KeySize)
		sealed, err := media.Encrypt(mediaKey, media.Image, photo)
		if err != nil {
			t.Fatal(err)
		}
		answer := func(n node.Node) (node.Node, bool) {
			kind, _ := n.Attr("type").Text()
			id, _ := n.Attr("id").Text()
			if n.Tag != "receipt" || kind != "server-error" {
				return node.Node{}, false
			}
			rmr, _ := n.Child("rmr")
			encrypted, _ := n.Child("encrypt")
			if to, _ := n.Attr("to").JID(); to != r.world.Phone.LID.WithoutDevice() {
				t.Errorf("the request went to %v, want our lid %v", to, r.world.Phone.LID)
			}
			var request wire.ServerErrorReceipt
			if err := mediaretry.Open(mediaKey, id, encrypted.Children[0].Bytes, encrypted.Children[1].Bytes, &request); err != nil || request.GetStanzaId() != id {
				t.Errorf("the phone could not read the request: %v", err)
			}
			reply := node.Node{Tag: "notification", Attrs: []node.Attr{{Key: "from", Value: node.Address(r.account)}, {Key: "type", Value: node.Text("mediaretry")}, {Key: "id", Value: node.Text(id)}},
				Children: []node.Node{rmr}}
			switch id {
			case "3EB0FAIL":
				reply.Children = append(reply.Children, node.Node{Tag: "error", Attrs: []node.Attr{{Key: "code", Value: node.Text("2")}}})
				return reply, true
			case "3EB0NOPE":
				ct, iv, err := mediaretry.Seal(rand.Reader, mediaKey, id, &wire.MediaRetryNotification{StanzaId: new(id), Result: wire.MediaRetryNotification_NOT_FOUND.Enum()})
				if err != nil {
					t.Error(err)
				}
				reply.Children = append(reply.Children, node.Node{Tag: "encrypt", Children: []node.Node{{Tag: "enc_p", Bytes: ct}, {Tag: "enc_iv", Bytes: iv}}})
				return reply, true
			case "3EB0SILENT":
				return node.Node{}, false
			}
			cdn.Put("/v/t62.7118-24/renewed.enc", sealed.File)
			ct, iv, err := mediaretry.Seal(rand.Reader, mediaKey, id, &wire.MediaRetryNotification{
				StanzaId: new(id), DirectPath: new("/v/t62.7118-24/renewed.enc?ccb=11-4"), Result: wire.MediaRetryNotification_SUCCESS.Enum(),
			})
			if err != nil {
				t.Error(err)
			}
			reply.Children = append(reply.Children, node.Node{Tag: "encrypt", Children: []node.Node{{Tag: "enc_p", Bytes: ct}, {Tag: "enc_iv", Bytes: iv}}})
			return reply, true
		}
		received := r.server.Received
		r.server.Received = func(n node.Node) {
			received(n)
			if reply, ok := answer(n); ok {
				go func() { r.server.Inbox <- reply }()
			}
		}
		c := r.connect()
		expired := media.Reference{Type: media.Image, DirectPath: "/v/t62.7118-24/expired.enc", MediaKey: mediaKey, FileSHA256: sealed.FileSHA256[:], FileEncSHA256: sealed.FileEncSHA256[:]}
		if _, err := c.Download(t.Context(), expired); !errors.Is(err, client.ErrGone) {
			t.Fatalf("downloading an expired file: %v", err)
		}
		target := mediaretry.Target{Chat: r.bob, Participant: r.bob}
		path, err := c.RetryMedia(t.Context(), expired, "3EB0EXPIRED", target)
		if err != nil || path != "/v/t62.7118-24/renewed.enc?ccb=11-4" {
			t.Fatalf("RetryMedia() = %q, %v", path, err)
		}
		renewed := expired
		renewed.DirectPath = path
		if again, err := c.Download(t.Context(), renewed); err != nil || !bytes.Equal(again, photo) {
			t.Fatalf("downloading after the phone uploaded it again: %d bytes, %v", len(again), err)
		}
		for _, id := range []string{"3EB0FAIL", "3EB0NOPE"} {
			if _, err := c.RetryMedia(t.Context(), expired, id, target); !errors.Is(err, client.ErrRetry) {
				t.Fatalf("RetryMedia(%s) = %v", id, err)
			}
		}
		waiting, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		defer cancel()
		if _, err := c.RetryMedia(waiting, expired, "3EB0SILENT", target); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("a phone that never answers: %v", err)
		}
		r.server.Inbox <- node.Node{Tag: "notification", Attrs: []node.Attr{{Key: "type", Value: node.Text("mediaretry")}, {Key: "id", Value: node.Text("3EB0NOBODY")}}}
		synctest.Wait()

		before := len(r.sent)
		if err := c.MarkRead(t.Context(), r.bob, r.bob, []string{"3EB0A1", "3EB0A2"}); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		r.mu.Lock()
		read := slices.ContainsFunc(r.sent[before:], func(n node.Node) bool { kind, _ := n.Attr("type").Text(); return n.Tag == "receipt" && kind == "read" })
		r.mu.Unlock()
		if !read {
			t.Fatal("no read receipt was sent")
		}
		if err := c.Close(); err != nil {
			t.Fatal(err)
		}
		if err := c.MarkRead(t.Context(), r.bob, r.bob, []string{"3EB0A3"}); err == nil {
			t.Fatal("marking read on a closed connection reported success")
		}
	})
}

func TestAStaleMediaAuthIsFetchedAgain(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		cdn := fakecdn.New()
		r.http = httptest.NewTestServer(t, cdn).Client()
		c := r.connect()
		for range 2 {
			if _, err := c.Upload(t.Context(), media.Document, []byte("x")); err != nil {
				t.Fatal(err)
			}
		}
		if got := r.mediaConns(); got != 1 {
			t.Fatalf("%d media_conn queries for two uploads, want the cached one", got)
		}
		cdn.ExpectAuth("rotated")
		if _, err := c.Upload(t.Context(), media.Document, []byte("x")); err == nil {
			t.Fatal("an upload with a stale auth succeeded")
		}
		cdn.ExpectAuth(fakecdn.Auth)
		if _, err := c.Upload(t.Context(), media.Document, []byte("x")); err != nil {
			t.Fatal(err)
		}
		if got := r.mediaConns(); got != 2 {
			t.Fatalf("%d media_conn queries; a 401 must drop the cached auth", got)
		}
	})
}

func TestEditsAndDeletesReachTheApp(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		r.connect()
		companion := r.world.Phone.JID
		deliver := func(m *wire.Message) {
			t.Helper()
			out, err := r.bobPhone.Send(r.keys, r.devices, r.account, m)
			if err != nil {
				t.Fatal(err)
			}
			stanza := fakerelay.Deliver(r.bob, "Bob", time.Now(), out)[companion]
			r.server.Inbox <- stanza
		}
		key := &wire.MessageKey{RemoteJid: new(r.account.String()), FromMe: new(true), Id: new("3EB0ORIG")}
		deliver(&wire.Message{ProtocolMessage: &wire.Message_ProtocolMessage{Key: key, Type: wire.Message_ProtocolMessage_MESSAGE_EDIT.Enum(), EditedMessage: &wire.Message{Conversation: new("fixed")}}})
		got := <-r.received
		if got.Edit != message.EditMessage || got.Message.GetProtocolMessage().GetEditedMessage().GetConversation() != "fixed" {
			t.Fatalf("edit = %+v", got)
		}
		deliver(&wire.Message{ProtocolMessage: &wire.Message_ProtocolMessage{Key: key, Type: wire.Message_ProtocolMessage_REVOKE.Enum()}})
		if got := <-r.received; got.Edit != message.EditSenderRevoke || got.Message.GetProtocolMessage().GetType() != wire.Message_ProtocolMessage_REVOKE {
			t.Fatalf("delete = %+v", got)
		}
		deliver(&wire.Message{ProtocolMessage: &wire.Message_ProtocolMessage{Type: wire.Message_ProtocolMessage_REVOKE.Enum()}})
		deliver(&wire.Message{ProtocolMessage: &wire.Message_ProtocolMessage{Key: key}})
		deliver(&wire.Message{Conversation: new("plain")})
		if got := <-r.received; got.Message.GetConversation() != "plain" || got.Edit != message.EditNone {
			t.Fatalf("a protocol message without its key or type reached the app: %+v", got)
		}
	})
}

func TestReceiptsReachTheApp(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		r.receipts = make(chan message.Receipt, 4)
		r.connect()
		text := func(k, v string) node.Attr { return node.Attr{Key: k, Value: node.Text(v)} }
		r.server.Inbox <- node.Node{Tag: "receipt", Attrs: []node.Attr{{Key: "from", Value: node.Address(r.bob)}, text("id", "3EB0A1"), text("type", "read"), text("t", "1790000000")}}
		r.server.Inbox <- node.Node{Tag: "receipt", Attrs: []node.Attr{text("id", "no-from")}}
		r.server.Inbox <- node.Node{Tag: "receipt", Attrs: []node.Attr{{Key: "from", Value: node.Address(r.bob)}, text("id", "3EB0A2"), text("type", "read-self")}}
		synctest.Wait()
		first, second := <-r.receipts, <-r.receipts
		if first.Ack != message.AckRead || first.Self || !slices.Equal(first.IDs, []string{"3EB0A1"}) || second.Ack != message.AckRead || !second.Self {
			t.Fatalf("receipts = %+v, %+v", first, second)
		}
		select {
		case extra := <-r.receipts:
			t.Fatalf("a malformed receipt reached the app: %+v", extra)
		default:
		}
		r.mu.Lock()
		acks := 0
		for _, n := range r.sent {
			if class, _ := n.Attr("class").Text(); n.Tag == "ack" && class == "receipt" {
				acks++
			}
		}
		r.mu.Unlock()
		if acks < 3 {
			t.Fatalf("%d receipts acknowledged, want all three", acks)
		}
	})
}

func TestKeepalivePingsFindADeadConnection(t *testing.T) {
	t.Parallel()
	pings := func(r *rig) int {
		r.mu.Lock()
		defer r.mu.Unlock()
		count := 0
		for _, n := range r.sent {
			if xmlns, _ := n.Attr("xmlns").Text(); n.Tag == "iq" && xmlns == "w:p" {
				count++
			}
		}
		return count
	}
	t.Run("answered", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			r := newRig(t)
			r.keepAlive = 0
			c := r.connect()
			time.Sleep(5 * time.Minute)
			synctest.Wait()
			select {
			case <-c.Done():
				t.Fatalf("a healthy connection ended: %v", c.Err())
			default:
			}
			if got := pings(r); got < 10 || got > 15 {
				t.Fatalf("%d pings in five minutes, want one every 20 to 30 seconds", got)
			}
			_ = c.Close()
		})
	})
	t.Run("unanswered", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			r := newRig(t)
			r.keepAlive = 0
			r.server.Drop = func(n node.Node) bool {
				xmlns, _ := n.Attr("xmlns").Text()
				return n.Tag == "iq" && xmlns == "w:p"
			}
			c := r.connect()
			start := time.Now()
			select {
			case <-c.Done():
			case <-time.After(2 * time.Minute):
				t.Fatal("the connection stayed up with its pings unanswered")
			}
			if waited := time.Since(start); waited < 75*time.Second || waited > 2*time.Minute || !errors.Is(c.Err(), client.ErrClosed) || !strings.Contains(c.Err().Error(), "keepalive") {
				t.Fatalf("after %s: %v", waited, c.Err())
			}
		})
	})
	t.Run("after the computer slept", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			r := newRig(t)
			r.keepAlive = 0
			var asleep atomic.Bool
			var ahead atomic.Int64
			r.now = func() time.Time { return time.Now().Add(time.Duration(ahead.Load())) }
			r.server.Drop = func(n node.Node) bool {
				xmlns, _ := n.Attr("xmlns").Text()
				return asleep.Load() && n.Tag == "iq" && xmlns == "w:p"
			}
			c := r.connect()
			time.Sleep(time.Minute)
			synctest.Wait()
			if got := pings(r); got < 1 {
				t.Fatalf("%d pings in the first minute", got)
			}
			asleep.Store(true)
			ahead.Store(int64(time.Hour))
			woke := time.Now()
			select {
			case <-c.Done():
			case <-time.After(time.Minute):
				t.Fatal("a connection that slept through an hour was kept")
			}
			if waited := time.Since(woke); waited > 20*time.Second {
				t.Fatalf("noticed the dead connection %s after waking", waited)
			}
		})
	})
}

func TestServerPingsAreAnswered(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		c := r.connect()
		server := node.Address(node.JID{Server: node.ServerUser})
		r.server.Inbox <- node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "from", Value: server}, {Key: "id", Value: node.Text("ping-7")}, {Key: "type", Value: node.Text("get")}, {Key: "xmlns", Value: node.Text("urn:xmpp:ping")}}}
		r.server.Inbox <- node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "from", Value: server}, {Key: "id", Value: node.Text("other-8")}, {Key: "type", Value: node.Text("get")}, {Key: "xmlns", Value: node.Text("w:other")}}}
		synctest.Wait()
		r.mu.Lock()
		var answered []string
		for _, n := range r.sent {
			if kind, _ := n.Attr("type").Text(); n.Tag == "iq" && kind == "result" {
				id, _ := n.Attr("id").Text()
				to, _ := n.Attr("to").JID()
				answered = append(answered, id+" to "+to.String())
			}
		}
		r.mu.Unlock()
		if !slices.Equal(answered, []string{"ping-7 to s.whatsapp.net"}) {
			t.Fatalf("answered %v", answered)
		}
		_ = c.Close()
	})
}

func TestARepliesFromAPrivateIDOnTheSessionStartedByNumber(t *testing.T) {
	bobLID := node.JID{User: "99001", Server: node.ServerLID}
	for _, tt := range []struct {
		name  string
		teach func(stanza node.Node) node.Node
		seed  map[node.JID]node.JID
	}{
		{name: "the message names the number", teach: func(n node.Node) node.Node {
			return n.With("sender_pn", node.Address(node.JID{User: "40722222222", Server: node.ServerUser}))
		}},
		{name: "the pairing is already known", teach: func(n node.Node) node.Node { return n }, seed: map[node.JID]node.JID{bobLID: {User: "40722222222", Server: node.ServerUser}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				r := newRig(t)
				r.lids = tt.seed
				c := r.connect()
				self := r.world.Phone.JID
				if _, err := c.Send(t.Context(), r.bob, &wire.Message{Conversation: new("hello bob")}); err != nil {
					t.Fatal(err)
				}
				r.mu.Lock()
				toBob := r.delivered[r.bob]
				r.mu.Unlock()
				if _, _, err := r.bobPhone.Receive(toBob[0]); err != nil {
					t.Fatal(err)
				}
				reply, err := r.bobPhone.Send(r.keys, r.devices, r.account, &wire.Message{Conversation: new("from my private id")})
				if err != nil {
					t.Fatal(err)
				}
				r.server.Inbox <- tt.teach(fakerelay.Deliver(bobLID, "Bob", time.Now(), reply)[self])
				select {
				case got := <-r.received:
					if got.Message.GetConversation() != "from my private id" {
						t.Fatalf("received %+v", got)
					}
				case <-time.After(time.Minute):
					t.Fatal("the reply from the private id never decrypted")
				}
				synctest.Wait()
				r.mu.Lock()
				defer r.mu.Unlock()
				for _, n := range r.sent {
					if kind, _ := n.Attr("type").Text(); n.Tag == "receipt" && kind == "retry" {
						t.Fatalf("asked for a retry: %s", n)
					}
				}
				last := r.persisted[len(r.persisted)-1]
				for _, s := range last.Sessions {
					if s.Device.User == r.bob.User {
						t.Fatalf("a session is still kept under bob's number: %v", s.Device)
					}
				}
			})
		})
	}
}

func (r *rig) deliveredTo(device node.JID) []node.Node {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.delivered[device])
}

func TestRetriesAreAnsweredWithTheMessageAgain(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		c := r.connect()
		ask := func(stanza node.Node, count int) node.Node {
			t.Helper()
			before := len(r.deliveredTo(r.bob))
			receipt, err := r.bobPhone.AskAgain(stanza, count)
			if err != nil {
				t.Fatal(err)
			}
			r.server.Inbox <- fakerelay.DeliverReceipt(r.bob, receipt)
			synctest.Wait()
			got := r.deliveredTo(r.bob)
			if len(got) != before+1 {
				t.Fatalf("retry %d: %d new deliveries to bob, want 1", count, len(got)-before)
			}
			return got[before]
		}
		check := func(resend node.Node, id string, count int, text string) {
			t.Helper()
			enc, _ := resend.Child("enc")
			if got, _ := resend.Attr("id").Text(); got != id {
				t.Fatalf("resend id %s, want %s", got, id)
			}
			if got, _ := enc.Attr("count").Text(); got != fmt.Sprint(count) {
				t.Fatalf("resend count %q, want %d", got, count)
			}
			_, m, err := r.bobPhone.Receive(resend)
			if err != nil || m.GetConversation() != text {
				t.Fatalf("bob read %v: %v", m, err)
			}
		}

		first, err := c.Send(t.Context(), r.bob, &wire.Message{Conversation: new("did you get this")})
		if err != nil {
			t.Fatal(err)
		}
		original := r.deliveredTo(r.bob)[0]
		check(ask(original, 1), first, 1, "did you get this")

		second, err := c.Send(t.Context(), r.bob, &wire.Message{Conversation: new("and this")})
		if err != nil {
			t.Fatal(err)
		}
		resend := ask(r.deliveredTo(r.bob)[2], 2)
		if enc, _ := resend.Child("enc"); enc.Attr("type").String() != "pkmsg" {
			t.Fatalf("a retry with keys was answered on the old session: %s", resend)
		}
		check(resend, second, 2, "and this")

		for count := 3; count <= 6; count++ {
			ask(original, count)
		}
		receipt, err := r.bobPhone.AskAgain(original, 7)
		if err != nil {
			t.Fatal(err)
		}
		r.server.Inbox <- fakerelay.DeliverReceipt(r.bob, receipt)
		synctest.Wait()
		if n := len(r.deliveredTo(r.bob)); n != 8 {
			t.Fatalf("%d deliveries to bob after a sixth retry, want 8", n)
		}

		stranger := original
		stranger.Attrs = slices.Clone(original.Attrs)
		for i, a := range stranger.Attrs {
			if a.Key == "id" {
				stranger.Attrs[i].Value = node.Text("3EB0NEVERSENT")
			}
		}
		receipt, err = r.bobPhone.AskAgain(stranger, 1)
		if err != nil {
			t.Fatal(err)
		}
		r.server.Inbox <- fakerelay.DeliverReceipt(r.bob, receipt)
		synctest.Wait()
		if n := len(r.deliveredTo(r.bob)); n != 8 {
			t.Fatalf("a message we never sent was sent: %d deliveries", n)
		}
	})
}

func TestSendingToANumberWithoutWhatsApp(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		c := r.connect()
		nobody := node.JID{User: "40733333333", Server: node.ServerUser}
		if _, err := c.Send(t.Context(), nobody, &wire.Message{Conversation: new("hello?")}); !errors.Is(err, client.ErrNoTarget) {
			t.Fatalf("Send = %v, want ErrNoTarget", err)
		}
		keyless := node.JID{User: "40744444444", Server: node.ServerUser}
		r.devices.Set(keyless, fakeusync.Device{ID: 0})
		if _, err := c.Send(t.Context(), keyless, &wire.Message{Conversation: new("hello?")}); !errors.Is(err, client.ErrNoTarget) {
			t.Fatalf("Send to a device without keys = %v, want ErrNoTarget", err)
		}
		if got := r.deliveredTo(r.account); len(got) != 0 {
			t.Fatalf("our phone got %d copies of a message that went nowhere", len(got))
		}
	})
}

func (r *rig) answers(id string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, n := range r.sent {
		if got, _ := n.Attr("id").Text(); got != id || n.Tag != "receipt" && n.Tag != "ack" {
			continue
		}
		answer := n.Tag
		for _, key := range []string{"class", "type", "error"} {
			if v, ok := n.Attr(key).Text(); ok {
				answer += " " + key + "=" + v
			}
		}
		out = append(out, answer)
	}
	return out
}

func TestADuplicateIsNeverAskedForAgain(t *testing.T) {
	for _, tt := range []struct {
		name   string
		stored bool
		reply  bool
		want   []string
	}{
		{name: "a first message we kept", stored: true, want: []string{"receipt", "receipt"}},
		{name: "a first message we lost", want: []string{"receipt", "ack class=message type=text error=496"}},
		{name: "a reply we kept", stored: true, reply: true, want: []string{"receipt", "receipt"}},
		{name: "a reply we lost", reply: true, want: []string{"receipt", "ack class=message type=text error=496"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				r := newRig(t)
				c := r.connect()
				self := r.world.Phone.JID
				r.seen = func(node.JID, string) bool { return tt.stored }
				if tt.reply {
					if _, err := c.Send(t.Context(), r.bob, &wire.Message{Conversation: new("hi bob")}); err != nil {
						t.Fatal(err)
					}
					if _, _, err := r.bobPhone.Receive(r.deliveredTo(r.bob)[0]); err != nil {
						t.Fatal(err)
					}
				}
				out, err := r.bobPhone.Send(r.keys, r.devices, r.account, &wire.Message{Conversation: new("twice")})
				if err != nil {
					t.Fatal(err)
				}
				stanza := fakerelay.Deliver(r.bob, "Bob", time.Now(), out)[self]
				r.server.Inbox <- stanza
				<-r.received
				r.server.Inbox <- stanza
				synctest.Wait()
				if got := r.answers(stanza.Attr("id").String()); !slices.Equal(got, tt.want) {
					t.Fatalf("answers %q, want %q", got, tt.want)
				}
				select {
				case m := <-r.received:
					t.Fatalf("the duplicate reached the app: %v", m.Message)
				default:
				}
			})
		})
	}
}

func TestStanzasWeCannotUseAreRefused(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		r.connect()
		self := r.world.Phone.JID
		from := node.Attr{Key: "from", Value: node.Address(r.bob)}
		text := func(k, v string) node.Attr { return node.Attr{Key: k, Value: node.Text(v)} }
		garbled, err := r.bobPhone.SendUnreadable(r.keys, self)
		if err != nil {
			t.Fatal(err)
		}
		garbledID := garbled.Attr("id").String()
		r.server.Inbox <- fakerelay.Deliver(r.bob, "Bob", time.Now(), garbled)[self]
		r.server.Inbox <- node.Node{Tag: "message", Attrs: []node.Attr{from, text("id", "3EB0NOTIME"), text("type", "text")}}
		r.server.Inbox <- node.Node{Tag: "call", Attrs: []node.Attr{from, text("id", "C1"), text("t", "1790000000")}, Children: []node.Node{{Tag: "offer", Attrs: []node.Attr{text("call-id", "X1")}}}}
		r.server.Inbox <- node.Node{Tag: "status", Attrs: []node.Attr{from, text("id", "S1"), text("t", "1790000000")}}
		synctest.Wait()
		for id, want := range map[string][]string{
			garbledID:    {"ack class=message type=text error=491"},
			"3EB0NOTIME": {"ack class=message type=text error=487"},
			"C1":         {"ack class=call"},
			"S1":         {"ack class=status error=415"},
		} {
			if got := r.answers(id); !slices.Equal(got, want) {
				t.Errorf("answers to %s: %q, want %q", id, got, want)
			}
		}
	})
}

func TestACrashOnOneMessageDoesNotStopTheNext(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		r.connect()
		self := r.world.Phone.JID
		r.receive = func(m client.Received) {
			if m.Message.GetConversation() == "boom" {
				panic("the app choked")
			}
		}
		var ids []string
		for _, text := range []string{"boom", "still here"} {
			out, err := r.bobPhone.Send(r.keys, r.devices, r.account, &wire.Message{Conversation: new(text)})
			if err != nil {
				t.Fatal(err)
			}
			ids = append(ids, out.Attr("id").String())
			r.server.Inbox <- fakerelay.Deliver(r.bob, "Bob", time.Now(), out)[self]
		}
		if got := <-r.received; got.Message.GetConversation() != "still here" {
			t.Fatalf("received %v after the crash", got.Message)
		}
		synctest.Wait()
		if got := r.answers(ids[0]); !slices.Equal(got, []string{"ack class=message type=text error=500"}) {
			t.Fatalf("answers to the crashing message: %q", got)
		}
		if got := r.answers(ids[1]); !slices.Equal(got, []string{"receipt"}) {
			t.Fatalf("answers to the next message: %q", got)
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		if len(r.problems) != 1 || !strings.Contains(r.problems[0].Error(), "the app choked") {
			t.Fatalf("problems reported: %v", r.problems)
		}
	})
}

func TestSessionsKeptUnderANumberMoveToThePrivateIDOnRestart(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		bobLID := node.JID{User: "99001", Server: node.ServerLID}
		w := r.world
		w.Script(w.QRPairing(time.Second), w.Login(fakeworld.Success()), w.Serve(r.server), w.Serve(r.server))
		cfg := linkflow.Config{Dial: w.Dial, Dictionary: w.Dictionary, Root: w.Authority.Root(), Version: w.Version, Random: rand.Reader, Now: time.Now, ShowQR: w.ShowQR, Save: func(linkflow.Linked) error { return nil }, KeepAlive: -1}
		linked, err := linkflow.Link(t.Context(), cfg)
		if err != nil {
			t.Fatal(err)
		}
		var saved client.State
		open := func(state client.State, lids map[node.JID]node.JID) *client.Client {
			t.Helper()
			c, err := client.Connect(t.Context(), client.Config{Link: cfg, LIDs: lids, Receive: func(m client.Received) { r.received <- m },
				Persist: func(s client.State) error {
					r.mu.Lock()
					defer r.mu.Unlock()
					saved = s
					return nil
				}}, state)
			if err != nil {
				t.Fatal(err)
			}
			return c
		}
		first := open(client.State{Linked: linked}, nil)
		if _, err := first.Send(t.Context(), r.bob, &wire.Message{Conversation: new("hello bob")}); err != nil {
			t.Fatal(err)
		}
		if _, _, err := r.bobPhone.Receive(r.deliveredTo(r.bob)[0]); err != nil {
			t.Fatal(err)
		}
		_ = first.Close()
		r.mu.Lock()
		before := saved
		r.mu.Unlock()
		if !slices.ContainsFunc(before.Sessions, func(s client.SessionEntry) bool { return s.Device.User == r.bob.User }) {
			t.Fatal("the first run kept no session under bob's number")
		}

		second := open(before, map[node.JID]node.JID{bobLID: r.bob})
		defer second.Close()
		reply, err := r.bobPhone.Send(r.keys, r.devices, r.account, &wire.Message{Conversation: new("from my private id")})
		if err != nil {
			t.Fatal(err)
		}
		r.server.Inbox <- fakerelay.Deliver(bobLID, "Bob", time.Now(), reply)[w.Phone.JID]
		if got := <-r.received; got.Message.GetConversation() != "from my private id" {
			t.Fatalf("received %+v", got)
		}
		synctest.Wait()
		r.mu.Lock()
		defer r.mu.Unlock()
		for _, s := range saved.Sessions {
			if s.Device.User == r.bob.User {
				t.Fatalf("a session is still kept under bob's number after the restart: %v", s.Device)
			}
		}
	})
}

func (r *rig) sentStanza(id string) node.Node {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, n := range r.sent {
		if got, _ := n.Attr("id").Text(); n.Tag == "message" && got == id {
			return n
		}
	}
	r.t.Fatalf("no message %s was sent", id)
	return node.Node{}
}

func (r *rig) gives() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, n := range r.sent {
		if xmlns, _ := n.Attr("xmlns").Text(); n.Tag == "iq" && xmlns == "privacy" {
			tokens, _ := n.Child("tokens")
			for _, token := range tokens.Children {
				out = append(out, token.Attr("jid").String()+" t="+token.Attr("t").String())
			}
		}
	}
	return out
}

func TestPrivacyTokensTravelWithPersonalMessages(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		var start time.Time
		carol := node.JID{User: "40733333333", Server: node.ServerUser}
		carolPhone, err := fakedevice.New(rand.Reader, carol)
		if err != nil {
			t.Fatal(err)
		}
		if err := carolPhone.Upload(r.keys, 3); err != nil {
			t.Fatal(err)
		}
		r.devices.Set(carol, fakeusync.Device{ID: 0})
		r.tokenOf = func(contact node.JID) privacy.Token {
			switch contact {
			case r.bob:
				return privacy.Token{Theirs: []byte{9, 9}, Given: start.Add(-24 * time.Hour)}
			case carol:
				return privacy.Token{Theirs: []byte{8}, Given: start.Add(-40 * 24 * time.Hour)}
			}
			return privacy.Token{}
		}
		c := r.connect()
		start = time.Now()
		send := func(to node.JID, m *wire.Message) node.Node {
			t.Helper()
			id, err := c.Send(t.Context(), to, m)
			if err != nil {
				t.Fatal(err)
			}
			synctest.Wait()
			return r.sentStanza(id)
		}
		token := func(stanza node.Node) []byte {
			last := stanza.Children[len(stanza.Children)-1]
			if last.Tag != "tctoken" {
				return nil
			}
			return last.Bytes
		}
		stamp := func(at time.Time) string { return fmt.Sprint(at.Unix()) }

		if got := token(send(r.bob, &wire.Message{Conversation: new("hello")})); !bytes.Equal(got, []byte{9, 9}) {
			t.Fatalf("tctoken on a message to bob = %v", got)
		}
		send(r.bob, &wire.Message{Conversation: new("again")})
		if got := token(send(carol, &wire.Message{Conversation: new("hi carol")})); got != nil {
			t.Fatalf("an expired token went out: %v", got)
		}
		if got := token(send(r.account, &wire.Message{Conversation: new("note to self")})); got != nil {
			t.Fatalf("a note to self carried a token: %v", got)
		}
		want := []string{"40722222222@s.whatsapp.net t=" + stamp(start), "40733333333@s.whatsapp.net t=" + stamp(start)}
		if got := r.gives(); !slices.Equal(got, want) {
			t.Fatalf("tokens given %q, want %q", got, want)
		}

		time.Sleep(8 * 24 * time.Hour)
		later := time.Now()
		revoke := &wire.Message{ProtocolMessage: &wire.Message_ProtocolMessage{Type: wire.Message_ProtocolMessage_REVOKE.Enum(), Key: &wire.MessageKey{Id: new("3EB0OLD")}}}
		if got := token(send(r.bob, revoke)); !bytes.Equal(got, []byte{9, 9}) {
			t.Fatalf("tctoken on a delete = %v", got)
		}
		if got := r.gives(); len(got) != 2 {
			t.Fatalf("a delete gave our token again: %q", got)
		}
		send(r.bob, &wire.Message{Conversation: new("a week later")})
		if got := r.gives(); len(got) != 3 || got[2] != "40722222222@s.whatsapp.net t="+stamp(later) {
			t.Fatalf("tokens given after a week: %q", got)
		}
		r.mu.Lock()
		saved := slices.Clone(r.saved)
		r.mu.Unlock()
		if len(saved) != 3 || saved[0].Contact != r.bob || !saved[0].Ours.Equal(start) || saved[2].Contact != r.bob || !saved[2].Ours.Equal(later) {
			t.Fatalf("our tokens were not remembered: %+v", saved)
		}

		r.server.Inbox <- node.Node{Tag: "notification", Attrs: []node.Attr{
			{Key: "from", Value: node.Address(carol)}, {Key: "type", Value: node.Text("privacy_token")}, {Key: "id", Value: node.Text("PT1")}, {Key: "t", Value: node.Text(stamp(later))},
		}, Children: []node.Node{{Tag: "tokens", Children: []node.Node{{Tag: "token", Attrs: []node.Attr{
			{Key: "jid", Value: node.Address(r.account)}, {Key: "t", Value: node.Text(stamp(later))}, {Key: "type", Value: node.Text("trusted_contact")},
		}, Bytes: []byte{4, 2}}}}}}
		synctest.Wait()
		r.mu.Lock()
		defer r.mu.Unlock()
		last := r.saved[len(r.saved)-1]
		if last.Contact != carol || !bytes.Equal(last.Theirs, []byte{4, 2}) || !last.Given.Equal(time.Unix(later.Unix(), 0)) {
			t.Fatalf("carol's new token was not kept: %+v", last)
		}
	})
}

func TestANoteToSelfByPrivateIDReachesEachDeviceOnce(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		ourLID := node.JID{User: r.world.Phone.LID.User, Server: node.ServerLID}
		r.devices.Set(ourLID, fakeusync.Device{ID: 0}, fakeusync.Device{ID: r.world.Phone.JID.Device})
		r.keys.Alias(ourLID, r.account)
		c := r.connect()
		id, err := c.Send(t.Context(), ourLID, &wire.Message{Conversation: new("note to self")})
		if err != nil {
			t.Fatal(err)
		}
		stanza := r.sentStanza(id)
		envelopes := 0
		for _, child := range stanza.Children {
			switch child.Tag {
			case "enc":
				envelopes++
			case "participants":
				envelopes += len(child.Children)
			}
		}
		if envelopes != 1 {
			t.Fatalf("the note went out %d times: %s", envelopes, stanza)
		}
	})
}

func TestANewPhoneOrIdentityGetsFreshSessionsAndOurGroupKey(t *testing.T) {
	for _, tt := range []struct {
		name   string
		notice func(r *rig) node.Node
	}{
		{name: "a new identity", notice: func(r *rig) node.Node {
			return node.Node{Tag: "notification", Attrs: []node.Attr{
				{Key: "from", Value: node.Address(r.bob)}, {Key: "type", Value: node.Text("encrypt")}, {Key: "id", Value: node.Text("E1")},
				{Key: "lid", Value: node.Address(node.JID{User: "99001", Server: node.ServerLID})},
			}, Children: []node.Node{{Tag: "identity"}}}
		}},
		{name: "a removed device", notice: func(r *rig) node.Node {
			return node.Node{Tag: "notification", Attrs: []node.Attr{
				{Key: "from", Value: node.Address(r.bob)}, {Key: "type", Value: node.Text("devices")}, {Key: "id", Value: node.Text("D1")},
			}, Children: []node.Node{{Tag: "remove", Children: []node.Node{{Tag: "device", Attrs: []node.Attr{{Key: "jid", Value: node.Address(r.bob)}}}}}}}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				r := newRig(t)
				family := groups.Group{
					JID: node.JID{User: "120363000000000009", Server: node.ServerGroup}, Subject: "Family", Created: time.Unix(1700000000, 0), AddressingMode: "pn",
					Participants: []groups.Participant{{JID: r.account, Admin: true}, {JID: r.bob}},
				}
				r.server.Groups = fakegroups.New(family)
				r.server.Members = func(node.JID) []node.JID { return []node.JID{r.bob, r.account, r.world.Phone.JID} }
				c := r.connect()
				bobHas := func() (kind string, withKey bool) {
					t.Helper()
					toBob := r.deliveredTo(r.bob)
					in, _, err := r.bobPhone.Receive(toBob[len(toBob)-1])
					if err != nil {
						t.Fatal(err)
					}
					for _, enc := range in.Encs {
						if enc.Type != "skmsg" {
							kind, withKey = enc.Type, true
						}
					}
					return kind, withKey
				}
				if _, err := c.Send(t.Context(), r.bob, &wire.Message{Conversation: new("hi")}); err != nil {
					t.Fatal(err)
				}
				bobHas()
				reply, err := r.bobPhone.Send(r.keys, r.devices, r.account, &wire.Message{Conversation: new("hello back")})
				if err != nil {
					t.Fatal(err)
				}
				r.server.Inbox <- fakerelay.Deliver(r.bob, "Bob", time.Now(), reply)[r.world.Phone.JID]
				<-r.received
				for i, text := range []string{"to the group", "again"} {
					if _, err := c.SendGroup(t.Context(), family, &wire.Message{Conversation: new(text)}); err != nil {
						t.Fatal(err)
					}
					if _, withKey := bobHas(); withKey != (i == 0) {
						t.Fatalf("group message %d carried our key: %v", i, withKey)
					}
				}
				if _, err := c.Send(t.Context(), r.bob, &wire.Message{Conversation: new("before")}); err != nil {
					t.Fatal(err)
				}
				if kind, _ := bobHas(); kind != "msg" {
					t.Fatalf("an established session sent %s", kind)
				}

				r.server.Inbox <- tt.notice(r)
				synctest.Wait()
				if _, err := c.Send(t.Context(), r.bob, &wire.Message{Conversation: new("after")}); err != nil {
					t.Fatal(err)
				}
				if kind, _ := bobHas(); kind != "pkmsg" {
					t.Fatalf("after %s the old session was kept (%s)", tt.name, kind)
				}
				if _, err := c.SendGroup(t.Context(), family, &wire.Message{Conversation: new("welcome back")}); err != nil {
					t.Fatal(err)
				}
				if _, withKey := bobHas(); !withKey {
					t.Fatalf("after %s bob did not get our group key again", tt.name)
				}
			})
		})
	}
}

func TestAServerLowOnOurPrekeysGetsMore(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		r.connect()
		self := r.world.Phone.JID
		before := r.keys.Keys(self)
		r.server.Inbox <- node.Node{Tag: "notification", Attrs: []node.Attr{
			{Key: "from", Value: node.Address(node.JID{Server: node.ServerUser})}, {Key: "type", Value: node.Text("encrypt")}, {Key: "id", Value: node.Text("K1")},
		}, Children: []node.Node{{Tag: "count", Attrs: []node.Attr{{Key: "value", Value: node.Text("4")}}}}}
		synctest.Wait()
		if after := r.keys.Keys(self); after != before+prekeys.Batch {
			t.Fatalf("WhatsApp holds %d prekeys after asking for more, had %d", after, before)
		}
	})
}

func TestAConnectionThatAcknowledgesNothingIsReplaced(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		r.server.Drop = func(n node.Node) bool { return n.Tag == "message" }
		c := r.connect()
		for i := range 2 {
			if _, err := c.Send(t.Context(), r.bob, &wire.Message{Conversation: new("anyone there?")}); err == nil {
				t.Fatalf("send %d was acknowledged", i)
			}
			synctest.Wait()
			select {
			case <-c.Done():
				if i == 0 {
					t.Fatal("one missing acknowledgement ended the connection")
				}
			default:
				if i == 1 {
					t.Fatal("the connection stayed up after two unacknowledged messages")
				}
			}
		}
		if !errors.Is(c.Err(), client.ErrClosed) || !strings.Contains(c.Err().Error(), "acknowledged none") {
			t.Fatalf("ended with %v", c.Err())
		}
	})
}

func TestPeerMessagesReachOurPhoneAlone(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		c := r.connect()
		request := message.KeyRequest([][]byte{{1, 2, 3}})
		id, err := c.SendPeer(t.Context(), request)
		if err != nil {
			t.Fatal(err)
		}
		stanza := r.sentStanza(id)
		for _, want := range []string{"category=peer", "push_priority=high"} {
			k, v, _ := strings.Cut(want, "=")
			if got := stanza.Attr(k).String(); got != v {
				t.Fatalf("%s = %q in %s", k, got, stanza)
			}
		}
		if to := stanza.Attr("to").String(); to != r.account.String() {
			t.Fatalf("sent to %s, want our phone %s", to, r.account)
		}
		if meta, ok := stanza.Child("meta"); !ok || meta.Attr("appdata").String() != "default" {
			t.Fatalf("meta = %s", meta)
		}
		if got := r.deliveredTo(r.bob); len(got) != 0 {
			t.Fatalf("a peer message went to bob: %d", len(got))
		}
		toPhone := r.deliveredTo(r.account)
		if len(toPhone) != 1 {
			t.Fatalf("%d deliveries to our phone", len(toPhone))
		}
		_, m, err := r.ourPhone.Receive(toPhone[0])
		if err != nil || m.GetDeviceSentMessage() != nil || m.GetProtocolMessage().GetType() != wire.Message_ProtocolMessage_APP_STATE_SYNC_KEY_REQUEST ||
			!bytes.Equal(m.GetProtocolMessage().GetAppStateSyncKeyRequest().GetKeyIds()[0].GetKeyId(), []byte{1, 2, 3}) {
			t.Fatalf("our phone read %v: %v", m, err)
		}
	})
}

func TestOurPhoneIsAskedForMessagesWeCouldNotRead(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		r.connect()
		companion := r.world.Phone.JID
		unreadable := func(id string) node.Node {
			return node.Node{Tag: "message", Attrs: []node.Attr{
				{Key: "from", Value: node.Address(r.bob)}, {Key: "type", Value: node.Text("text")},
				{Key: "id", Value: node.Text(id)}, {Key: "t", Value: node.Text("1790000000")},
			}, Children: []node.Node{{Tag: "enc", Attrs: []node.Attr{{Key: "v", Value: node.Text("2")}, {Key: "type", Value: node.Text("msg")}}, Bytes: []byte{0x33, 1, 2, 3, 4, 5, 6, 7, 8, 9}}}}
		}
		r.server.Inbox <- unreadable("3EB0LATE")
		synctest.Wait()
		resent, err := r.bobPhone.Send(r.keys, r.devices, r.account, &wire.Message{Conversation: new("sent again")})
		if err != nil {
			t.Fatal(err)
		}
		resent = resent.With("id", node.Text("3EB0LATE"))
		r.server.Inbox <- fakerelay.Deliver(r.bob, "Bob", time.Now(), resent)[companion]
		if got := <-r.received; got.ID != "3EB0LATE" || got.Message.GetConversation() != "sent again" {
			t.Fatalf("received %+v", got)
		}
		r.server.Inbox <- unreadable("3EB0LOST")
		time.Sleep(10 * time.Second)
		synctest.Wait()
		toPhone := r.deliveredTo(r.account)
		if len(toPhone) != 1 {
			t.Fatalf("%d messages to our phone, want one resend request for the message nobody resent", len(toPhone))
		}
		_, request, err := r.ourPhone.Receive(toPhone[0])
		wanted := request.GetProtocolMessage().GetPeerDataOperationRequestMessage().GetPlaceholderMessageResendRequest()
		if err != nil || len(wanted) != 1 || wanted[0].GetMessageKey().GetId() != "3EB0LOST" || wanted[0].GetMessageKey().GetRemoteJid() != r.bob.String() {
			t.Fatalf("our phone got %v: %v", request, err)
		}
		info, err := proto.Marshal(&wire.MessageInfo{
			Key:     &wire.MessageKey{RemoteJid: new(r.bob.String()), FromMe: new(false), Id: new("3EB0LOST")},
			Message: &wire.Message{Conversation: new("the lost one")}, MessageTimestamp: new(uint64(1790000000)), PushName: new("Bob"),
		})
		if err != nil {
			t.Fatal(err)
		}
		answer := &wire.Message{ProtocolMessage: &wire.Message_ProtocolMessage{
			Type: wire.Message_ProtocolMessage_PEER_DATA_OPERATION_REQUEST_RESPONSE_MESSAGE.Enum(),
			PeerDataOperationRequestResponseMessage: &wire.Message_PeerDataOperationRequestResponseMessage{
				PeerDataOperationRequestType: wire.Message_PLACEHOLDER_MESSAGE_RESEND.Enum(),
				PeerDataOperationResult: []*wire.Message_PeerDataOperationRequestResponseMessage_PeerDataOperationResult{{
					PlaceholderMessageResendResponse: &wire.Message_PeerDataOperationRequestResponseMessage_PeerDataOperationResult_PlaceholderMessageResendResponse{WebMessageInfoBytes: info},
				}},
			},
		}}
		out, err := r.ourPhone.SendPeer(r.keys, companion, answer)
		if err != nil {
			t.Fatal(err)
		}
		r.server.Inbox <- fakerelay.DeliverPeer(r.ourPhone.JID, time.Now(), out)
		got := <-r.received
		if got.ID != "3EB0LOST" || got.Chat != r.bob || got.Author != r.bob || got.Message.GetConversation() != "the lost one" || !got.Time.Equal(time.Unix(1790000000, 0)) {
			t.Fatalf("recovered %+v", got)
		}
	})
}
