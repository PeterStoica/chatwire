package node_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/PeterStoica/chatwire/internal/node"
)

func TestJIDString(t *testing.T) {
	tests := []struct {
		jid  node.JID
		want string
	}{
		{jid: node.JID{}, want: ""},
		{jid: node.JID{Server: node.ServerUser}, want: "s.whatsapp.net"},
		{jid: node.JID{User: "1", Server: node.ServerUser}, want: "1@s.whatsapp.net"},
		{jid: node.JID{User: "1", Device: 2, Server: node.ServerLID}, want: "1:2@lid"},
	}
	for _, tt := range tests {
		if got := tt.jid.String(); got != tt.want {
			t.Errorf("%#v.String() = %q, want %q", tt.jid, got, tt.want)
		}
	}
}

func TestValueAccessors(t *testing.T) {
	jid := node.JID{User: "1", Device: 2, Server: node.ServerLID}
	address, text, zero := node.Address(jid), node.Text("hello"), node.Value{}
	if got, ok := address.JID(); !ok || got != jid {
		t.Fatalf("Address.JID() = %v, %v", got, ok)
	}
	if _, ok := address.Text(); ok {
		t.Fatal("Address.Text() reports text")
	}
	if got, ok := text.Text(); !ok || got != "hello" {
		t.Fatalf("Text.Text() = %q, %v", got, ok)
	}
	if _, ok := text.JID(); ok {
		t.Fatal("Text.JID() reports a jid")
	}
	if address.String() != "1:2@lid" || text.String() != "hello" || zero.String() != "" {
		t.Fatalf("String() = %q, %q, %q", address.String(), text.String(), zero.String())
	}
	if !zero.IsZero() || address.IsZero() || text.IsZero() || node.Text("").IsZero() {
		t.Fatal("IsZero is only true for the zero value")
	}
}

func TestStringShowsShortPrintableContentAsText(t *testing.T) {
	for _, tt := range []struct {
		content []byte
		want    string
	}{
		{content: []byte{}, want: "<iq>[0 bytes]</iq>\n"},
		{content: []byte("hello"), want: "<iq>hello</iq>\n"},
		{content: bytes.Repeat([]byte("a"), 64), want: "<iq>" + strings.Repeat("a", 64) + "</iq>\n"},
		{content: bytes.Repeat([]byte("a"), 65), want: "<iq>[65 bytes]</iq>\n"},
		{content: []byte("bell\a"), want: "<iq>[5 bytes]</iq>\n"},
		{content: []byte{0xff, 0xfe}, want: "<iq>[2 bytes]</iq>\n"},
	} {
		if got := (node.Node{Tag: "iq", Bytes: tt.content}).String(); got != tt.want {
			t.Errorf("String() of %q = %q, want %q", tt.content, got, tt.want)
		}
	}
	n := node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "type", Value: node.Text("get")}, {Key: "id", Value: node.Text("1")}}}
	if got := n.String(); got != "<iq type=\"get\" id=\"1\"/>\n" {
		t.Errorf("String() = %q, want attributes in their order", got)
	}
}

func TestDeviceAddressedValues(t *testing.T) {
	dict := dictionary(t)
	for _, tt := range []struct {
		name string
		jid  node.JID
		want []byte
	}{
		{"primary phone", node.JID{User: "7", Server: node.ServerUser}, []byte{0xf7, 0, 0}},
		{"companion", node.JID{User: "7", Device: 3, Server: node.ServerUser}, []byte{0xf7, 0, 3}},
		{"lid", node.JID{User: "7", Server: node.ServerLID}, []byte{0xf7, 1, 0}},
		{"hosted", node.JID{User: "7", Device: 99, Server: node.ServerHosted}, []byte{0xf7, 128, 99}},
		{"hosted lid", node.JID{User: "7", Server: node.ServerHostedLID}, []byte{0xf7, 129, 0}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			value := node.Device(tt.jid)
			if jid, ok := value.JID(); !ok || jid != tt.jid || value.String() != tt.jid.String() || value.IsZero() {
				t.Fatalf("Device(%v) = %v, %v", tt.jid, jid, ok)
			}
			encoded, err := dict.Marshal(node.Node{Tag: "to", Attrs: []node.Attr{{Key: "jid", Value: value}}})
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(encoded, tt.want) {
				t.Fatalf("encoded %x, want the device form %x", encoded, tt.want)
			}
			decoded, err := dict.Unmarshal(encoded)
			if got, _ := decoded.Attr("jid").JID(); err != nil || got != tt.jid {
				t.Fatalf("decoded %v, %v", got, err)
			}
		})
	}
	if _, err := dict.Marshal(node.Node{Tag: "to", Attrs: []node.Attr{{Key: "jid", Value: node.Device(node.JID{User: "1", Server: "g.us"})}}}); !errors.Is(err, node.ErrUnencodable) {
		t.Fatalf("a group addressed by device: %v", err)
	}
	plain, err := dict.Marshal(node.Node{Tag: "to", Attrs: []node.Attr{{Key: "jid", Value: node.Address(node.JID{User: "7", Server: node.ServerUser})}}})
	if err != nil || bytes.Contains(plain, []byte{0xf7}) {
		t.Fatalf("Address of a primary phone uses the device form: %x, %v", plain, err)
	}
}

func TestParseJID(t *testing.T) {
	t.Parallel()
	user := func(u string, device uint8, server node.Server) node.JID {
		return node.JID{User: u, Device: device, Server: server}
	}
	valid := []struct {
		name string
		in   string
		want node.JID
	}{
		{name: "phone", in: "40712345678@s.whatsapp.net", want: user("40712345678", 0, node.ServerUser)},
		{name: "legacy domain", in: "40712345678@c.us", want: user("40712345678", 0, node.ServerUser)},
		{name: "shortest phone", in: "12345@s.whatsapp.net", want: user("12345", 0, node.ServerUser)},
		{name: "longest phone", in: "12345678901234567890@s.whatsapp.net", want: user("12345678901234567890", 0, node.ServerUser)},
		{name: "server user", in: "0@s.whatsapp.net", want: user("0", 0, node.ServerUser)},
		{name: "device", in: "40712345678:7@s.whatsapp.net", want: user("40712345678", 7, node.ServerUser)},
		{name: "two digit device", in: "40712345678:99@s.whatsapp.net", want: user("40712345678", 99, node.ServerUser)},
		{name: "device zero", in: "40712345678:0@s.whatsapp.net", want: user("40712345678", 0, node.ServerUser)},
		{name: "agent and device", in: "40712345678.0:3@s.whatsapp.net", want: user("40712345678", 3, node.ServerUser)},
		{name: "lid", in: "123456789012345@lid", want: user("123456789012345", 0, node.ServerLID)},
		{name: "lid of one digit", in: "7@lid", want: user("7", 0, node.ServerLID)},
		{name: "lid device", in: "88123:12@lid", want: user("88123", 12, node.ServerLID)},
		{name: "hosted", in: "40712345678:99@hosted", want: user("40712345678", 99, node.ServerHosted)},
		{name: "hosted lid", in: "88123:99@hosted.lid", want: user("88123", 99, node.ServerHostedLID)},
		{name: "group", in: "120363000000000021@g.us", want: user("120363000000000021", 0, node.ServerGroup)},
		{name: "legacy group", in: "40712345678-1600000000@g.us", want: user("40712345678-1600000000", 0, node.ServerGroup)},
		{name: "status", in: "status@broadcast", want: user("status", 0, node.ServerBroadcast)},
		{name: "status in capitals", in: "STATUS@broadcast", want: user("STATUS", 0, node.ServerBroadcast)},
		{name: "location", in: "location@broadcast", want: user("location", 0, node.ServerBroadcast)},
		{name: "chat", in: "chat@broadcast", want: user("chat", 0, node.ServerBroadcast)},
		{name: "broadcast list", in: "1690000000@broadcast", want: user("1690000000", 0, node.ServerBroadcast)},
		{name: "newsletter", in: "120363111111111111@newsletter", want: user("120363111111111111", 0, node.ServerNewsletter)},
		{name: "bot", in: "13135550002@bot", want: user("13135550002", 0, node.ServerBot)},
		{name: "bot device zero", in: "13135550002:0@bot", want: user("13135550002", 0, node.ServerBot)},
	}
	for _, tt := range valid {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := node.ParseJID(tt.in)
			if err != nil || got != tt.want {
				t.Fatalf("ParseJID(%q) = %#v, %v; want %#v", tt.in, got, err, tt.want)
			}
		})
	}
	invalid := []struct{ name, in string }{
		{name: "no server", in: "40712345678"},
		{name: "empty", in: ""},
		{name: "unknown server", in: "40712345678@example.com"},
		{name: "server in capitals", in: "40712345678@C.US"},
		{name: "four digit phone", in: "1234@s.whatsapp.net"},
		{name: "phone too long", in: "123456789012345678901@s.whatsapp.net"},
		{name: "phone starting with 10", in: "10712345678@s.whatsapp.net"},
		{name: "phone with leading zero", in: "0712345678@s.whatsapp.net"},
		{name: "letters", in: "4071234567a@s.whatsapp.net"},
		{name: "server user with agent", in: "0.0@s.whatsapp.net"},
		{name: "server user with device", in: "0:1@s.whatsapp.net"},
		{name: "three digit device", in: "40712345678:100@s.whatsapp.net"},
		{name: "empty device", in: "40712345678:@s.whatsapp.net"},
		{name: "device with letters", in: "40712345678:a@s.whatsapp.net"},
		{name: "agent other than zero", in: "40712345678.1@s.whatsapp.net"},
		{name: "lid too long", in: "1234567890123456@lid"},
		{name: "lid with leading zero", in: "0123@lid"},
		{name: "hosted without device", in: "40712345678@hosted"},
		{name: "hosted with another device", in: "40712345678:98@hosted"},
		{name: "hosted lid too long", in: "1234567890123456:99@hosted.lid"},
		{name: "legacy group with short timestamp", in: "40712345678-160000000@g.us"},
		{name: "legacy group with long timestamp", in: "40712345678-16000000000@g.us"},
		{name: "legacy group timestamp with leading zero", in: "40712345678-0600000000@g.us"},
		{name: "legacy group creator starting with 10", in: "10712345678-1600000000@g.us"},
		{name: "group id too long", in: "123456789012345678901@g.us"},
		{name: "broadcast word", in: "chats@broadcast"},
		{name: "newsletter with letters", in: "abc@newsletter"},
		{name: "bot with a device", in: "13135550002:1@bot"},
		{name: "call", in: "abc123def456abc123ff@call"},
		{name: "two servers", in: "a@b@s.whatsapp.net"},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got, err := node.ParseJID(tt.in); !errors.Is(err, node.ErrJID) {
				t.Fatalf("ParseJID(%q) = %#v, %v; want %v", tt.in, got, err, node.ErrJID)
			}
		})
	}
}

func FuzzParsedJIDsRoundTrip(f *testing.F) {
	for _, seed := range []string{"40712345678:7@s.whatsapp.net", "0@c.us", "88123:99@hosted.lid", "40712345678-1600000000@g.us", "STATUS@broadcast", "13135550002:0@bot", "a@b@c", "@", ""} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		j, err := node.ParseJID(s)
		if err != nil {
			return
		}
		again, err := node.ParseJID(j.String())
		if err != nil || again != j {
			t.Fatalf("ParseJID(%q) = %#v, whose String %q parses to %#v, %v", s, j, j.String(), again, err)
		}
	})
}

func TestWithoutDevice(t *testing.T) {
	t.Parallel()
	j := node.JID{User: "40711111111", Device: 7, Server: node.ServerUser}
	if got := j.WithoutDevice(); got != (node.JID{User: "40711111111", Server: node.ServerUser}) {
		t.Fatalf("WithoutDevice() = %#v", got)
	}
	if j.Device != 7 {
		t.Fatal("WithoutDevice changed its receiver")
	}
}

func TestStatusBroadcast(t *testing.T) {
	t.Parallel()
	if got := node.StatusBroadcast().String(); got != "status@broadcast" {
		t.Fatalf("StatusBroadcast() = %s", got)
	}
	for _, tt := range []struct {
		jid  node.JID
		want bool
	}{
		{node.StatusBroadcast(), true},
		{node.JID{User: "STATUS", Server: node.ServerBroadcast}, true},
		{node.JID{User: "1790000000", Server: node.ServerBroadcast}, false},
		{node.JID{User: "status", Server: node.ServerUser}, false},
	} {
		if got := tt.jid.IsStatus(); got != tt.want {
			t.Errorf("%s.IsStatus() = %v", tt.jid, got)
		}
	}
}
