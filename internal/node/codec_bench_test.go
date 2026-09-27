package node_test

import (
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
