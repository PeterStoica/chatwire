package noise

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"github.com/PeterStoica/chatwire/internal/curve"
)

const (
	protocolName = "Noise_XX_25519_AESGCM_SHA256\x00\x00\x00\x00"
	keySize      = 32
	nonceSize    = 12
)

type SymmetricState struct {
	hash    []byte
	chain   []byte
	aead    cipher.AEAD
	counter uint32
}

func NewSymmetricState(prologue []byte) (*SymmetricState, error) {
	initial := []byte(protocolName)
	aead, err := newAEAD(initial)
	if err != nil {
		return nil, err
	}
	state := &SymmetricState{hash: initial, chain: initial, aead: aead}
	state.MixHash(prologue)
	return state, nil
}

func (s *SymmetricState) HandshakeHash() []byte {
	return bytes.Clone(s.hash)
}

func (s *SymmetricState) MixHash(data []byte) {
	digest := sha256.New()
	digest.Write(s.hash)
	digest.Write(data)
	s.hash = digest.Sum(nil)
}

func (s *SymmetricState) MixKey(material []byte) error {
	chain, key, err := expand(s.chain, material)
	if err != nil {
		return err
	}
	aead, err := newAEAD(key)
	if err != nil {
		return err
	}
	s.chain, s.aead, s.counter = chain, aead, 0
	return nil
}

func (s *SymmetricState) MixDH(local curve.KeyPair, remote curve.PublicKey) error {
	secret, err := local.SharedSecret(remote)
	if err != nil {
		return err
	}
	return s.MixKey(secret)
}

func (s *SymmetricState) EncryptAndHash(plaintext []byte) []byte {
	var iv [nonceSize]byte
	ciphertext := s.aead.Seal(nil, nonce(&iv, s.counter), plaintext, s.hash)
	s.counter++
	s.MixHash(ciphertext)
	return ciphertext
}

func (s *SymmetricState) DecryptAndHash(ciphertext []byte) ([]byte, error) {
	var iv [nonceSize]byte
	plaintext, err := s.aead.Open(nil, nonce(&iv, s.counter), ciphertext, s.hash)
	if err != nil {
		return nil, fmt.Errorf("noise: decrypt handshake payload: %w", err)
	}
	s.counter++
	s.MixHash(ciphertext)
	return plaintext, nil
}

func (s *SymmetricState) split(swap bool) (*Transport, error) {
	write, read, err := expand(s.chain, nil)
	if err != nil {
		return nil, err
	}
	if swap {
		write, read = read, write
	}
	writer, err := newAEAD(write)
	if err != nil {
		return nil, err
	}
	reader, err := newAEAD(read)
	if err != nil {
		return nil, err
	}
	return &Transport{writer: writer, reader: reader}, nil
}

type Transport struct {
	writer     cipher.AEAD
	reader     cipher.AEAD
	writeCount uint32
	readCount  uint32
	writeIV    [nonceSize]byte
	readIV     [nonceSize]byte
}

func (t *Transport) Seal(plaintext []byte) []byte {
	ciphertext := t.writer.Seal(nil, nonce(&t.writeIV, t.writeCount), plaintext, nil)
	t.writeCount++
	return ciphertext
}

func (t *Transport) OpenInPlace(ciphertext []byte) ([]byte, error) {
	plaintext, err := t.reader.Open(ciphertext[:0], nonce(&t.readIV, t.readCount), ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("noise: decrypt frame %d: %w", t.readCount, err)
	}
	t.readCount++
	return plaintext, nil
}

func expand(salt, material []byte) (first, second []byte, err error) {
	okm, err := hkdf.Key(sha256.New, material, salt, "", 2*keySize)
	if err != nil {
		return nil, nil, fmt.Errorf("noise: hkdf: %w", err)
	}
	return okm[:keySize], okm[keySize:], nil
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("noise: aes: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("noise: gcm: %w", err)
	}
	return aead, nil
}

func nonce(iv *[nonceSize]byte, counter uint32) []byte {
	binary.BigEndian.PutUint32(iv[nonceSize-4:], counter)
	return iv[:]
}
