package messenger_test

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"go.uber.org/goleak"

	"github.com/PeterStoica/chatwire/internal/client"
	"github.com/PeterStoica/chatwire/internal/device"
	"github.com/PeterStoica/chatwire/internal/groups"
	"github.com/PeterStoica/chatwire/internal/linkflow"
	"github.com/PeterStoica/chatwire/internal/media"
	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/messenger"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/pairing"
	"github.com/PeterStoica/chatwire/internal/store"
	"github.com/PeterStoica/chatwire/internal/testkit/fakedevice"
	"github.com/PeterStoica/chatwire/internal/testkit/fakegroups"
	"github.com/PeterStoica/chatwire/internal/testkit/fakekeys"
	"github.com/PeterStoica/chatwire/internal/testkit/fakerelay"
	"github.com/PeterStoica/chatwire/internal/testkit/fakeusync"
	"github.com/PeterStoica/chatwire/internal/testkit/fakeworld"
	"github.com/PeterStoica/chatwire/internal/wire"
	"sync"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

type rig struct {
	world    *fakeworld.World
	server   *fakeworld.Server
	account  node.JID
	bob      node.JID
	bobPhone *fakedevice.Device
	delivers atomic.Int32
	m        *messenger.Messenger
	state    client.State
	store    *store.Store
	cfg      linkflow.Config
}

func newRig(t *testing.T, cdn ...*http.Client) *rig {
	t.Helper()
	w, err := fakeworld.New(61)
	if err != nil {
		t.Fatal(err)
	}
	r := &rig{world: w, account: node.JID{User: w.Phone.JID.User, Server: w.Phone.JID.Server}, bob: node.JID{User: "40722222222", Server: node.ServerUser}}
	keys, devices := fakekeys.New(), fakeusync.New()
	if r.bobPhone, err = fakedevice.New(rand.Reader, r.bob); err != nil {
		t.Fatal(err)
	}
	if err := r.bobPhone.Upload(keys, 5); err != nil {
		t.Fatal(err)
	}
	devices.Set(r.account, fakeusync.Device{ID: w.Phone.JID.Device})
	devices.Set(r.bob, fakeusync.Device{ID: 0})
	r.server = &fakeworld.Server{
		Keys: keys, Devices: devices, PushName: "Me", Inbox: make(chan node.Node, 4),
		Deliver: func(node.JID, node.Node) { r.delivers.Add(1) },
	}
	identity, err := device.New(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	r.state = client.State{Linked: linkflow.Linked{Identity: identity.Store(), Account: pairing.Account{JID: w.Phone.JID}}}
	cfg := linkflow.Config{Dial: w.Dial, Dictionary: w.Dictionary, Root: w.Authority.Root(), Version: w.Version, Random: rand.Reader, Now: time.Now, KeepAlive: -1}
	var httpClient *http.Client
	if len(cdn) > 0 {
		httpClient = cdn[0]
	}
	if r.store, err = store.Open(t.Context(), filepath.Join(t.TempDir(), "messages.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.store.Close() })
	r.cfg = cfg
	r.m = messenger.New(cfg, httpClient, func(client.State) error { return nil }, r.store)
	t.Cleanup(r.m.Close)
	return r
}

func (r *rig) recent(t *testing.T) []store.Message {
	t.Helper()
	messages, err := r.m.Messages(t.Context(), store.Query{Limit: 1000})
	if err != nil {
		t.Fatal(err)
	}
	return messages
}

func (r *rig) send(t *testing.T, text string) {
	t.Helper()
	if _, err := r.m.SendText(t.Context(), r.bob, text); err != nil {
		t.Fatalf("send %q: %v", text, err)
	}
}

func TestNothingToSendBeforeLinking(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		if _, linked := r.m.Self(); linked {
			t.Fatal("Self() reports a link")
		}
		if _, err := r.m.SendText(t.Context(), r.bob, "hi"); !errors.Is(err, messenger.ErrNotLinked) {
			t.Fatalf("SendText() = %v, want %v", err, messenger.ErrNotLinked)
		}
	})
}

func TestReconnectsAfterWhatsAppDropsTheStream(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		w := r.world
		w.Script(func(c *fakeworld.Conn) { c.Send(fakeworld.Failure("503")) }, w.Serve(r.server), w.Serve(r.server))
		r.m.Start(t.Context(), r.state)
		if self, linked := r.m.Self(); !linked || self != r.account {
			t.Fatalf("Self() = %v, %v", self, linked)
		}
		if _, err := r.m.SendText(t.Context(), r.bob, "refused"); !errors.Is(err, linkflow.ErrLoginRejected) {
			t.Fatalf("SendText() while WhatsApp refuses the login = %v, want the refusal", err)
		}
		synctest.Sleep(time.Second)
		r.send(t, "first")
		r.server.Inbox <- node.Node{Tag: "stream:error", Attrs: []node.Attr{{Key: "code", Value: node.Text("503")}}}
		synctest.Wait()
		r.send(t, "after the drop")
		if dials := w.Dials(); dials != 3 {
			t.Fatalf("dialled %d times, want a refused login, a session and a reconnect", dials)
		}
		if delivered := r.delivers.Load(); delivered != 2 {
			t.Fatalf("delivered %d messages", delivered)
		}
	})
}

func TestRemembersWhatArrives(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		r.world.Script(r.world.Serve(r.server))
		r.m.Start(t.Context(), r.state)
		r.send(t, "open the session")
		reply, err := r.bobPhone.Send(r.server.Keys, r.server.Devices, r.account, &wire.Message{Conversation: new("hello back")})
		if err != nil {
			t.Fatal(err)
		}
		r.server.Inbox <- fakerelay.Deliver(r.bob, "Bob", time.Now(), reply)[r.world.Phone.JID]
		synctest.Wait()
		recent := r.recent(t)
		if len(recent) != 2 || recent[0].Message.GetConversation() != "open the session" || !recent[0].FromMe ||
			recent[1].Message.GetConversation() != "hello back" || recent[1].Chat != r.bob || recent[1].FromMe || recent[1].PushName != "Bob" {
			t.Fatalf("recent = %+v", recent)
		}
		names, err := r.m.Names(t.Context())
		if err != nil || names[r.bob].Push != "Bob" {
			t.Fatalf("names = %v, %v", names, err)
		}
	})
}

func TestRetriesBackOffUpToAMinute(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		var attempts []time.Time
		refuse := func(c *fakeworld.Conn) {
			attempts = append(attempts, time.Now())
			c.Send(fakeworld.Failure("503"))
		}
		scripts := make([]fakeworld.Script, 0, 9)
		for range 8 {
			scripts = append(scripts, refuse)
		}
		r.world.Script(append(scripts, r.world.Serve(r.server))...)
		r.m.Start(t.Context(), r.state)
		synctest.Sleep(4 * time.Minute)
		r.send(t, "finally")
		want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 32 * time.Second, time.Minute}
		if len(attempts) != len(want)+1 {
			t.Fatalf("%d refused attempts", len(attempts))
		}
		for i, gap := range want {
			if got := attempts[i+1].Sub(attempts[i]); got < gap*8/10 || got > gap*12/10 {
				t.Fatalf("retry %d after %s, want %s give or take a fifth", i+1, got, gap)
			}
		}
	})
}

func TestMessagesOutliveTheMessenger(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		r.world.Script(r.world.Serve(r.server))
		r.m.Start(t.Context(), r.state)
		r.send(t, "open the session")
		for i := range 250 {
			reply, err := r.bobPhone.Send(r.server.Keys, r.server.Devices, r.account, &wire.Message{Conversation: new(fmt.Sprint(i))})
			if err != nil {
				t.Fatal(err)
			}
			r.server.Inbox <- fakerelay.Deliver(r.bob, "Bob", time.Now(), reply)[r.world.Phone.JID]
		}
		synctest.Wait()
		if err := r.m.Connection().StoreErr; err != nil {
			t.Fatalf("store: %v", err)
		}
		r.m.Close()
		again := messenger.New(r.cfg, nil, func(client.State) error { return nil }, r.store)
		kept, err := again.Messages(t.Context(), store.Query{Chat: r.bob, Limit: 1000})
		if err != nil || len(kept) != 251 || kept[250].Message.GetConversation() != "249" {
			t.Fatalf("kept %d messages, %v", len(kept), err)
		}
	})
}

func TestGroupNamesAreKnownRightAfterConnecting(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		family := groups.Group{JID: node.JID{User: "120363000000000021", Server: node.ServerGroup}, Subject: "Family", Created: time.Unix(1700000000, 0)}
		r.server.Groups = fakegroups.New(family)
		r.world.Script(r.world.Serve(r.server))
		r.m.Start(t.Context(), r.state)
		synctest.Wait()
		chats, err := r.m.Chats(t.Context(), 10)
		if err != nil || len(chats) != 1 || chats[0].JID != family.JID || chats[0].Name != "Family" {
			t.Fatalf("Chats() = %+v, %v", chats, err)
		}
	})
}

func TestMediaMineAndGroupsThroughTheMessenger(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		photo := bytes.Repeat([]byte("jpeg"), 3000)
		mediaKey := bytes.Repeat([]byte{9}, media.KeySize)
		sealed, err := media.Encrypt(mediaKey, media.Image, photo)
		if err != nil {
			t.Fatal(err)
		}
		cdn := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.URL.Path != "/v/photo.enc" {
				http.NotFound(w, req)
				return
			}
			_, _ = w.Write(sealed.File)
		}))
		r := newRig(t, cdn.Client())
		lid := node.JID{User: "88123456789012", Server: node.ServerLID}
		r.state.Linked.Account.LID = lid
		if got := r.m.Connection(); got.Linked || got.Connected || got.Err != nil {
			t.Fatalf("Connection() before linking = %+v", got)
		}
		if r.m.Mine(r.account) {
			t.Fatal("nothing is mine before linking")
		}
		if _, _, err := r.m.Media(t.Context(), "3EB0NONE"); !errors.Is(err, messenger.ErrUnknownMessage) {
			t.Fatalf("Media() of an unknown id = %v", err)
		}
		family := groups.Group{JID: node.JID{User: "120363000000000021", Server: node.ServerGroup}, Subject: "Family",
			Participants: []groups.Participant{{JID: r.account}, {JID: r.bob}}}
		r.server.Groups = fakegroups.New(family)
		r.server.Members = func(node.JID) []node.JID { return []node.JID{r.bob, r.account} }
		r.world.Script(r.world.Serve(r.server))
		r.m.Start(t.Context(), r.state)
		synctest.Wait()
		if got := r.m.Connection(); !got.Linked || !got.Connected || got.Err != nil {
			t.Fatalf("Connection() once connected = %+v", got)
		}
		device := r.account
		device.Device = 7
		for _, tt := range []struct {
			jid  node.JID
			mine bool
		}{{r.account, true}, {device, true}, {lid, true}, {r.bob, false}, {node.JID{User: "1", Server: node.ServerLID}, false}, {node.JID{User: lid.User, Server: node.ServerUser}, false}} {
			if got := r.m.Mine(tt.jid); got != tt.mine {
				t.Errorf("Mine(%v) = %v", tt.jid, got)
			}
		}
		r.send(t, "open the session")
		for _, m := range []*wire.Message{
			{ImageMessage: &wire.Message_ImageMessage{DirectPath: new("/v/photo.enc"), MediaKey: mediaKey, FileSha256: sealed.FileSHA256[:], FileEncSha256: sealed.FileEncSHA256[:], Mimetype: new("image/jpeg")}},
			{Conversation: new("just words")},
		} {
			reply, err := r.bobPhone.Send(r.server.Keys, r.server.Devices, r.account, m)
			if err != nil {
				t.Fatal(err)
			}
			r.server.Inbox <- fakerelay.Deliver(r.bob, "Bob", time.Now(), reply)[r.world.Phone.JID]
		}
		synctest.Wait()
		recent := r.recent(t)[1:]
		if len(recent) != 2 {
			t.Fatalf("recent = %+v", recent)
		}
		ref, data, err := r.m.Media(t.Context(), recent[0].ID)
		if err != nil || ref.Type != media.Image || !bytes.Equal(data, photo) {
			t.Fatalf("Media() = %+v, %d bytes, %v", ref, len(data), err)
		}
		if _, _, err := r.m.Media(t.Context(), recent[1].ID); !errors.Is(err, messenger.ErrNoMedia) {
			t.Fatalf("Media() of a text = %v", err)
		}
		before := r.delivers.Load()
		if id, err := r.m.SendText(t.Context(), family.JID, "hello family"); err != nil || id == "" {
			t.Fatalf("SendText() to a group = %q, %v", id, err)
		}
		stranger := node.JID{User: "120363999999999999", Server: node.ServerGroup}
		if _, err := r.m.SendText(t.Context(), stranger, "let me in"); !errors.Is(err, messenger.ErrNotMember) {
			t.Fatalf("SendText() to a group the user is not in = %v", err)
		}
		if sent, err := r.m.Messages(t.Context(), store.Query{Chat: family.JID, Limit: 5}); err != nil || len(sent) != 1 || !sent[0].FromMe || sent[0].Message.GetConversation() != "hello family" {
			t.Fatalf("the group message was not kept: %+v, %v", sent, err)
		}
		if r.delivers.Load() != before+2 {
			t.Fatalf("the group message went to %d devices, want Bob's phone and our own phone", r.delivers.Load()-before)
		}
	})
}

func TestARefusalFor463HoldsMessagesToNewContacts(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		w := r.world
		w.Script(w.Serve(r.server))
		r.server.Override = func(n node.Node) (node.Node, bool) {
			if n.Tag != "message" {
				return node.Node{}, false
			}
			return node.Node{Tag: "ack", Attrs: []node.Attr{{Key: "class", Value: node.Text("message")}, {Key: "id", Value: n.Attr("id")}, {Key: "error", Value: node.Text("463")}}}, true
		}
		r.m.Start(t.Context(), r.state)
		var rejected client.Rejection
		if _, err := r.m.SendText(t.Context(), r.bob, "hello"); !errors.As(err, &rejected) || rejected.Code != client.CodeRestricted || !errors.Is(err, client.ErrRejected) {
			t.Fatalf("SendText() = %v, want a 463 rejection", err)
		}
		stranger := node.JID{User: "40733333333", Server: node.ServerUser}
		if _, err := r.m.SendText(t.Context(), stranger, "hi"); !errors.Is(err, messenger.ErrRestricted) {
			t.Fatalf("SendText() to a new contact after a 463 = %v, want it held", err)
		}
	})
}

func TestGroupSendsReuseWhatTheyKnowUntilTheGroupChanges(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		family := groups.Group{JID: node.JID{User: "120363000000000021", Server: node.ServerGroup}, Subject: "Family",
			Participants: []groups.Participant{{JID: r.account}, {JID: r.bob}}}
		known := fakegroups.New(family)
		r.server.Groups = known
		r.server.Members = func(node.JID) []node.JID { return []node.JID{r.bob, r.account} }
		r.world.Script(r.world.Serve(r.server))
		r.m.Start(t.Context(), r.state)
		synctest.Wait()
		for _, text := range []string{"one", "two", "three"} {
			if _, err := r.m.SendText(t.Context(), family.JID, text); err != nil {
				t.Fatal(err)
			}
		}
		if lists, lookups := known.Queries(); lists != 1 || lookups != 0 {
			t.Fatalf("three sends cost %d group lists and %d lookups, want the one list made on connecting", lists, lookups)
		}
		r.server.Inbox <- node.Node{Tag: "notification", Attrs: []node.Attr{
			{Key: "from", Value: node.Address(family.JID)}, {Key: "type", Value: node.Text("w:gp2")}, {Key: "id", Value: node.Text("G1")},
		}, Children: []node.Node{{Tag: "subject", Attrs: []node.Attr{{Key: "subject", Value: node.Text("Family!")}}}}}
		synctest.Wait()
		if _, err := r.m.SendText(t.Context(), family.JID, "four"); err != nil {
			t.Fatal(err)
		}
		if lists, lookups := known.Queries(); lists != 1 || lookups != 1 {
			t.Fatalf("after the group changed: %d lists and %d lookups, want one fresh lookup", lists, lookups)
		}
		time.Sleep(6 * time.Minute)
		if _, err := r.m.SendText(t.Context(), family.JID, "five"); err != nil {
			t.Fatal(err)
		}
		if _, lookups := known.Queries(); lookups != 2 {
			t.Fatalf("an old copy was used: %d lookups", lookups)
		}
	})
}

func TestSentMessagesCarryASecretForLaterEdits(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRig(t)
		var toBob []node.Node
		var mu sync.Mutex
		r.server.Deliver = func(device node.JID, stanza node.Node) {
			mu.Lock()
			defer mu.Unlock()
			if device == r.bob {
				toBob = append(toBob, stanza)
			}
		}
		r.world.Script(r.world.Serve(r.server))
		r.m.Start(t.Context(), r.state)
		synctest.Wait()
		r.send(t, "hello")
		mu.Lock()
		stanza := toBob[0]
		mu.Unlock()
		_, m, err := r.bobPhone.Receive(stanza)
		if err != nil || m.GetConversation() != "hello" || len(m.GetMessageContextInfo().GetMessageSecret()) != message.SecretSize {
			t.Fatalf("bob got %v: %v", m, err)
		}
		stored := r.recent(t)
		if len(stored) != 1 || !bytes.Equal(stored[0].Message.GetMessageContextInfo().GetMessageSecret(), m.GetMessageContextInfo().GetMessageSecret()) {
			t.Fatalf("the secret was not kept with our copy: %+v", stored)
		}
	})
}
