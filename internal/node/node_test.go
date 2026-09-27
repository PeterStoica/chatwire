package node_test

import (
	"bytes"
	"compress/zlib"
	"errors"
	"testing"

	"github.com/PeterStoica/chatwire/internal/node"
)

func TestUnmarshalInflatesUpToTheCap(t *testing.T) {
	const limit = 64 << 20
	header := []byte{0xf8, 0x02, tokenIQ, 0xfe}
	fits := append(bytes.Clone(header), 0, 0, 0, 0)
	payload := limit - len(fits)
	fits[4], fits[5], fits[6], fits[7] = byte(payload>>24), byte(payload>>16), byte(payload>>8), byte(payload)
	got, err := dictionary(t).Unmarshal(compress(t, fits, payload))
	if err != nil {
		t.Fatalf("exactly 64 MiB inflated: %v", err)
	}
	if len(got.Bytes) != payload {
		t.Fatalf("content is %d bytes, want %d", len(got.Bytes), payload)
	}
	if _, err := dictionary(t).Unmarshal(compress(t, fits, payload+1)); !errors.Is(err, node.ErrTooLarge) {
		t.Fatalf("64 MiB + 1 inflated: %v, want %v", err, node.ErrTooLarge)
	}
}

func TestUnmarshalInflatesCompressedFrame(t *testing.T) {
	got, err := dictionary(t).Unmarshal(compress(t, []byte{0xf8, 0x01, tokenIQ}, 0))
	if err != nil {
		t.Fatal(err)
	}
	if got.Tag != "iq" || got.Attrs != nil || got.Children != nil || got.Bytes != nil {
		t.Fatalf("Unmarshal() = %+v, want bare <iq/>", got)
	}
}

func TestUnmarshalRejectsBrokenCompression(t *testing.T) {
	if _, err := dictionary(t).Unmarshal([]byte{2, 0x78, 0x9c, 0xff}); err == nil {
		t.Fatal("accepted a corrupt zlib stream")
	}
}

func compress(t *testing.T, prefix []byte, zeros int) []byte {
	t.Helper()
	var out bytes.Buffer
	out.WriteByte(2)
	writer, err := zlib.NewWriterLevel(&out, zlib.BestSpeed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(prefix); err != nil {
		t.Fatal(err)
	}
	chunk := make([]byte, 32<<10)
	for zeros > 0 {
		n := min(zeros, len(chunk))
		if _, err := writer.Write(chunk[:n]); err != nil {
			t.Fatal(err)
		}
		zeros -= n
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func FuzzUnmarshalIsTotal(f *testing.F) {
	f.Add([]byte{0, 0xf8, 0x01, 0x19})
	f.Add([]byte{0, 0xf8, 0x01, 0xff, 0x80})
	f.Add([]byte{2, 0x78, 0x9c})
	dict := dictionary(f)
	f.Fuzz(func(t *testing.T, frame []byte) {
		n, err := dict.Unmarshal(frame)
		if err == nil {
			assertCanonicalShape(t, n, frame)
		}
	})
}

func assertCanonicalShape(t *testing.T, n node.Node, frame []byte) {
	t.Helper()
	if n.Attrs != nil && len(n.Attrs) == 0 {
		t.Fatalf("empty but non-nil attributes from %x", frame)
	}
	seen := map[string]bool{}
	for _, a := range n.Attrs {
		if a.Value.IsZero() || seen[a.Key] {
			t.Fatalf("attribute %q absent or repeated from %x", a.Key, frame)
		}
		seen[a.Key] = true
	}
	if n.Children != nil && n.Bytes != nil {
		t.Fatalf("children and bytes together from %x", frame)
	}
	for _, child := range n.Children {
		assertCanonicalShape(t, child, frame)
	}
}

func dictionary(tb testing.TB) node.Dictionary {
	tb.Helper()
	dict, err := node.LoadDictionary()
	if err != nil {
		tb.Fatal(err)
	}
	return dict
}

func FuzzDecodedNodesRoundTrip(f *testing.F) {
	f.Add([]byte{0, 0xf8, 0x01, 0x19})
	f.Add([]byte{0, 0xf8, 0x03, 0x19, 0x08, 0xfa, 0x00, 0x03})
	f.Add([]byte{0, 0xf8, 0x02, 0x19, 0xf8, 0x01, 0xf8, 0x01, 0x13})
	dict := dictionary(f)
	f.Fuzz(func(t *testing.T, frame []byte) {
		first, err := dict.Unmarshal(frame)
		if err != nil {
			return
		}
		encoded, err := dict.Marshal(first)
		if err != nil {
			t.Fatalf("cannot re-encode %s: %v", first, err)
		}
		second, err := dict.Unmarshal(encoded)
		if err != nil {
			t.Fatalf("cannot decode our own encoding of %s: %v", first, err)
		}
		if first.String() != second.String() {
			t.Fatalf("round trip changed the node\nfirst  %s\nsecond %s", first, second)
		}
	})
}

func TestNestingDepthIsCappedInsteadOfExhaustingTheStack(t *testing.T) {
	dict := dictionary(t)
	nested := func(depth int) []byte {
		frame := []byte{0}
		for range depth - 1 {
			frame = append(frame, 0xf8, 0x02, 0x19, 0xf8, 0x01)
		}
		return append(frame, 0xf8, 0x01, 0x19)
	}
	if _, err := dict.Unmarshal(nested(256)); err != nil {
		t.Fatalf("256 levels: %v", err)
	}
	if _, err := dict.Unmarshal(nested(257)); !errors.Is(err, node.ErrInvalidNode) {
		t.Fatalf("257 levels: %v, want %v", err, node.ErrInvalidNode)
	}
	var body bytes.Buffer
	writer := zlib.NewWriter(&body)
	deep := nested(1_000_000)
	if _, err := writer.Write(deep[1:]); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	compressed := append([]byte{2}, body.Bytes()...)
	if _, err := dict.Unmarshal(compressed); !errors.Is(err, node.ErrInvalidNode) {
		t.Fatalf("a %d-byte compressed frame nesting a million levels: %v, want %v", len(compressed), err, node.ErrInvalidNode)
	}
}

func TestSiblingsDoNotCountAsNesting(t *testing.T) {
	dict := dictionary(t)
	wide := node.Node{Tag: "list", Children: make([]node.Node, 1000)}
	for i := range wide.Children {
		wide.Children[i] = node.Node{Tag: "item", Children: []node.Node{{Tag: "leaf"}}}
	}
	encoded, err := dict.Marshal(wide)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := dict.Unmarshal(encoded)
	if err != nil || len(decoded.Children) != len(wide.Children) {
		t.Fatalf("a node with %d children: %d decoded, %v", len(wide.Children), len(decoded.Children), err)
	}
}

func TestEmptyChildListDecodesAsNoChildren(t *testing.T) {
	n, err := dictionary(t).Unmarshal([]byte{0, 0xf8, 0x02, 0x19, 0xf8, 0x00})
	if err != nil || n.Children != nil {
		t.Fatalf("Unmarshal = %#v, %v", n, err)
	}
}
