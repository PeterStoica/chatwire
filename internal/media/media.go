package media

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
)

type Type string

const (
	Image    Type = "image"
	Sticker  Type = "sticker"
	Video    Type = "video"
	GIF      Type = "gif"
	Audio    Type = "audio"
	Voice    Type = "ptt"
	Document Type = "document"
	AppState Type = "md-app-state"
	History  Type = "md-msg-hist"

	KeySize    = 32
	macSize    = 10
	keysLength = 112
)

var (
	ErrType     = errors.New("media: unknown media type")
	ErrKey      = errors.New("media: media key must be 32 bytes")
	ErrMAC      = errors.New("media: file authentication failed")
	ErrHash     = errors.New("media: file hash mismatch")
	ErrTooShort = errors.New("media: file too short")
	ErrPadding  = errors.New("media: bad padding")
)

type Keys struct {
	IV     [aes.BlockSize]byte
	Cipher [KeySize]byte
	MAC    [KeySize]byte
	Ref    [KeySize]byte
}

type Encrypted struct {
	File          []byte
	FileSHA256    [sha256.Size]byte
	FileEncSHA256 [sha256.Size]byte
	FileLength    int
}

func info(t Type) (string, error) {
	switch t {
	case Document:
		return "WhatsApp Document Keys", nil
	case Image, Sticker, "xma-image":
		return "WhatsApp Image Keys", nil
	case Video, GIF:
		return "WhatsApp Video Keys", nil
	case Audio, Voice:
		return "WhatsApp Audio Keys", nil
	case AppState:
		return "WhatsApp App State Keys", nil
	case History:
		return "WhatsApp History Keys", nil
	default:
		return "", fmt.Errorf("%w: %q", ErrType, t)
	}
}

func Derive(mediaKey []byte, t Type) (Keys, error) {
	if len(mediaKey) != KeySize {
		return Keys{}, fmt.Errorf("%w: got %d", ErrKey, len(mediaKey))
	}
	label, err := info(t)
	if err != nil {
		return Keys{}, err
	}
	material, err := hkdf.Key(sha256.New, mediaKey, nil, label, keysLength)
	if err != nil {
		return Keys{}, fmt.Errorf("media: hkdf: %w", err)
	}
	return Keys{
		IV:     [aes.BlockSize]byte(material[:16]),
		Cipher: [KeySize]byte(material[16:48]),
		MAC:    [KeySize]byte(material[48:80]),
		Ref:    [KeySize]byte(material[80:112]),
	}, nil
}

func Encrypt(mediaKey []byte, t Type, plaintext []byte) (Encrypted, error) {
	keys, err := Derive(mediaKey, t)
	if err != nil {
		return Encrypted{}, err
	}
	block, err := aes.NewCipher(keys.Cipher[:])
	if err != nil {
		return Encrypted{}, fmt.Errorf("media: cipher: %w", err)
	}
	pad := aes.BlockSize - len(plaintext)%aes.BlockSize
	file := make([]byte, len(plaintext)+pad, len(plaintext)+pad+macSize)
	copy(file, plaintext)
	for i := len(plaintext); i < len(file); i++ {
		file[i] = byte(pad)
	}
	cipher.NewCBCEncrypter(block, keys.IV[:]).CryptBlocks(file, file)
	file = append(file, sign(keys, file)...)
	return Encrypted{File: file, FileSHA256: sha256.Sum256(plaintext), FileEncSHA256: sha256.Sum256(file), FileLength: len(plaintext)}, nil
}

func sign(keys Keys, ciphertext []byte) []byte {
	mac := hmac.New(sha256.New, keys.MAC[:])
	mac.Write(keys.IV[:])
	mac.Write(ciphertext)
	return mac.Sum(nil)[:macSize]
}

func Decrypt(mediaKey []byte, t Type, file, fileEncSHA256, fileSHA256 []byte) ([]byte, error) {
	keys, err := Derive(mediaKey, t)
	if err != nil {
		return nil, err
	}
	if fileEncSHA256 != nil {
		if sum := sha256.Sum256(file); !hmac.Equal(sum[:], fileEncSHA256) {
			return nil, fmt.Errorf("%w: encrypted file", ErrHash)
		}
	}
	if len(file) < macSize+aes.BlockSize || (len(file)-macSize)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("%w: %d bytes", ErrTooShort, len(file))
	}
	ciphertext, mac := file[:len(file)-macSize], file[len(file)-macSize:]
	if !hmac.Equal(sign(keys, ciphertext), mac) {
		return nil, ErrMAC
	}
	block, err := aes.NewCipher(keys.Cipher[:])
	if err != nil {
		return nil, fmt.Errorf("media: cipher: %w", err)
	}
	plain := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, keys.IV[:]).CryptBlocks(plain, ciphertext)
	pad := int(plain[len(plain)-1])
	if pad == 0 || pad > aes.BlockSize || !bytes.Equal(plain[len(plain)-pad:], bytes.Repeat([]byte{byte(pad)}, pad)) {
		return nil, ErrPadding
	}
	plain = plain[:len(plain)-pad]
	if fileSHA256 != nil {
		if sum := sha256.Sum256(plain); !hmac.Equal(sum[:], fileSHA256) {
			return nil, fmt.Errorf("%w: plaintext", ErrHash)
		}
	}
	return plain, nil
}
