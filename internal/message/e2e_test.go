package message_test

import (
	"crypto/rand"
	"testing"
	"time"

	"github.com/PeterStoica/chatwire/internal/curve"
	"github.com/PeterStoica/chatwire/internal/device"
	"github.com/PeterStoica/chatwire/internal/fakekeys"
	"github.com/PeterStoica/chatwire/internal/fakerelay"
	"github.com/PeterStoica/chatwire/internal/fakeusync"
	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/prekeys"
	"github.com/PeterStoica/chatwire/internal/signal"
	"github.com/PeterStoica/chatwire/internal/usync"
	"github.com/PeterStoica/chatwire/internal/wire"
)

type world struct {
	t       *testing.T
	keys    *fakekeys.Server
	devices *fakeusync.Server
	all     map[node.JID]*phone
}

type phone struct {
	jid      node.JID
	identity device.Identity
	oneTime  map[uint32]curve.KeyPair
	sessions map[node.JID]*signal.Session
}

func (p *phone) PreKey(id uint32) (curve.KeyPair, bool) {
	key, ok := p.oneTime[id]
	return key, ok
}

func (p *phone) SignedPreKey(id uint32) (curve.KeyPair, bool) {
	return p.identity.SignedPreKey(), id == p.identity.Registration(device.Props()).SignedPreKey.ID
}

func newWorld(t *testing.T) *world {
	t.Helper()
	return &world{t: t, keys: fakekeys.New(), devices: fakeusync.New(), all: map[node.JID]*phone{}}
}

func (w *world) account(user node.JID, ids ...uint8) {
	w.t.Helper()
	listed := make([]fakeusync.Device, len(ids))
	for i, id := range ids {
		identity, err := device.New(rand.Reader)
		if err != nil {
			w.t.Fatal(err)
		}
		p := &phone{jid: node.JID{User: user.User, Device: id, Server: user.Server}, identity: identity, oneTime: map[uint32]curve.KeyPair{}, sessions: map[node.JID]*signal.Session{}}
		keys, _, err := prekeys.Generate(rand.Reader, 1, 5)
		if err != nil {
			w.t.Fatal(err)
		}
		for _, k := range keys {
			p.oneTime[k.ID] = k.Key
		}
		if err := prekeys.Result(w.keys.Handle(p.jid, prekeys.Upload(identity.Registration(device.Props()), keys))); err != nil {
			w.t.Fatal(err)
		}
		w.all[p.jid] = p
		listed[i] = fakeusync.Device{ID: id}
	}
	w.devices.Set(user, listed...)
}

func (w *world) send(from *phone, to node.JID, m *wire.Message) map[node.JID]node.Node {
	w.t.Helper()
	self := node.JID{User: from.jid.User, Server: from.jid.Server}
	users, err := usync.ParseDevices(w.devices.Handle(usync.DevicesRequest("1.2-3", usync.ContextMessage, []node.JID{to, self})))
	if err != nil {
		w.t.Fatal(err)
	}
	var targets, missing []node.JID
	for _, u := range users {
		for _, d := range u.Devices {
			if d.JID == from.jid {
				continue
			}
			targets = append(targets, d.JID)
			if from.sessions[d.JID] == nil {
				missing = append(missing, d.JID)
			}
		}
	}
	if len(missing) > 0 {
		bundles, failed, err := prekeys.ParseBundles(w.keys.Handle(from.jid, prekeys.FetchRequest(missing)))
		if err != nil || len(failed) > 0 {
			w.t.Fatalf("bundles: %v %v", failed, err)
		}
		for _, b := range bundles {
			session, err := signal.Initiate(rand.Reader, from.identity.Signal(), nil, b.Keys)
			if err != nil {
				w.t.Fatal(err)
			}
			from.sessions[b.Device] = session
		}
	}
	parts := make([]message.Part, 0, len(targets))
	for _, device := range targets {
		payload := m
		if device.User == self.User {
			payload = message.SentByUs(to, m)
		}
		padded, err := message.Encode(rand.Reader, payload)
		if err != nil {
			w.t.Fatal(err)
		}
		ciphertext, err := from.sessions[device].Encrypt(padded)
		if err != nil {
			w.t.Fatal(err)
		}
		parts = append(parts, message.Part{Device: device, Ciphertext: ciphertext})
	}
	id, err := message.NewID(time.Now(), self, rand.Reader)
	if err != nil {
		w.t.Fatal(err)
	}
	stanza := message.Outgoing(id, to, m, parts, []byte("signed device identity"))
	return fakerelay.Deliver(from.jid, "sender", time.Now(), stanza)
}

func (w *world) receive(p *phone, stanza node.Node) (message.Incoming, *wire.Message) {
	w.t.Helper()
	mine := func(j node.JID) bool { return j.User == p.jid.User && j.Server == p.jid.Server }
	in, err := message.ParseIncoming(mine, stanza)
	if err != nil || len(in.Encs) != 1 {
		w.t.Fatalf("%v parses %s: %v", p.jid, stanza, err)
	}
	enc := in.Encs[0]
	var padded []byte
	if enc.Type == "pkmsg" {
		if len(in.DeviceIdentity) == 0 {
			w.t.Fatalf("%v: a prekey message without the sender's device identity", p.jid)
		}
		session, plaintext, _, err := signal.Respond(rand.Reader, p.identity.Signal(), p, p.sessions[in.Author], enc.Ciphertext)
		if err != nil {
			w.t.Fatalf("%v decrypts: %v", p.jid, err)
		}
		p.sessions[in.Author], padded = session, plaintext
	} else {
		plaintext, err := p.sessions[in.Author].Decrypt(rand.Reader, enc.Ciphertext)
		if err != nil {
			w.t.Fatalf("%v decrypts: %v", p.jid, err)
		}
		padded = plaintext
	}
	m, err := message.Decode(padded)
	if err != nil {
		w.t.Fatal(err)
	}
	return in, m
}

func TestTextMessagesRoundTripBetweenAccounts(t *testing.T) {
	w := newWorld(t)
	alice := node.JID{User: "40711111111", Server: node.ServerUser}
	bob := node.JID{User: "40722222222", Server: node.ServerUser}
	w.account(alice, 0, 17)
	w.account(bob, 0, 3)
	us := w.all[node.JID{User: alice.User, Device: 17, Server: alice.Server}]

	deliveries := w.send(us, bob, &wire.Message{Conversation: new("hello bob")})
	if len(deliveries) != 3 {
		t.Fatalf("delivered to %d devices, want bob's two and alice's phone", len(deliveries))
	}
	for device, stanza := range deliveries {
		in, m := w.receive(w.all[device], stanza)
		if in.Author != us.jid {
			t.Fatalf("%v: author %v", device, in.Author)
		}
		switch device.User {
		case bob.User:
			if in.Chat != alice || m.GetConversation() != "hello bob" {
				t.Fatalf("%v got %v in chat %v", device, m, in.Chat)
			}
		case alice.User:
			sent := m.GetDeviceSentMessage()
			if in.Chat != bob || sent.GetDestinationJid() != bob.String() || sent.GetMessage().GetConversation() != "hello bob" {
				t.Fatalf("alice's phone got %v in chat %v", m, in.Chat)
			}
		}
	}

	bobPhone := w.all[node.JID{User: bob.User, Server: bob.Server}]
	replies := w.send(bobPhone, alice, &wire.Message{Conversation: new("hi alice")})
	ours, ok := replies[us.jid]
	if !ok || len(replies) != 3 {
		t.Fatalf("reply delivered to %d devices", len(replies))
	}
	in, m := w.receive(us, ours)
	if in.Encs[0].Type != "msg" || in.Chat != bob || m.GetConversation() != "hi alice" {
		t.Fatalf("our device got %v (%s) in chat %v", m, in.Encs[0].Type, in.Chat)
	}
	for device, stanza := range replies {
		if device != us.jid {
			w.receive(w.all[device], stanza)
		}
	}
}
