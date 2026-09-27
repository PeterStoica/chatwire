package node_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/PeterStoica/chatwire/internal/node"
)

const (
	user     = "1234567"
	tokenLID = 118
)

var userBytes = []byte{0xff, 0x84, 0x12, 0x34, 0x56, 0x7f}

func join(parts ...[]byte) []byte {
	return bytes.Join(parts, nil)
}

func iq(attrs []node.Attr) node.Node {
	return node.Node{Tag: "iq", Attrs: attrs}
}

func to(j node.JID) []node.Attr {
	return []node.Attr{{Key: "to", Value: node.Address(j)}}
}

func id(text string) []node.Attr {
	return []node.Attr{{Key: "id", Value: node.Text(text)}}
}

func TestMarshalBytes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		node node.Node
		want []byte
	}{
		{name: "bare node", node: iq(nil), want: []byte{0, 0xf8, 0x01, tokenIQ}},
		{name: "empty children are no content, as the official client writes them", node: node.Node{Tag: "iq", Children: []node.Node{}}, want: []byte{0, 0xf8, 0x01, tokenIQ}},
		{name: "empty byte content", node: node.Node{Tag: "iq", Bytes: []byte{}}, want: []byte{0, 0xf8, 0x02, tokenIQ, 0xfc, 0x00}},
		{name: "content that is a token stays bytes", node: node.Node{Tag: "iq", Bytes: []byte("result")}, want: []byte{0, 0xf8, 0x02, tokenIQ, 0xfc, 0x06, 'r', 'e', 's', 'u', 'l', 't'}},
		{name: "double token", node: iq(id("reaction")), want: []byte{0, 0xf8, 0x03, tokenIQ, tokenID, 0xec, 0x04}},
		{name: "double token from table 1", node: iq(id("reject")), want: []byte{0, 0xf8, 0x03, tokenIQ, tokenID, 0xed, 0x00}},
		{name: "empty attribute value is binary8 of length 0, as the official client writes it", node: iq(id("")), want: []byte{0, 0xf8, 0x03, tokenIQ, tokenID, 0xfc, 0x00}},
		{name: "empty tag is binary8 of length 0, as the official client writes it", node: node.Node{}, want: []byte{0, 0xf8, 0x01, 0xfc, 0x00}},
		{name: "absent attribute value is omitted", node: node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "id", Value: node.Value{}}}}, want: []byte{0, 0xf8, 0x01, tokenIQ}},
		{name: "attributes keep their order", node: iq([]node.Attr{{Key: "type", Value: node.Text("result")}, {Key: "id", Value: node.Text("result")}}),
			want: []byte{0, 0xf8, 0x05, tokenIQ, tokenType, tokenResult, tokenID, tokenResult}},
		{name: "user jid", node: iq(to(node.JID{User: user, Server: node.ServerUser})), want: join([]byte{0, 0xf8, 0x03, tokenIQ, tokenTo, 0xfa}, userBytes, []byte{tokenServer})},
		{name: "server-only jid", node: iq(to(node.JID{Server: node.ServerUser})), want: []byte{0, 0xf8, 0x03, tokenIQ, tokenTo, 0xfa, 0x00, tokenServer}},
		{name: "user device jid", node: iq(to(node.JID{User: user, Device: 2, Server: node.ServerUser})), want: join([]byte{0, 0xf8, 0x03, tokenIQ, tokenTo, 0xf7, 0x00, 0x02}, userBytes)},
		{name: "lid without device", node: iq(to(node.JID{User: user, Server: node.ServerLID})), want: join([]byte{0, 0xf8, 0x03, tokenIQ, tokenTo, 0xfa}, userBytes, []byte{tokenLID})},
		{name: "lid device", node: iq(to(node.JID{User: user, Device: 2, Server: node.ServerLID})), want: join([]byte{0, 0xf8, 0x03, tokenIQ, tokenTo, 0xf7, 0x01, 0x02}, userBytes)},
		{name: "hosted", node: iq(to(node.JID{User: user, Server: node.ServerHosted})), want: join([]byte{0, 0xf8, 0x03, tokenIQ, tokenTo, 0xf7, 0x80, 0x00}, userBytes)},
		{name: "hosted lid", node: iq(to(node.JID{User: user, Server: node.ServerHostedLID})), want: join([]byte{0, 0xf8, 0x03, tokenIQ, tokenTo, 0xf7, 0x81, 0x00}, userBytes)},
		{name: "nibble alphabet edges", node: iq(id("09-.")), want: []byte{0, 0xf8, 0x03, tokenIQ, tokenID, 0xff, 0x02, 0x09, 0xab}},
		{name: "hex alphabet edges", node: iq(id("0AF9")), want: []byte{0, 0xf8, 0x03, tokenIQ, tokenID, 0xfb, 0x02, 0x0a, 0xf9}},
		{name: "odd hex", node: iq(id("A")), want: []byte{0, 0xf8, 0x03, tokenIQ, tokenID, 0xfb, 0x81, 0xaf}},
		{name: "slash is raw", node: iq(id("/")), want: []byte{0, 0xf8, 0x03, tokenIQ, tokenID, 0xfc, 0x01, '/'}},
		{name: "colon is raw", node: iq(id(":")), want: []byte{0, 0xf8, 0x03, tokenIQ, tokenID, 0xfc, 0x01, ':'}},
		{name: "at sign is raw", node: iq(id("@")), want: []byte{0, 0xf8, 0x03, tokenIQ, tokenID, 0xfc, 0x01, '@'}},
		{name: "G is raw", node: iq(id("G")), want: []byte{0, 0xf8, 0x03, tokenIQ, tokenID, 0xfc, 0x01, 'G'}},
		{name: "lowercase hex is raw", node: iq(id("a")), want: []byte{0, 0xf8, 0x03, tokenIQ, tokenID, 0xfc, 0x01, 'a'}},
	}
	dict := dictionary(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := dict.Marshal(tt.node)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Fatalf("Marshal() = % x\n          want % x", got, tt.want)
			}
		})
	}
}

func TestMarshalPackingLimit(t *testing.T) {
	dict := dictionary(t)
	packed, err := dict.Marshal(iq(id(strings.Repeat("1", 127))))
	if err != nil {
		t.Fatal(err)
	}
	if packed[5] != 0xff || packed[6] != 0x80|64 {
		t.Fatalf("127 digits: tag %#x head %#x, want packed nibbles", packed[5], packed[6])
	}
	raw, err := dict.Marshal(iq(id(strings.Repeat("1", 128))))
	if err != nil {
		t.Fatal(err)
	}
	if raw[5] != 0xfc || raw[6] != 128 {
		t.Fatalf("128 digits: tag %#x length %d, want raw binary8", raw[5], raw[6])
	}
}

func TestMarshalBinaryLengths(t *testing.T) {
	tests := []struct {
		size   int
		header []byte
	}{
		{size: 255, header: []byte{0xfc, 0xff}},
		{size: 256, header: []byte{0xfd, 0x00, 0x01, 0x00}},
		{size: 1<<20 - 1, header: []byte{0xfd, 0x0f, 0xff, 0xff}},
		{size: 1 << 20, header: []byte{0xfe, 0x00, 0x10, 0x00, 0x00}},
		{size: 1 << 24, header: []byte{0xfe, 0x01, 0x00, 0x00, 0x00}},
	}
	dict := dictionary(t)
	for _, tt := range tests {
		frame, err := dict.Marshal(node.Node{Tag: "iq", Bytes: make([]byte, tt.size)})
		if err != nil {
			t.Fatalf("%d bytes: %v", tt.size, err)
		}
		if header := frame[4 : 4+len(tt.header)]; !bytes.Equal(header, tt.header) {
			t.Fatalf("%d bytes: header % x, want % x", tt.size, header, tt.header)
		}
		if len(frame) != 4+len(tt.header)+tt.size {
			t.Fatalf("%d bytes: frame is %d bytes", tt.size, len(frame))
		}
	}
	if _, err := dict.Marshal(node.Node{Tag: "iq", Bytes: make([]byte, 1<<24+1)}); !errors.Is(err, node.ErrUnencodable) {
		t.Fatalf("16 MiB + 1: %v, want %v", err, node.ErrUnencodable)
	}
}

func TestMarshalListLengths(t *testing.T) {
	dict := dictionary(t)
	for _, tt := range []struct {
		children int
		header   []byte
	}{
		{children: 255, header: []byte{0xf8, 0xff}},
		{children: 256, header: []byte{0xf9, 0x01, 0x00}},
		{children: 65535, header: []byte{0xf9, 0xff, 0xff}},
	} {
		children := make([]node.Node, tt.children)
		for i := range children {
			children[i] = node.Node{Tag: "iq"}
		}
		frame, err := dict.Marshal(node.Node{Tag: "iq", Children: children})
		if err != nil {
			t.Fatalf("%d children: %v", tt.children, err)
		}
		if header := frame[4 : 4+len(tt.header)]; !bytes.Equal(header, tt.header) {
			t.Fatalf("%d children: header % x, want % x", tt.children, header, tt.header)
		}
	}
	children := make([]node.Node, 65536)
	for i := range children {
		children[i] = node.Node{Tag: "iq"}
	}
	if _, err := dict.Marshal(node.Node{Tag: "iq", Children: children}); !errors.Is(err, node.ErrUnencodable) {
		t.Fatalf("65536 children: %v, want %v", err, node.ErrUnencodable)
	}
}

func TestMarshalErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		node node.Node
		want error
	}{
		{name: "bytes and children", node: node.Node{Tag: "iq", Bytes: []byte{1}, Children: []node.Node{{Tag: "iq"}}}, want: node.ErrMixedContent},
		{name: "attribute jid without server", node: iq(to(node.JID{User: user})), want: node.ErrUnencodable},
		{name: "device on a group", node: iq(to(node.JID{User: user, Device: 1, Server: "g.us"})), want: node.ErrUnencodable},
		{name: "second child unencodable", node: node.Node{Tag: "iq", Children: []node.Node{{Tag: "iq"}, iq(to(node.JID{User: user}))}}, want: node.ErrUnencodable},
		{name: "attribute twice", node: iq([]node.Attr{{Key: "id", Value: node.Text("1")}, {Key: "id", Value: node.Text("2")}}), want: node.ErrUnencodable},
	}
	dict := dictionary(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := dict.Marshal(tt.node); !errors.Is(err, tt.want) {
				t.Fatalf("Marshal() error = %v, want %v", err, tt.want)
			}
		})
	}
}
