package device_test

import (
	"bytes"
	"math/rand/v2"
	"testing"

	"github.com/PeterStoica/chatwire/internal/curve"
	"github.com/PeterStoica/chatwire/internal/device"
)

func TestNewIsDeterministicAndConsumesTheDocumentedOrder(t *testing.T) {
	identity, err := device.New(rand.NewChaCha8([32]byte{4}))
	if err != nil {
		t.Fatal(err)
	}
	random := rand.NewChaCha8([32]byte{4})
	noise, identityKey, preKey := keyPair(t, random), keyPair(t, random), keyPair(t, random)
	if identity.NoiseKey().Public() != noise.Public() {
		t.Fatal("noise key is not the first key drawn")
	}
	registration := identity.Registration(device.Props())
	if registration.Identity != identityKey.Public() || registration.SignedPreKey.Key != preKey.Public() {
		t.Fatal("identity and signed prekey are not the second and third keys drawn")
	}
	if registration.SignedPreKey.ID != 1 {
		t.Fatalf("signed prekey id = %d, want 1", registration.SignedPreKey.ID)
	}
	typed := append([]byte{curve.KeyType}, registration.SignedPreKey.Key[:]...)
	if err := curve.Verify(registration.Identity, typed, registration.SignedPreKey.Signature); err != nil {
		t.Fatalf("prekey signature does not verify under the identity key: %v", err)
	}
	if registration.Device != device.Props() {
		t.Fatalf("device props = %+v", registration.Device)
	}
}

func TestRegistrationIDRange(t *testing.T) {
	for _, tt := range []struct {
		raw  []byte
		want uint32
	}{
		{raw: []byte{0x00, 0x00}, want: 1},
		{raw: []byte{0x3f, 0xfb}, want: 16380},
		{raw: []byte{0x3f, 0xfc}, want: 1},
		{raw: []byte{0xff, 0xff}, want: 16},
	} {
		random := &prefixed{keys: bytes.Repeat([]byte{7}, 3*curve.KeySize+64), tail: tt.raw}
		identity, err := device.New(random)
		if err != nil {
			t.Fatal(err)
		}
		if got := identity.Registration(device.Props()).RegistrationID; got != tt.want {
			t.Fatalf("registration id from %x = %d, want %d", tt.raw, got, tt.want)
		}
	}
}

func TestNewFailsWhenRandomRunsOut(t *testing.T) {
	for _, size := range []int{0, curve.KeySize, 3 * curve.KeySize, 3*curve.KeySize + 64, 3*curve.KeySize + 65} {
		if _, err := device.New(bytes.NewReader(make([]byte, size))); err == nil {
			t.Fatalf("New succeeded with %d random bytes", size)
		}
	}
}

func TestProps(t *testing.T) {
	props := device.Props()
	if props.OS != "Chatwire" || props.Version.String() != "0.1.0" {
		t.Fatalf("Props() = %+v", props)
	}
}

type prefixed struct {
	keys []byte
	tail []byte
}

func (p *prefixed) Read(b []byte) (int, error) {
	if len(p.keys) > 0 {
		n := copy(b, p.keys)
		p.keys = p.keys[n:]
		return n, nil
	}
	return copy(b, p.tail), nil
}

func keyPair(t *testing.T, random *rand.ChaCha8) curve.KeyPair {
	t.Helper()
	kp, err := curve.NewKeyPair(random)
	if err != nil {
		t.Fatal(err)
	}
	return kp
}
