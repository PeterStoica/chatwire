package curve

import (
	"bytes"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/sha512"
	"errors"
	"fmt"
	"io"

	"filippo.io/edwards25519"
	"filippo.io/edwards25519/field"
)

const (
	KeySize       = 32
	SignatureSize = 64
	KeyType       = 0x05
)

var (
	ErrInvalidSignature = errors.New("curve: invalid signature")
	ErrTypedKey         = errors.New("curve: not a type-prefixed public key")
)

type PublicKey [KeySize]byte

type Signature [SignatureSize]byte

type KeyPair struct {
	private *ecdh.PrivateKey
	public  PublicKey
}

func NewKeyPair(random io.Reader) (KeyPair, error) {
	var seed [KeySize]byte
	if _, err := io.ReadFull(random, seed[:]); err != nil {
		return KeyPair{}, fmt.Errorf("curve: read seed: %w", err)
	}
	seed[0] &= 0xf8
	seed[KeySize-1] = seed[KeySize-1]&0x7f | 0x40
	private, err := ecdh.X25519().NewPrivateKey(seed[:])
	if err != nil {
		return KeyPair{}, fmt.Errorf("curve: new private key: %w", err)
	}
	return KeyPair{private: private, public: PublicKey(private.PublicKey().Bytes())}, nil
}

func (kp KeyPair) Seed() [KeySize]byte {
	return [KeySize]byte(kp.private.Bytes())
}

func ParsePublicKey(raw []byte) (PublicKey, error) {
	if len(raw) != KeySize {
		return PublicKey{}, fmt.Errorf("curve: public key is %d bytes, want %d", len(raw), KeySize)
	}
	return PublicKey(raw), nil
}

func ParseTypedPublicKey(raw []byte) (PublicKey, error) {
	if len(raw) != KeySize+1 || raw[0] != KeyType {
		return PublicKey{}, fmt.Errorf("%w: %d bytes", ErrTypedKey, len(raw))
	}
	return PublicKey(raw[1:]), nil
}

func (k PublicKey) Typed() []byte {
	return append([]byte{KeyType}, k[:]...)
}

func (kp KeyPair) Public() PublicKey {
	return kp.public
}

func (kp KeyPair) SharedSecret(peer PublicKey) ([]byte, error) {
	remote, err := ecdh.X25519().NewPublicKey(peer[:])
	if err != nil {
		return nil, fmt.Errorf("curve: peer key: %w", err)
	}
	secret, err := kp.private.ECDH(remote)
	if err != nil {
		return nil, fmt.Errorf("curve: ecdh: %w", err)
	}
	return secret, nil
}

func (kp KeyPair) Sign(random io.Reader, message []byte) (Signature, error) {
	var nonceSeed [64]byte
	if _, err := io.ReadFull(random, nonceSeed[:]); err != nil {
		return Signature{}, fmt.Errorf("curve: read nonce seed: %w", err)
	}
	seed := kp.private.Bytes()
	scalar, err := edwards25519.NewScalar().SetBytesWithClamping(seed)
	if err != nil {
		return Signature{}, fmt.Errorf("curve: private scalar: %w", err)
	}
	edPublic := new(edwards25519.Point).ScalarBaseMult(scalar).Bytes()

	domain := bytes.Repeat([]byte{0xff}, KeySize)
	domain[0] = 0xfe
	nonceHash := sha512.New()
	nonceHash.Write(domain)
	nonceHash.Write(seed)
	nonceHash.Write(message)
	nonceHash.Write(nonceSeed[:])
	nonce, err := edwards25519.NewScalar().SetUniformBytes(nonceHash.Sum(nil))
	if err != nil {
		return Signature{}, fmt.Errorf("curve: nonce scalar: %w", err)
	}
	commitment := new(edwards25519.Point).ScalarBaseMult(nonce).Bytes()

	challengeHash := sha512.New()
	challengeHash.Write(commitment)
	challengeHash.Write(edPublic)
	challengeHash.Write(message)
	challenge, err := edwards25519.NewScalar().SetUniformBytes(challengeHash.Sum(nil))
	if err != nil {
		return Signature{}, fmt.Errorf("curve: challenge scalar: %w", err)
	}
	response := edwards25519.NewScalar().MultiplyAdd(challenge, scalar, nonce).Bytes()

	var signature Signature
	copy(signature[:KeySize], commitment)
	copy(signature[KeySize:], response)
	signature[SignatureSize-1] |= edPublic[KeySize-1] & 0x80
	return signature, nil
}

func (kp KeyPair) SignKey(random io.Reader, key PublicKey) (Signature, error) {
	return kp.Sign(random, key.Typed())
}

func Verify(key PublicKey, message []byte, signature Signature) error {
	montgomery := key
	montgomery[KeySize-1] &= 0x7f
	var u, one, numerator, denominator field.Element
	if _, err := u.SetBytes(montgomery[:]); err != nil {
		return fmt.Errorf("curve: public key element: %w", err)
	}
	one.One()
	numerator.Subtract(&u, &one)
	denominator.Add(&u, &one)
	denominator.Invert(&denominator)
	edwardsY := new(field.Element).Multiply(&numerator, &denominator).Bytes()
	edwardsY[KeySize-1] |= signature[SignatureSize-1] & 0x80
	signature[SignatureSize-1] &= 0x7f
	if !ed25519.Verify(edwardsY, message, signature[:]) {
		return ErrInvalidSignature
	}
	return nil
}
