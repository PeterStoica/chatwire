package signal

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"slices"

	"github.com/PeterStoica/chatwire/internal/curve"
)

const (
	keySize           = 32
	macSize           = 8
	infoText          = "WhisperText"
	infoRatchet       = "WhisperRatchet"
	infoMessageKeys   = "WhisperMessageKeys"
	infoGroup         = "WhisperGroup"
	messageKeysLength = 80
	groupKeysLength   = 48
)

var (
	ErrMAC     = errors.New("signal: message authentication failed")
	ErrPadding = errors.New("signal: bad padding")
)

type chainKey struct {
	key   [keySize]byte
	index uint32
}

type messageKeys struct {
	cipher [keySize]byte
	mac    [keySize]byte
	iv     [aes.BlockSize]byte
	index  uint32
}

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

func (c chainKey) next() chainKey {
	return chainKey{key: [keySize]byte(hmacSHA256(c.key[:], []byte{0x02})), index: c.index + 1}
}

func (c chainKey) messageKeys() (messageKeys, error) {
	seed := hmacSHA256(c.key[:], []byte{0x01})
	material, err := hkdf.Key(sha256.New, seed, nil, infoMessageKeys, messageKeysLength)
	if err != nil {
		return messageKeys{}, fmt.Errorf("signal: message keys: %w", err)
	}
	return messageKeys{
		cipher: [keySize]byte(material[:32]),
		mac:    [keySize]byte(material[32:64]),
		iv:     [aes.BlockSize]byte(material[64:80]),
		index:  c.index,
	}, nil
}

func (c chainKey) groupKeys() (messageKeys, error) {
	material, err := hkdf.Key(sha256.New, hmacSHA256(c.key[:], []byte{0x01}), nil, infoGroup, groupKeysLength)
	if err != nil {
		return messageKeys{}, fmt.Errorf("signal: group keys: %w", err)
	}
	return messageKeys{
		iv:     [aes.BlockSize]byte(material[:aes.BlockSize]),
		cipher: [keySize]byte(material[aes.BlockSize:]),
		index:  c.index,
	}, nil
}

type skippingChain struct {
	chain   chainKey
	skipped []messageKeys
}

func (c *skippingChain) keys(counter uint32, derive func(chainKey) (messageKeys, error)) (messageKeys, error) {
	if c.chain.index > counter {
		for i, skipped := range c.skipped {
			if skipped.index == counter {
				c.skipped = append(c.skipped[:i:i], c.skipped[i+1:]...)
				return skipped, nil
			}
		}
		return messageKeys{}, fmt.Errorf("%w: counter %d, chain at %d", ErrDuplicate, counter, c.chain.index)
	}
	if counter-c.chain.index > maxFutureMessages {
		return messageKeys{}, fmt.Errorf("%w: counter %d, chain at %d", ErrTooFarAhead, counter, c.chain.index)
	}
	for range counter - c.chain.index {
		keys, err := derive(c.chain)
		if err != nil {
			return messageKeys{}, err
		}
		c.skipped = append(c.skipped, keys)
		if len(c.skipped) > maxSkippedKeys {
			c.skipped = c.skipped[1:]
		}
		c.chain = c.chain.next()
	}
	keys, err := derive(c.chain)
	if err != nil {
		return messageKeys{}, err
	}
	c.chain = c.chain.next()
	return keys, nil
}

func (c *skippingChain) clone() skippingChain {
	return skippingChain{chain: c.chain, skipped: slices.Clip(c.skipped)}
}

func derive(material, salt []byte, info string) ([keySize]byte, chainKey, error) {
	out, err := hkdf.Key(sha256.New, material, salt, info, 2*keySize)
	if err != nil {
		return [keySize]byte{}, chainKey{}, fmt.Errorf("signal: hkdf: %w", err)
	}
	return [keySize]byte(out[:keySize]), chainKey{key: [keySize]byte(out[keySize:])}, nil
}

func step(root [keySize]byte, theirs curve.PublicKey, ours curve.KeyPair) ([keySize]byte, chainKey, error) {
	shared, err := ours.SharedSecret(theirs)
	if err != nil {
		return [keySize]byte{}, chainKey{}, err
	}
	return derive(shared, root[:], infoRatchet)
}

func encryptCBC(keys messageKeys, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(keys.cipher[:])
	if err != nil {
		return nil, err
	}
	padding := aes.BlockSize - len(plaintext)%aes.BlockSize
	padded := make([]byte, len(plaintext)+padding)
	copy(padded, plaintext)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = byte(padding)
	}
	cipher.NewCBCEncrypter(block, keys.iv[:]).CryptBlocks(padded, padded)
	return padded, nil
}

func decryptCBC(keys messageKeys, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("%w: %d bytes of ciphertext", ErrPadding, len(ciphertext))
	}
	block, err := aes.NewCipher(keys.cipher[:])
	if err != nil {
		return nil, err
	}
	plain := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, keys.iv[:]).CryptBlocks(plain, ciphertext)
	padding := int(plain[len(plain)-1])
	if padding == 0 || padding > aes.BlockSize {
		return nil, ErrPadding
	}
	for _, b := range plain[len(plain)-padding:] {
		if int(b) != padding {
			return nil, ErrPadding
		}
	}
	return plain[:len(plain)-padding], nil
}

func mac(keys messageKeys, sender, receiver curve.PublicKey, serialized []byte) []byte {
	h := hmac.New(sha256.New, keys.mac[:])
	h.Write(sender.Typed())
	h.Write(receiver.Typed())
	h.Write(serialized)
	return h.Sum(nil)[:macSize]
}

func untyped(raw []byte) (curve.PublicKey, error) {
	key, err := curve.ParseTypedPublicKey(raw)
	if err != nil {
		return curve.PublicKey{}, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	return key, nil
}
