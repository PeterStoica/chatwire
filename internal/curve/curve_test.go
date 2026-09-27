package curve_test

import (
	"bytes"
	"crypto/rand"
	"errors"
	"testing"

	"github.com/PeterStoica/chatwire/internal/curve"
)

func TestVerify(t *testing.T) {
	t.Parallel()
	signer, other := newKeyPair(t), newKeyPair(t)
	message := []byte("signed prekey")
	signature, err := signer.Sign(rand.Reader, message)
	if err != nil {
		t.Fatal(err)
	}
	flipped := signature
	flipped[curve.SignatureSize-1] ^= 0x80
	tests := []struct {
		name      string
		key       curve.PublicKey
		message   []byte
		signature curve.Signature
		want      error
	}{
		{name: "valid", key: signer.Public(), message: message, signature: signature},
		{name: "tampered message", key: signer.Public(), message: []byte("signed prekeY"), signature: signature, want: curve.ErrInvalidSignature},
		{name: "other key", key: other.Public(), message: message, signature: signature, want: curve.ErrInvalidSignature},
		{name: "sign bit flipped", key: signer.Public(), message: message, signature: flipped, want: curve.ErrInvalidSignature},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := curve.Verify(tt.key, tt.message, tt.signature); !errors.Is(err, tt.want) {
				t.Fatalf("Verify() = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestSignKeyCoversTypedKey(t *testing.T) {
	signer, key := newKeyPair(t), newKeyPair(t).Public()
	signature, err := signer.SignKey(rand.Reader, key)
	if err != nil {
		t.Fatal(err)
	}
	typed := append([]byte{curve.KeyType}, key[:]...)
	if err := curve.Verify(signer.Public(), typed, signature); err != nil {
		t.Fatalf("SignKey signature does not verify over 0x05||key: %v", err)
	}
	if err := curve.Verify(signer.Public(), key[:], signature); err == nil {
		t.Fatal("SignKey signature verifies over the bare key")
	}
}

func TestSharedSecretAgrees(t *testing.T) {
	alice, bob := newKeyPair(t), newKeyPair(t)
	fromAlice, err := alice.SharedSecret(bob.Public())
	if err != nil {
		t.Fatal(err)
	}
	fromBob, err := bob.SharedSecret(alice.Public())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(fromAlice, fromBob) {
		t.Fatal("shared secrets differ")
	}
}

func TestParsePublicKeyRequires32Bytes(t *testing.T) {
	for _, size := range []int{0, 31, 33} {
		if _, err := curve.ParsePublicKey(make([]byte, size)); err == nil {
			t.Fatalf("ParsePublicKey(%d bytes) succeeded", size)
		}
	}
	key, err := curve.ParsePublicKey(bytes.Repeat([]byte{7}, curve.KeySize))
	if err != nil || key[0] != 7 || key[curve.KeySize-1] != 7 {
		t.Fatalf("ParsePublicKey(32 bytes) = %x, %v", key, err)
	}
}

func TestTypedPublicKeys(t *testing.T) {
	key := newKeyPair(t).Public()
	typed := key.Typed()
	if len(typed) != curve.KeySize+1 || typed[0] != 0x05 || !bytes.Equal(typed[1:], key[:]) {
		t.Fatalf("Typed() = %x", typed)
	}
	if parsed, err := curve.ParseTypedPublicKey(typed); err != nil || parsed != key {
		t.Fatalf("ParseTypedPublicKey(Typed()) = %x, %v", parsed, err)
	}
	for _, tt := range []struct {
		name string
		raw  []byte
	}{
		{"empty", nil},
		{"untyped", key[:]},
		{"one byte long", append(bytes.Clone(typed), 0)},
		{"other type", append([]byte{0x06}, key[:]...)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := curve.ParseTypedPublicKey(tt.raw); !errors.Is(err, curve.ErrTypedKey) {
				t.Fatalf("ParseTypedPublicKey(%x) error = %v, want %v", tt.raw, err, curve.ErrTypedKey)
			}
		})
	}
}

func TestNewKeyPairFailsOnShortRandom(t *testing.T) {
	if _, err := curve.NewKeyPair(bytes.NewReader(make([]byte, curve.KeySize-1))); err == nil {
		t.Fatal("NewKeyPair succeeded with 31 random bytes")
	}
}

func newKeyPair(t *testing.T) curve.KeyPair {
	t.Helper()
	kp, err := curve.NewKeyPair(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return kp
}
