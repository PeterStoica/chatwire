package prekeys_test

import (
	"crypto/rand"
	"errors"
	"maps"
	"testing"

	"github.com/PeterStoica/chatwire/internal/curve"
	"github.com/PeterStoica/chatwire/internal/device"
	"github.com/PeterStoica/chatwire/internal/fakekeys"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/prekeys"
	"github.com/PeterStoica/chatwire/internal/signon"
)

var self = node.JID{User: "40700000000", Device: 17, Server: node.ServerUser}

type uploaded struct {
	server       *fakekeys.Server
	registration signon.Registration
	keys         map[uint32]curve.PublicKey
}

func upload(t *testing.T, count int) uploaded {
	t.Helper()
	identity, err := device.New(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	u := uploaded{server: fakekeys.New(), registration: identity.Registration(device.Props()), keys: map[uint32]curve.PublicKey{}}
	if count == 0 {
		return u
	}
	generated, _, err := prekeys.Generate(rand.Reader, 1, count)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range generated {
		u.keys[k.ID] = k.Key.Public()
	}
	if err := prekeys.Result(u.server.Handle(self, prekeys.Upload(u.registration, generated))); err != nil {
		t.Fatal(err)
	}
	return u
}

func (u uploaded) lookup(id uint32) (curve.PublicKey, bool) {
	key, ok := u.keys[id]
	return key, ok
}

func (u uploaded) digest(t *testing.T) prekeys.Digest {
	t.Helper()
	reply := u.server.Handle(self, prekeys.DigestRequest())
	if prekeys.NeedsUpload(reply) {
		t.Fatal("the server has no keys after an upload")
	}
	d, err := prekeys.ParseDigest(reply)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestDigestAfterUploadVerifies(t *testing.T) {
	u := upload(t, prekeys.Batch)
	d := u.digest(t)
	if len(d.KeyIDs) != prekeys.Batch || d.KeyIDs[0] != 1 || d.KeyIDs[prekeys.Batch-1] != prekeys.Batch {
		t.Fatalf("digest lists %d keys", len(d.KeyIDs))
	}
	if d.RegistrationID != u.registration.RegistrationID || d.Identity != u.registration.Identity || d.SignedPreKey != u.registration.SignedPreKey {
		t.Fatalf("digest = %+v", d)
	}
	if err := d.Verify(u.registration, u.lookup); err != nil {
		t.Fatal(err)
	}
}

func TestNoKeysOnTheServerMeansUpload(t *testing.T) {
	u := upload(t, 0)
	if reply := u.server.Handle(self, prekeys.DigestRequest()); !prekeys.NeedsUpload(reply) {
		t.Fatalf("NeedsUpload(%s) = false", reply)
	}
	u = upload(t, 3)
	if reply := u.server.Handle(self, prekeys.DigestRequest()); prekeys.NeedsUpload(reply) {
		t.Fatal("NeedsUpload after uploading")
	}
}

func TestDigestMismatches(t *testing.T) {
	u := upload(t, 4)
	for _, tt := range []struct {
		name   string
		mutate func(*prekeys.Digest, *signon.Registration, map[uint32]curve.PublicKey)
	}{
		{"hash", func(d *prekeys.Digest, _ *signon.Registration, _ map[uint32]curve.PublicKey) { d.Hash[19] ^= 1 }},
		{"registration id", func(_ *prekeys.Digest, r *signon.Registration, _ map[uint32]curve.PublicKey) { r.RegistrationID++ }},
		{"identity", func(_ *prekeys.Digest, r *signon.Registration, _ map[uint32]curve.PublicKey) { r.Identity[0] ^= 1 }},
		{"signed prekey", func(_ *prekeys.Digest, r *signon.Registration, _ map[uint32]curve.PublicKey) {
			r.SignedPreKey.Key[0] ^= 1
		}},
		{"signature", func(_ *prekeys.Digest, r *signon.Registration, _ map[uint32]curve.PublicKey) {
			r.SignedPreKey.Signature[63] ^= 1
		}},
		{"prekey value", func(_ *prekeys.Digest, _ *signon.Registration, keys map[uint32]curve.PublicKey) {
			keys[3] = curve.PublicKey{}
		}},
		{"missing prekey", func(_ *prekeys.Digest, _ *signon.Registration, keys map[uint32]curve.PublicKey) { delete(keys, 4) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d, registration := u.digest(t), u.registration
			keys := maps.Clone(u.keys)
			tt.mutate(&d, &registration, keys)
			lookup := func(id uint32) (curve.PublicKey, bool) {
				key, ok := keys[id]
				return key, ok
			}
			if err := d.Verify(registration, lookup); !errors.Is(err, prekeys.ErrMismatch) {
				t.Fatalf("Verify() = %v, want %v", err, prekeys.ErrMismatch)
			}
		})
	}
}

func TestMalformedDigests(t *testing.T) {
	good := upload(t, 2).server.Handle(self, prekeys.DigestRequest())
	digest, _ := good.Child("digest")
	without := func(tag string) node.Node {
		reply := good
		trimmed := digest
		trimmed.Children = nil
		for _, child := range digest.Children {
			if child.Tag != tag {
				trimmed.Children = append(trimmed.Children, child)
			}
		}
		reply.Children = []node.Node{trimmed}
		return reply
	}
	shortID := without("list")
	listed, _ := digest.Child("list")
	listed.Children = []node.Node{{Tag: "id", Bytes: []byte{1, 2}}}
	shortID.Children[0].Children = append(shortID.Children[0].Children, listed)
	for _, tt := range []struct {
		name  string
		reply node.Node
	}{
		{"no digest", node.Node{Tag: "iq"}},
		{"no hash", without("hash")},
		{"no registration", without("registration")},
		{"no type", without("type")},
		{"no identity", without("identity")},
		{"no signed prekey", without("skey")},
		{"two-byte key id", shortID},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := prekeys.ParseDigest(tt.reply); !errors.Is(err, prekeys.ErrDigest) {
				t.Fatalf("ParseDigest() = %v, want %v", err, prekeys.ErrDigest)
			}
		})
	}
	if d, err := prekeys.ParseDigest(without("list")); err != nil || len(d.KeyIDs) != 0 {
		t.Fatalf("a digest without keys = %+v, %v", d, err)
	}
}

func TestInvalidUploadsAreRefusedByTheKeyServer(t *testing.T) {
	u := upload(t, 0)
	keys, _, err := prekeys.Generate(rand.Reader, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := prekeys.Result(u.server.Handle(self, prekeys.Upload(u.registration, nil))); !errors.Is(err, prekeys.ErrRejected) {
		t.Fatalf("empty upload: %v", err)
	}
	if err := prekeys.Result(u.server.Handle(self, prekeys.Upload(u.registration, keys))); err != nil || u.server.Keys(self) != 1 {
		t.Fatalf("upload: %v, server holds %d keys", err, u.server.Keys(self))
	}
}
