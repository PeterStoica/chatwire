package prekeys_test

import (
	"crypto/rand"
	"errors"
	"slices"
	"testing"

	"github.com/PeterStoica/chatwire/internal/curve"
	"github.com/PeterStoica/chatwire/internal/device"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/prekeys"
	"github.com/PeterStoica/chatwire/internal/signal"
	"github.com/PeterStoica/chatwire/internal/testkit/fakekeys"
)

type companion struct {
	jid      node.JID
	identity device.Identity
	oneTime  map[uint32]curve.KeyPair
}

func newCompanion(t *testing.T, jid node.JID) *companion {
	t.Helper()
	identity, err := device.New(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return &companion{jid: jid, identity: identity, oneTime: map[uint32]curve.KeyPair{}}
}

func (c *companion) upload(t *testing.T, server *fakekeys.Server, count int) {
	t.Helper()
	keys, _, err := prekeys.Generate(rand.Reader, 1, count)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range keys {
		c.oneTime[k.ID] = k.Key
	}
	if err := prekeys.Result(server.Handle(c.jid, prekeys.Upload(c.identity.Registration(device.Props()), keys))); err != nil {
		t.Fatal(err)
	}
}

func (c *companion) PreKey(id uint32) (curve.KeyPair, bool) {
	key, ok := c.oneTime[id]
	return key, ok
}

func (c *companion) SignedPreKey(id uint32) (curve.KeyPair, bool) {
	return c.identity.SignedPreKey(), id == c.identity.Registration(device.Props()).SignedPreKey.ID
}

func (c *companion) fetch(t *testing.T, server *fakekeys.Server, devices ...node.JID) ([]prekeys.Bundle, map[node.JID]error) {
	t.Helper()
	bundles, failed, err := prekeys.ParseBundles(server.Handle(c.jid, prekeys.FetchRequest(devices)))
	if err != nil {
		t.Fatal(err)
	}
	return bundles, failed
}

func TestFetchedBundlesStartSessionsTheOwnerCanRead(t *testing.T) {
	alice := newCompanion(t, node.JID{User: "40711111111", Device: 3, Server: node.ServerUser})
	bob := newCompanion(t, node.JID{User: "40722222222", Server: node.ServerUser})
	server := fakekeys.New()
	server.Register(alice.jid, []byte("alice's signed device identity"))
	alice.upload(t, server, 2)
	stranger := node.JID{User: "40733333333", Server: node.ServerUser}
	for round, wantPreKey := range []uint32{1, 2, 0} {
		bundles, failed := bob.fetch(t, server, alice.jid, stranger)
		if len(bundles) != 1 || bundles[0].Device != alice.jid || !errors.Is(failed[stranger], prekeys.ErrUnfetched) {
			t.Fatalf("round %d: bundles %+v, failed %v", round, bundles, failed)
		}
		b := bundles[0]
		if string(b.DeviceIdentity) != "alice's signed device identity" || b.Keys.Identity != alice.identity.Signal().Identity.Public() {
			t.Fatalf("round %d: bundle %+v", round, b)
		}
		if got := b.Keys.PreKey; wantPreKey == 0 && got != nil || wantPreKey != 0 && (got == nil || got.ID != wantPreKey) {
			t.Fatalf("round %d: one-time prekey %+v, want id %d", round, got, wantPreKey)
		}
		session, err := signal.Initiate(rand.Reader, bob.identity.Signal(), nil, b.Keys)
		if err != nil {
			t.Fatal(err)
		}
		sent, err := session.Encrypt([]byte("hello alice"))
		if err != nil {
			t.Fatal(err)
		}
		_, plaintext, used, err := signal.Respond(rand.Reader, alice.identity.Signal(), alice, nil, sent.Bytes)
		if err != nil || string(plaintext) != "hello alice" {
			t.Fatalf("round %d: alice read %q, %v", round, plaintext, err)
		}
		if wantPreKey == 0 && used != nil || wantPreKey != 0 && (used == nil || *used != wantPreKey) {
			t.Fatalf("round %d: consumed prekey %v, want %d", round, used, wantPreKey)
		}
	}
}

func TestMalformedBundlesAreRefused(t *testing.T) {
	alice := newCompanion(t, node.JID{User: "40711111111", Device: 3, Server: node.ServerUser})
	server := fakekeys.New()
	alice.upload(t, server, 1)
	good := server.Handle(alice.jid, prekeys.FetchRequest([]node.JID{alice.jid}))
	edit := func(change func(user *node.Node)) node.Node {
		reply := good
		list, _ := reply.Child("list")
		user := list.Children[0]
		user.Children = append([]node.Node(nil), user.Children...)
		change(&user)
		list.Children = []node.Node{user}
		reply.Children = []node.Node{list}
		return reply
	}
	replace := func(tag string, replacement node.Node) func(*node.Node) {
		return func(user *node.Node) {
			for i, child := range user.Children {
				if child.Tag == tag {
					user.Children[i] = replacement
				}
			}
		}
	}
	for _, tt := range []struct {
		name  string
		reply node.Node
	}{
		{"not a result", node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "type", Value: node.Text("error")}}}},
		{"no list", node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "type", Value: node.Text("result")}}}},
		{"entry without jid", edit(func(user *node.Node) { user.Attrs = nil })},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := prekeys.ParseBundles(tt.reply); !errors.Is(err, prekeys.ErrBundle) {
				t.Fatalf("ParseBundles() = %v, want %v", err, prekeys.ErrBundle)
			}
		})
	}
	for _, tt := range []struct {
		name  string
		reply node.Node
	}{
		{"short registration", edit(replace("registration", node.Node{Tag: "registration", Bytes: []byte{1, 2, 3}}))},
		{"wrong key type", edit(replace("type", node.Node{Tag: "type", Bytes: []byte{6}}))},
		{"short identity", edit(replace("identity", node.Node{Tag: "identity", Bytes: make([]byte, 31)}))},
		{"one-time prekey with short id", edit(replace("key", node.Node{Tag: "key", Children: []node.Node{{Tag: "id", Bytes: []byte{1}}, {Tag: "value", Bytes: make([]byte, 32)}}}))},
		{"signed prekey without signature", edit(replace("skey", node.Node{Tag: "skey", Children: []node.Node{{Tag: "id", Bytes: []byte{0, 0, 1}}, {Tag: "value", Bytes: make([]byte, 32)}}}))},
	} {
		t.Run(tt.name+" skips only that device", func(t *testing.T) {
			list, _ := tt.reply.Child("list")
			goodList, _ := good.Child("list")
			both := tt.reply
			both.Children = []node.Node{{Tag: "list", Children: append(slices.Clone(list.Children), goodList.Children[0].With("jid", node.Address(node.JID{User: "40799999999", Device: 1, Server: node.ServerUser})))}}
			bundles, failed, err := prekeys.ParseBundles(both)
			if err != nil || len(bundles) != 1 || bundles[0].Device.User != "40799999999" || !errors.Is(failed[alice.jid], prekeys.ErrBundle) {
				t.Fatalf("ParseBundles() = %d bundles, failed %v, %v", len(bundles), failed, err)
			}
		})
	}
	withoutType := edit(func(user *node.Node) {
		kept := user.Children[:0]
		for _, child := range user.Children {
			if child.Tag != "type" {
				kept = append(kept, child)
			}
		}
		user.Children = kept
	})
	if bundles, _, err := prekeys.ParseBundles(withoutType); err != nil || len(bundles) != 1 {
		t.Fatalf("a bundle without the optional key type: %d bundles, %v", len(bundles), err)
	}
}
