package appstate

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"encoding/json/v2"
	"errors"
	"fmt"
	"github.com/PeterStoica/chatwire/internal/wire"
	"io"
	"slices"

	"google.golang.org/protobuf/proto"
)

const (
	mutationKeysInfo   = "WhatsApp Mutation Keys"
	patchIntegrityInfo = "WhatsApp Patch Integrity"
	keySize            = 32
	macSize            = 32
	ivSize             = aes.BlockSize
	lengthField        = 8
	opSet              = 0x01
	opRemove           = 0x02
)

var (
	ErrKey       = errors.New("appstate: bad sync key")
	ErrValueMAC  = errors.New("appstate: value mac mismatch")
	ErrIndexMAC  = errors.New("appstate: index mac mismatch")
	ErrMalformed = errors.New("appstate: malformed mutation")
)

type Keys struct {
	Index           [keySize]byte
	ValueEncryption [keySize]byte
	ValueMAC        [keySize]byte
	SnapshotMAC     [keySize]byte
	PatchMAC        [keySize]byte
}

func Expand(keyData []byte) (Keys, error) {
	if len(keyData) == 0 {
		return Keys{}, ErrKey
	}
	derived, err := hkdf.Key(sha256.New, keyData, nil, mutationKeysInfo, 5*keySize)
	if err != nil {
		return Keys{}, fmt.Errorf("%w: %w", ErrKey, err)
	}
	var k Keys
	for i, part := range []*[keySize]byte{&k.Index, &k.ValueEncryption, &k.ValueMAC, &k.SnapshotMAC, &k.PatchMAC} {
		copy(part[:], derived[i*keySize:])
	}
	return k, nil
}

type Mutation struct {
	Operation wire.SyncdMutation_SyncdOperation
	Index     []string
	Value     *wire.SyncActionValue
	Version   int32
	IndexMAC  []byte
	ValueMAC  []byte
}

func operationByte(op wire.SyncdMutation_SyncdOperation) (byte, error) {
	switch op {
	case wire.SyncdMutation_SET:
		return opSet, nil
	case wire.SyncdMutation_REMOVE:
		return opRemove, nil
	default:
		return 0, fmt.Errorf("%w: operation %d", ErrMalformed, op)
	}
}

func valueMAC(key []byte, op byte, keyID, ivAndCiphertext []byte) []byte {
	associated := slices.Concat([]byte{op}, keyID)
	mac := hmac.New(sha512.New, key)
	mac.Write(associated)
	mac.Write(ivAndCiphertext)
	mac.Write(binary.BigEndian.AppendUint64(nil, uint64(len(associated))))
	return mac.Sum(nil)[:macSize]
}

func Decrypt(op wire.SyncdMutation_SyncdOperation, record *wire.SyncdRecord, keys Keys) (Mutation, error) {
	opByte, err := operationByte(op)
	if err != nil {
		return Mutation{}, err
	}
	blob := record.GetValue().GetBlob()
	if len(blob) < ivSize+aes.BlockSize+macSize || len(blob)%aes.BlockSize != 0 {
		return Mutation{}, fmt.Errorf("%w: value of %d bytes", ErrMalformed, len(blob))
	}
	body, mac := blob[:len(blob)-macSize], blob[len(blob)-macSize:]
	if !hmac.Equal(mac, valueMAC(keys.ValueMAC[:], opByte, record.GetKeyId().GetId(), body)) {
		return Mutation{}, ErrValueMAC
	}
	plain, err := decryptCBC(keys.ValueEncryption[:], body[:ivSize], body[ivSize:])
	if err != nil {
		return Mutation{}, err
	}
	var data wire.SyncActionData
	if err := proto.Unmarshal(plain, &data); err != nil {
		return Mutation{}, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	indexMAC := hmac.New(sha256.New, keys.Index[:])
	indexMAC.Write(data.GetIndex())
	if !hmac.Equal(indexMAC.Sum(nil), record.GetIndex().GetBlob()) {
		return Mutation{}, ErrIndexMAC
	}
	var index []string
	if err := json.Unmarshal(data.GetIndex(), &index); err != nil {
		return Mutation{}, fmt.Errorf("%w: index: %w", ErrMalformed, err)
	}
	return Mutation{
		Operation: op, Index: index, Value: data.GetValue(), Version: data.GetVersion(),
		IndexMAC: record.GetIndex().GetBlob(), ValueMAC: mac,
	}, nil
}

func decryptCBC(key, iv, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	plain := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, ciphertext)
	pad := int(plain[len(plain)-1])
	if pad == 0 || pad > aes.BlockSize {
		return nil, fmt.Errorf("%w: padding", ErrMalformed)
	}
	for _, b := range plain[len(plain)-pad:] {
		if int(b) != pad {
			return nil, fmt.Errorf("%w: padding", ErrMalformed)
		}
	}
	return plain[:len(plain)-pad], nil
}

func networkVersion(version uint64) []byte {
	out := make([]byte, lengthField)
	binary.BigEndian.PutUint32(out[4:], uint32(version&0xffffffff))
	return out
}

func SnapshotMAC(keys Keys, hash LTHash, version uint64, collection string) []byte {
	mac := hmac.New(sha256.New, keys.SnapshotMAC[:])
	mac.Write(hash[:])
	mac.Write(networkVersion(version))
	mac.Write([]byte(collection))
	return mac.Sum(nil)
}

func PatchMAC(keys Keys, snapshotMAC []byte, valueMACs [][]byte, version uint64, collection string) []byte {
	mac := hmac.New(sha256.New, keys.PatchMAC[:])
	mac.Write(snapshotMAC)
	for _, v := range valueMACs {
		mac.Write(v)
	}
	mac.Write(networkVersion(version))
	mac.Write([]byte(collection))
	return mac.Sum(nil)
}

const ltHashSize = 128

type LTHash [ltHashSize]byte

func (h LTHash) Add(macs ...[]byte) (LTHash, error) {
	return h.apply(macs, func(a, b uint16) uint16 { return a + b })
}

func (h LTHash) Subtract(macs ...[]byte) (LTHash, error) {
	return h.apply(macs, func(a, b uint16) uint16 { return a - b })
}

func (h LTHash) apply(macs [][]byte, combine func(a, b uint16) uint16) (LTHash, error) {
	for _, mac := range macs {
		expanded, err := hkdf.Key(sha256.New, mac, nil, patchIntegrityInfo, ltHashSize)
		if err != nil {
			return h, fmt.Errorf("appstate: lthash: %w", err)
		}
		for i := 0; i < ltHashSize; i += 2 {
			binary.LittleEndian.PutUint16(h[i:], combine(binary.LittleEndian.Uint16(h[i:]), binary.LittleEndian.Uint16(expanded[i:])))
		}
	}
	return h, nil
}

func Encrypt(random io.Reader, op wire.SyncdMutation_SyncdOperation, index []string, value *wire.SyncActionValue, version int32, keyID []byte, keys Keys) (*wire.SyncdRecord, error) {
	opByte, err := operationByte(op)
	if err != nil {
		return nil, err
	}
	rawIndex, err := json.Marshal(index)
	if err != nil {
		return nil, fmt.Errorf("%w: index: %w", ErrMalformed, err)
	}
	plain, err := proto.Marshal(&wire.SyncActionData{Index: rawIndex, Value: value, Padding: []byte{}, Version: &version})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	iv := make([]byte, ivSize)
	if _, err := io.ReadFull(random, iv); err != nil {
		return nil, fmt.Errorf("appstate: iv: %w", err)
	}
	block, err := aes.NewCipher(keys.ValueEncryption[:])
	if err != nil {
		return nil, err
	}
	pad := aes.BlockSize - len(plain)%aes.BlockSize
	padded := slices.Concat(plain, bytes.Repeat([]byte{byte(pad)}, pad))
	ciphertext := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ciphertext, padded)
	body := slices.Concat(iv, ciphertext)
	indexMAC := hmac.New(sha256.New, keys.Index[:])
	indexMAC.Write(rawIndex)
	return &wire.SyncdRecord{
		Index: &wire.SyncdIndex{Blob: indexMAC.Sum(nil)},
		Value: &wire.SyncdValue{Blob: slices.Concat(body, valueMAC(keys.ValueMAC[:], opByte, keyID, body))},
		KeyId: &wire.KeyId{Id: keyID},
	}, nil
}
