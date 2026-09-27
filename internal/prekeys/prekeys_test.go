package prekeys_test

import (
	"bytes"
	"crypto/rand"
	"errors"
	"strings"
	"testing"

	"github.com/PeterStoica/chatwire/internal/curve"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/prekeys"
	"github.com/PeterStoica/chatwire/internal/signon"
)

func TestGenerateNumbersKeysAndWrapsAt24Bits(t *testing.T) {
	keys, next, err := prekeys.Generate(rand.Reader, prekeys.MaxID-1, 4)
	if err != nil {
		t.Fatal(err)
	}
	want := []uint32{prekeys.MaxID - 1, prekeys.MaxID, 1, 2}
	seen := map[curve.PublicKey]bool{}
	for i, k := range keys {
		if k.ID != want[i] || seen[k.Key.Public()] {
			t.Fatalf("key %d has id %d (want %d), repeated %v", i, k.ID, want[i], seen[k.Key.Public()])
		}
		seen[k.Key.Public()] = true
	}
	if next != 3 {
		t.Fatalf("next id = %d, want 3", next)
	}
}

func TestGenerateStartsAtTheLargestID(t *testing.T) {
	keys, next, err := prekeys.Generate(rand.Reader, prekeys.MaxID, 2)
	if err != nil || keys[0].ID != prekeys.MaxID || keys[1].ID != 1 || next != 2 {
		t.Fatalf("Generate(first %d) = ids %d, %d, next %d, %v", prekeys.MaxID, keys[0].ID, keys[1].ID, next, err)
	}
}

func TestGenerateRefusesIDsOutside24Bits(t *testing.T) {
	for _, first := range []uint32{0, prekeys.MaxID + 1} {
		if _, _, err := prekeys.Generate(rand.Reader, first, 1); !errors.Is(err, prekeys.ErrID) {
			t.Errorf("Generate(first %d) error = %v, want %v", first, err, prekeys.ErrID)
		}
	}
	if _, _, err := prekeys.Generate(bytes.NewReader(make([]byte, 40)), 1, 2); err == nil {
		t.Error("Generate succeeded without enough randomness for two keys")
	}
}

func TestUploadCarriesTheBundleInWhatsAppsLayout(t *testing.T) {
	keys, _, err := prekeys.Generate(rand.Reader, 0x010203, 2)
	if err != nil {
		t.Fatal(err)
	}
	identity, signedKey := key(t).Public(), key(t).Public()
	registration := signon.Registration{
		RegistrationID: 0x0a0b0c0d, Identity: identity,
		SignedPreKey: signon.SignedPreKey{ID: 0x040506, Key: signedKey, Signature: curve.Signature{9}},
	}
	upload := prekeys.Upload(registration, keys)
	attrs := make([]string, 0, len(upload.Attrs))
	for _, a := range upload.Attrs {
		attrs = append(attrs, a.Key+"="+a.Value.String())
	}
	if got := strings.Join(attrs, " "); upload.Tag != "iq" || got != "id= xmlns=encrypt type=set to=s.whatsapp.net" {
		t.Fatalf("iq attributes %q", got)
	}
	first, second := keys[0].Key.Public(), keys[1].Key.Public()
	for _, tt := range []struct {
		path []string
		want []byte
	}{
		{[]string{"registration"}, []byte{0x0a, 0x0b, 0x0c, 0x0d}},
		{[]string{"type"}, []byte{0x05}},
		{[]string{"identity"}, identity[:]},
		{[]string{"skey", "id"}, []byte{4, 5, 6}},
		{[]string{"skey", "value"}, signedKey[:]},
		{[]string{"skey", "signature"}, append([]byte{9}, make([]byte, curve.SignatureSize-1)...)},
	} {
		if got := walk(t, upload, tt.path...).Bytes; !bytes.Equal(got, tt.want) {
			t.Errorf("%v = %x, want %x", tt.path, got, tt.want)
		}
	}
	list := walk(t, upload, "list")
	if len(list.Children) != 2 {
		t.Fatalf("list holds %d keys", len(list.Children))
	}
	for i, want := range []struct {
		id    []byte
		value curve.PublicKey
	}{{[]byte{1, 2, 3}, first}, {[]byte{1, 2, 4}, second}} {
		k := list.Children[i]
		if k.Tag != "key" || !bytes.Equal(walk(t, k, "id").Bytes, want.id) || !bytes.Equal(walk(t, k, "value").Bytes, want.value[:]) {
			t.Fatalf("key %d = %s", i, k)
		}
	}
	tags := make([]string, 0, len(upload.Children))
	for _, child := range upload.Children {
		tags = append(tags, child.Tag)
	}
	if got := strings.Join(tags, ","); got != "registration,type,identity,list,skey" {
		t.Fatalf("children in order %s", got)
	}
}

func TestResultClassifiesTheReply(t *testing.T) {
	failure := func(code string) node.Node {
		return node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "type", Value: node.Text("error")}}, Children: []node.Node{
			{Tag: "error", Attrs: []node.Attr{{Key: "code", Value: node.Text(code)}, {Key: "text", Value: node.Text("x")}}},
		}}
	}
	for _, tt := range []struct {
		name  string
		reply node.Node
		want  error
	}{
		{"result", node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "type", Value: node.Text("result")}}}, nil},
		{"invalid keys", failure("406"), prekeys.ErrRejected},
		{"server busy", failure("503"), prekeys.ErrBackoff},
		{"lowest backoff code", failure("500"), prekeys.ErrBackoff},
		{"other error", failure("499"), prekeys.ErrReply},
		{"error without code", failure(""), prekeys.ErrReply},
		{"error without error child", node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "type", Value: node.Text("error")}}}, prekeys.ErrReply},
		{"not an iq", node.Node{Tag: "message", Attrs: []node.Attr{{Key: "type", Value: node.Text("result")}}}, prekeys.ErrReply},
		{"a get", node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "type", Value: node.Text("get")}}}, prekeys.ErrReply},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := prekeys.Result(tt.reply); !errors.Is(err, tt.want) || (tt.want == nil) != (err == nil) {
				t.Fatalf("Result() = %v, want %v", err, tt.want)
			}
		})
	}
}

func key(t *testing.T) curve.KeyPair {
	t.Helper()
	pair, err := curve.NewKeyPair(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pair
}

func walk(t *testing.T, n node.Node, path ...string) node.Node {
	t.Helper()
	for _, tag := range path {
		child, ok := n.Child(tag)
		if !ok {
			t.Fatalf("no %s under %s", tag, n.Tag)
		}
		n = child
	}
	return n
}
