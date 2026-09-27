package message

import (
	"errors"
	"fmt"
	"github.com/PeterStoica/chatwire/internal/wire"
	"io"

	"google.golang.org/protobuf/proto"
)

var ErrPadding = errors.New("message: bad padding")

func Pad(random io.Reader, plaintext []byte) ([]byte, error) {
	var seed [1]byte
	if _, err := io.ReadFull(random, seed[:]); err != nil {
		return nil, fmt.Errorf("message: padding: %w", err)
	}
	size := seed[0]&15 + 1
	padded := make([]byte, len(plaintext)+int(size))
	copy(padded, plaintext)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = size
	}
	return padded, nil
}

func Unpad(padded []byte) ([]byte, error) {
	if len(padded) == 0 {
		return nil, fmt.Errorf("%w: empty plaintext", ErrPadding)
	}
	size := int(padded[len(padded)-1])
	if size > len(padded) {
		return nil, fmt.Errorf("%w: %d bytes with a pad of %d", ErrPadding, len(padded), size)
	}
	return padded[:len(padded)-size], nil
}

func Encode(random io.Reader, m *wire.Message) ([]byte, error) {
	raw, err := proto.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("message: encode: %w", err)
	}
	return Pad(random, raw)
}

func Decode(padded []byte) (*wire.Message, error) {
	raw, err := Unpad(padded)
	if err != nil {
		return nil, err
	}
	var m wire.Message
	if err := proto.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("message: decode: %w", err)
	}
	return &m, nil
}
