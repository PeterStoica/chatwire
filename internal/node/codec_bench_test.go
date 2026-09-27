package node_test

import (
	"bytes"
	"testing"

	"github.com/PeterStoica/chatwire/internal/node"
)

func BenchmarkMarshalAttributes(b *testing.B) {
	dict := dictionary(b)
	n := node.Node{Tag: "message", Attrs: []node.Attr{
		{Key: "to", Value: node.Address(node.JID{User: "40700000000", Server: node.ServerUser})},
		{Key: "id", Value: node.Text("0123456789ABCDEF")},
		{Key: "type", Value: node.Text("text")},
		{Key: "offline", Value: node.Value{}},
	}}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := dict.Marshal(n); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnmarshalPacked(b *testing.B) {
	dict := dictionary(b)
	for _, tc := range []struct {
		name string
		tag  byte
		pair byte
	}{
		{"digits", 0xff, 0x12},
		{"hex", 0xfb, 0xab},
		{"invalid nibbles", 0xff, 0xcf},
	} {
		b.Run(tc.name, func(b *testing.B) {
			frame := append([]byte{0, 0xf8, 3, tokenIQ, tokenID, tc.tag, 64}, bytes.Repeat([]byte{tc.pair}, 64)...)
			b.ReportAllocs()
			for b.Loop() {
				if _, err := dict.Unmarshal(frame); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkUnmarshalChildren(b *testing.B) {
	dict := dictionary(b)
	n := node.Node{Tag: "list", Children: make([]node.Node, 32)}
	for i := range n.Children {
		n.Children[i] = node.Node{Tag: "item"}
	}
	frame, err := dict.Marshal(n)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := dict.Unmarshal(frame); err != nil {
			b.Fatal(err)
		}
	}
}
