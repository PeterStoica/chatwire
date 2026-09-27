package message_test

import (
	"bytes"
	"crypto/rand"
	"errors"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/wire"
)

func TestPadUsesTheLowNibblePlusOne(t *testing.T) {
	for seed := range 256 {
		padded, err := message.Pad(bytes.NewReader([]byte{byte(seed)}), []byte("hi"))
		if err != nil {
			t.Fatal(err)
		}
		size := seed%16 + 1
		if len(padded) != 2+size || !bytes.Equal(padded[2:], bytes.Repeat([]byte{byte(size)}, size)) || string(padded[:2]) != "hi" {
			t.Fatalf("seed %#x: padded = %x", seed, padded)
		}
	}
	if _, err := message.Pad(bytes.NewReader(nil), []byte("hi")); err == nil {
		t.Fatal("Pad succeeded without randomness")
	}
}

func TestPadDoesNotTouchTheInput(t *testing.T) {
	plaintext := make([]byte, 2, 64)
	copy(plaintext, "hi")
	if _, err := message.Pad(bytes.NewReader([]byte{3}), plaintext); err != nil {
		t.Fatal(err)
	}
	if extended := plaintext[:4]; extended[2] != 0 || extended[3] != 0 {
		t.Fatalf("Pad wrote into the caller's spare capacity: %x", extended)
	}
}

func TestUnpad(t *testing.T) {
	for _, tt := range []struct {
		padded []byte
		want   []byte
		err    error
	}{
		{nil, nil, message.ErrPadding},
		{[]byte{2}, nil, message.ErrPadding},
		{[]byte{1}, []byte{}, nil},
		{[]byte{7, 0}, []byte{7, 0}, nil},
		{[]byte{7, 2, 2}, []byte{7}, nil},
		{[]byte{7, 9, 2}, []byte{7}, nil},
	} {
		got, err := message.Unpad(tt.padded)
		if !errors.Is(err, tt.err) || !bytes.Equal(got, tt.want) {
			t.Errorf("Unpad(%x) = %x, %v, want %x, %v", tt.padded, got, err, tt.want, tt.err)
		}
	}
}

func TestEncodeThenDecode(t *testing.T) {
	sent := &wire.Message{Conversation: new("salut")}
	padded, err := message.Encode(rand.Reader, sent)
	if err != nil {
		t.Fatal(err)
	}
	received, err := message.Decode(padded)
	if err != nil || !proto.Equal(received, sent) {
		t.Fatalf("Decode(Encode()) = %v, %v", received, err)
	}
	if _, err := message.Decode([]byte{0xff, 1}); err == nil {
		t.Fatal("Decode accepted a truncated protobuf")
	}
	if _, err := message.Decode(nil); !errors.Is(err, message.ErrPadding) {
		t.Fatalf("Decode(nil) = %v", err)
	}
}

func TestOneEmoji(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		in   string
		want bool
	}{
		{"👍", true}, {"❤️", true}, {"❤", true}, {"👍🏽", true}, {"👨‍👩‍👧", true}, {"🇷🇴", true}, {"1️⃣", true}, {"#⃣", true},
		{"🏴\U000E0067\U000E0062\U000E0073\U000E0063\U000E0074\U000E007F", true}, {"😂", true}, {"🙏", true}, {"😮", true}, {"™️", true},
		{"", false}, {"not an emoji", false}, {"a", false}, {"👍👍", false}, {"👍a", false}, {"1", false}, {"🏽", false}, {"‍👍", false},
		{"👍‍", false}, {"👍‍‍👍", false}, {"🇷", false}, {"🇷🇴🇷🇴", false}, {"🇷🇴👍", false}, {" 👍", false}, {"11⃣", false},
	} {
		if got := message.OneEmoji(tt.in); got != tt.want {
			t.Errorf("OneEmoji(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
