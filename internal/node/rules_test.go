package node_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/PeterStoica/chatwire/internal/node"
)

const (
	lastSingleToken = "screen_height"
	tokenType       = 0x04
	tokenID         = 0x08
	tokenTo         = 0x11
	tokenResult     = 0x14
	tokenIQ         = 0x19
	tokenServer     = 0x03
)

func TestUnmarshalOutsideASmallDictionary(t *testing.T) {
	dict, err := node.NewDictionary(node.Tables{Version: 3, Single: []string{"", "iq"}, Double: [][]string{{"a"}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, frame := range [][]byte{
		{0, 0xf8, 0x02, 0x01, 0xed, 0x00},
		{0, 0xf8, 0x02, 0x01, 0xec, 0x01},
		{0, 0xf8, 0x02, 0x01, 0x02},
	} {
		if _, err := dict.Unmarshal(frame); !errors.Is(err, node.ErrInvalidToken) {
			t.Fatalf("Unmarshal(% x) error = %v, want %v", frame, err, node.ErrInvalidToken)
		}
	}
	got, err := dict.Unmarshal([]byte{0, 0xf8, 0x02, 0x01, 0xec, 0x00})
	if err != nil || string(got.Bytes) != "a" {
		t.Fatalf("Unmarshal(double token) = %v, %v", got, err)
	}
}

func TestUnmarshalRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		frame []byte
		want  node.Node
		err   error
	}{
		{
			name:  "empty input",
			frame: []byte{},
			err:   node.ErrTruncated,
		},
		{
			name:  "truncated after attribute key",
			frame: []byte{0, 0xf8, 0x03, tokenIQ, tokenID},
			err:   node.ErrTruncated,
		},
		{
			name:  "truncated binary content",
			frame: []byte{0, 0xf8, 0x02, tokenIQ, 0xfc, 0x05, 'A'},
			err:   node.ErrTruncated,
		},
		{
			name:  "truncated child list",
			frame: []byte{0, 0xf8, 0x02, tokenIQ, 0xf8},
			err:   node.ErrTruncated,
		},
		{
			name:  "invalid child",
			frame: []byte{0, 0xf8, 0x02, tokenIQ, 0xf8, 0x01, 0xf8, 0x00},
			err:   node.ErrInvalidNode,
		},
		{
			name:  "list as tag",
			frame: []byte{0, 0xf8, 0x01, 0xf8, 0x01, tokenIQ},
			err:   node.ErrUnexpectedList,
		},
		{
			name:  "last single token",
			frame: []byte{0, 0xf8, 0x02, tokenIQ, 0xeb},
			want:  node.Node{Tag: "iq", Bytes: []byte(lastSingleToken)},
		},
		{
			name:  "unassigned tag",
			frame: []byte{0, 0xf8, 0x01, 0xf0},
			err:   node.ErrInvalidToken,
		},
		{
			name:  "unsupported jid kind",
			frame: []byte{0, 0xf8, 0x03, tokenIQ, tokenTo, 0xf6},
			err:   node.ErrInvalidToken,
		},

		{
			name:  "list16 child count",
			frame: []byte{0, 0xf8, 0x02, tokenIQ, 0xf9, 0x00, 0x01, 0xf8, 0x01, tokenIQ},
			want:  node.Node{Tag: "iq", Children: []node.Node{{Tag: "iq"}}},
		},
		{
			name:  "server-only jid",
			frame: []byte{0, 0xf8, 0x03, tokenIQ, tokenTo, 0xfa, 0x00, tokenServer},
			want:  node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "to", Value: node.Address(node.JID{Server: node.ServerUser})}}},
		},
		{
			name:  "jid pair truncated before the user",
			frame: []byte{0, 0xf8, 0x03, tokenIQ, tokenTo, 0xfa},
			err:   node.ErrTruncated,
		},
		{
			name:  "bare node",
			frame: []byte{0, 0xf8, 0x01, tokenIQ},
			want:  node.Node{Tag: "iq"},
		},
		{
			name:  "trailing bytes after the root node are ignored, as the official client does",
			frame: []byte{0, 0xf8, 0x01, tokenIQ, 0x00, 0xff},
			want:  node.Node{Tag: "iq"},
		},
		{
			name:  "binary20 content",
			frame: []byte{0, 0xf8, 0x02, tokenIQ, 0xfd, 0x00, 0x00, 0x01, 'A'},
			want:  node.Node{Tag: "iq", Bytes: []byte("A")},
		},
		{
			name:  "binary20 length high bits are masked, as the official client does",
			frame: []byte{0, 0xf8, 0x02, tokenIQ, 0xfd, 0x10, 0x00, 0x01, 'A'},
			want:  node.Node{Tag: "iq", Bytes: []byte("A")},
		},
		{
			name:  "duplicate attribute key keeps the last value, as the official client does",
			frame: []byte{0, 0xf8, 0x05, tokenIQ, tokenID, tokenResult, tokenID, tokenType},
			want:  node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "id", Value: node.Text("type")}}},
		},
		{
			name:  "duplicate attribute key with an empty last value drops the key",
			frame: []byte{0, 0xf8, 0x05, tokenIQ, tokenID, tokenResult, tokenID, 0x00},
			want:  node.Node{Tag: "iq"},
		},
		{
			name:  "list as attribute value",
			frame: []byte{0, 0xf8, 0x03, tokenIQ, tokenID, 0xf8, 0x01, 0xf8, 0x01, tokenIQ},
			err:   node.ErrUnexpectedList,
		},
		{
			name:  "jid with empty server",
			frame: []byte{0, 0xf8, 0x03, tokenIQ, tokenTo, 0xfa, 0xff, 0x82, 0x12, 0x3f, 0xfc, 0x00},
			err:   node.ErrEmptyJIDServer,
		},
		{
			name:  "jid as attribute key",
			frame: []byte{0, 0xf8, 0x03, tokenIQ, 0xfa, 0x00, tokenServer, tokenResult},
			err:   node.ErrInvalidToken,
		},
		{
			name:  "jid as tag",
			frame: []byte{0, 0xf8, 0x01, 0xfa, 0x00, tokenServer},
			err:   node.ErrInvalidToken,
		},
		{
			name:  "jid as the user of a jid",
			frame: []byte{0, 0xf8, 0x03, tokenIQ, tokenTo, 0xfa, 0xfa, 0x00, tokenServer, tokenServer},
			err:   node.ErrInvalidToken,
		},
		{
			name:  "padding nibble inside a packed string becomes U+FFFD, as the official client does",
			frame: []byte{0, 0xf8, 0x03, tokenIQ, tokenID, 0xff, 0x02, 0x1f, 0x23},
			want:  node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "id", Value: node.Text("1\uFFFD23")}}},
		},
		{
			name:  "invalid nibbles become U+FFFD, as the official client does",
			frame: []byte{0, 0xf8, 0x03, tokenIQ, tokenID, 0xff, 0x02, 0xab, 0xcd},
			want:  node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "id", Value: node.Text("-.\uFFFD\uFFFD")}}},
		},
		{
			name:  "odd packed string ignores its last low nibble, as the official client does",
			frame: []byte{0, 0xf8, 0x03, tokenIQ, tokenID, 0xff, 0x82, 0x12, 0x34},
			want:  node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "id", Value: node.Text("123")}}},
		},
		{
			name:  "odd packed string with no bytes",
			frame: []byte{0, 0xf8, 0x03, tokenIQ, tokenID, 0xff, 0x80},
			err:   node.ErrInvalidToken,
		},
		{
			name:  "odd nibble string",
			frame: []byte{0, 0xf8, 0x03, tokenIQ, tokenID, 0xff, 0x82, 0x12, 0x3f},
			want:  node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "id", Value: node.Text("123")}}},
		},
		{
			name:  "odd hex string",
			frame: []byte{0, 0xf8, 0x03, tokenIQ, tokenID, 0xfb, 0x81, 0xaf},
			want:  node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "id", Value: node.Text("A")}}},
		},
		{
			name:  "empty attribute value is kept, as the official client keeps it",
			frame: []byte{0, 0xf8, 0x03, tokenIQ, tokenType, 0xfc, 0x00},
			want:  node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "type", Value: node.Text("")}}},
		},
		{
			name:  "null attribute value is dropped",
			frame: []byte{0, 0xf8, 0x03, tokenIQ, tokenType, 0x00},
			want:  node.Node{Tag: "iq"},
		},
		{
			name:  "lid device jid",
			frame: []byte{0, 0xf8, 0x03, tokenIQ, tokenTo, 0xf7, 0x01, 0x02, 0xff, 0x81, 0x1f},
			want:  node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "to", Value: node.Address(node.JID{User: "1", Device: 2, Server: node.ServerLID})}}},
		},
		{
			name:  "hosted jid",
			frame: []byte{0, 0xf8, 0x03, tokenIQ, tokenTo, 0xf7, 0x80, 0x00, 0xff, 0x81, 0x1f},
			want:  node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "to", Value: node.Address(node.JID{User: "1", Server: node.ServerHosted})}}},
		},
		{
			name:  "unknown jid domain type is rejected, as the official client rejects it",
			frame: []byte{0, 0xf8, 0x03, tokenIQ, tokenTo, 0xf7, 0x07, 0x02, 0xff, 0x81, 0x1f},
			err:   node.ErrJIDDomain,
		},
		{
			name:  "any even domain type with the high bit is hosted, as the official client reads it",
			frame: []byte{0, 0xf8, 0x03, tokenIQ, tokenTo, 0xf7, 0xfe, 0x63, 0xff, 0x81, 0x1f},
			want:  node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "to", Value: node.Address(node.JID{User: "1", Device: 99, Server: node.ServerHosted})}}},
		},
		{
			name:  "odd domain type above hosted lid is rejected, as the official client rejects it",
			frame: []byte{0, 0xf8, 0x03, tokenIQ, tokenTo, 0xf7, 0x83, 0x02, 0xff, 0x81, 0x1f},
			err:   node.ErrJIDDomain,
		},
	}
	dict := dictionary(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := dict.Unmarshal(tt.frame)
			if tt.err != nil {
				if !errors.Is(err, tt.err) {
					t.Fatalf("Unmarshal() error = %v, want %v", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Unmarshal() = %#v, want %#v", got, tt.want)
			}
			encoded, err := dict.Marshal(got)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if again, err := dict.Unmarshal(encoded); err != nil || !reflect.DeepEqual(again, tt.want) {
				t.Fatalf("round trip = %#v, %v", again, err)
			}
		})
	}
}

func TestUnmarshalLargestBinary20(t *testing.T) {
	const size = 1<<20 - 1
	frame := append([]byte{0, 0xf8, 0x02, tokenIQ, 0xfd, 0x0f, 0xff, 0xff}, make([]byte, size)...)
	got, err := dictionary(t).Unmarshal(frame)
	if err != nil || len(got.Bytes) != size {
		t.Fatalf("Unmarshal(binary20 of 0xfffff) = %d bytes, %v", len(got.Bytes), err)
	}
}
