package node

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxPrintable = 64

var (
	ErrTruncated      = errors.New("node: truncated input")
	ErrInvalidToken   = errors.New("node: invalid token")
	ErrInvalidNode    = errors.New("node: invalid node")
	ErrTooLarge       = errors.New("node: inflated frame too large")
	ErrUnexpectedList = errors.New("node: list where a single value was expected")
	ErrEmptyJIDServer = errors.New("node: jid has no server")
	ErrJIDDomain      = errors.New("node: unknown jid domain type")
	ErrMixedContent   = errors.New("node: more than one kind of content")
	ErrUnencodable    = errors.New("node: value cannot be encoded")
	ErrJID            = errors.New("node: malformed jid")
)

type Attr struct {
	Key   string
	Value Value
}

type Node struct {
	Tag      string
	Attrs    []Attr
	Children []Node
	Bytes    []byte
}

func (n Node) With(key string, value Value) Node {
	attrs := make([]Attr, 0, len(n.Attrs)+1)
	replaced := false
	for _, a := range n.Attrs {
		if a.Key == key {
			a.Value, replaced = value, true
		}
		attrs = append(attrs, a)
	}
	if !replaced {
		attrs = append(attrs, Attr{Key: key, Value: value})
	}
	n.Attrs = attrs
	return n
}

func (n Node) Attr(key string) Value {
	for _, a := range n.Attrs {
		if a.Key == key {
			return a.Value
		}
	}
	return Value{}
}

func (n Node) Child(tag string) (Node, bool) {
	for _, child := range n.Children {
		if child.Tag == tag {
			return child, true
		}
	}
	return Node{}, false
}

func (n Node) String() string {
	var b strings.Builder
	n.write(&b, 0)
	return b.String()
}

func (n Node) write(b *strings.Builder, depth int) {
	indent := strings.Repeat("  ", depth)
	b.WriteString(indent)
	b.WriteString("<")
	b.WriteString(n.Tag)
	for _, a := range n.Attrs {
		fmt.Fprintf(b, " %s=%q", a.Key, a.Value)
	}
	switch {
	case len(n.Children) > 0:
		b.WriteString(">\n")
		for _, child := range n.Children {
			child.write(b, depth+1)
		}
		b.WriteString(indent)
		b.WriteString("</")
		b.WriteString(n.Tag)
		b.WriteString(">\n")
	case n.Bytes != nil && printable(n.Bytes):
		fmt.Fprintf(b, ">%s</%s>\n", n.Bytes, n.Tag)
	case n.Bytes != nil:
		fmt.Fprintf(b, ">[%d bytes]</%s>\n", len(n.Bytes), n.Tag)
	default:
		b.WriteString("/>\n")
	}
}

func (n Node) contentKinds() int {
	kinds := 0
	if len(n.Children) > 0 {
		kinds++
	}
	if n.Bytes != nil {
		kinds++
	}
	return kinds
}

func printable(raw []byte) bool {
	if len(raw) == 0 || len(raw) > maxPrintable || !utf8.Valid(raw) {
		return false
	}
	for _, r := range string(raw) {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}
