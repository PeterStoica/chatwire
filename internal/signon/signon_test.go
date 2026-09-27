package signon_test

import (
	"bytes"
	"encoding/hex"
	"errors"
	"math/rand/v2"
	"os"
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"

	"github.com/PeterStoica/chatwire/internal/device"
	"github.com/PeterStoica/chatwire/internal/signon"
)

func TestRegistrationMatchesGoldenProvenByDifferential(t *testing.T) {
	raw, err := os.ReadFile("testdata/registration-seed-1.hex")
	if err != nil {
		t.Fatal(err)
	}
	want, err := hex.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	identity, err := device.New(rand.NewChaCha8([32]byte{1}))
	if err != nil {
		t.Fatal(err)
	}
	version := signon.Version{Primary: 2, Secondary: 3000, Tertiary: 1048570357}
	if got := signon.EncodeRegistration(version, identity.Registration(device.Props())); !bytes.Equal(got, want) {
		t.Fatalf("registration payload changed\n got %x\nwant %x", got, want)
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		text string
		want signon.Version
		ok   bool
	}{
		{text: "2.3000.1048570357", want: signon.Version{Primary: 2, Secondary: 3000, Tertiary: 1048570357}, ok: true},
		{text: "0.0.4294967295", want: signon.Version{Tertiary: 4294967295}, ok: true},
		{text: "2.3000"},
		{text: "2.3000.1.4"},
		{text: "2.x.1"},
		{text: "2.3000.4294967296"},
	}
	for _, tt := range tests {
		got, err := signon.ParseVersion(tt.text)
		if tt.ok != (err == nil) || got != tt.want {
			t.Errorf("ParseVersion(%q) = %+v, %v", tt.text, got, err)
		}
		if tt.ok && got.String() != tt.text {
			t.Errorf("%+v.String() = %q, want %q", got, got.String(), tt.text)
		}
	}
}

func TestHandshakeMessagesRoundTrip(t *testing.T) {
	ephemeral := bytes.Repeat([]byte{1}, 32)
	got, err := signon.ParseClientHello(signon.EncodeClientHello(ephemeral))
	if err != nil || !bytes.Equal(got, ephemeral) {
		t.Fatalf("client hello = %x, %v", got, err)
	}
	hello := signon.ServerHello{Ephemeral: []byte{1}, Static: []byte{2}, Payload: []byte{3}}
	if parsed, err := signon.ParseServerHello(signon.EncodeServerHello(hello)); err != nil || !equalHello(parsed, hello) {
		t.Fatalf("server hello = %+v, %v", parsed, err)
	}
	finish := signon.ClientFinish{Static: []byte{4}, Payload: []byte{5}}
	if parsed, err := signon.ParseClientFinish(signon.EncodeClientFinish(finish)); err != nil || !bytes.Equal(parsed.Static, finish.Static) || !bytes.Equal(parsed.Payload, finish.Payload) {
		t.Fatalf("client finish = %+v, %v", parsed, err)
	}
}

func equalHello(a, b signon.ServerHello) bool {
	return bytes.Equal(a.Ephemeral, b.Ephemeral) && bytes.Equal(a.Static, b.Static) && bytes.Equal(a.Payload, b.Payload)
}

func TestCertificateRoundTrip(t *testing.T) {
	details := signon.CertificateDetails{Serial: 7, IssuerSerial: 1<<32 - 1, Key: []byte{9, 9}, NotBefore: 1, NotAfter: 1<<64 - 1}
	parsed, err := signon.ParseCertificateDetails(signon.EncodeCertificateDetails(details))
	if err != nil || parsed.Serial != 7 || parsed.IssuerSerial != 1<<32-1 || !bytes.Equal(parsed.Key, details.Key) || parsed.NotBefore != 1 || parsed.NotAfter != 1<<64-1 {
		t.Fatalf("details = %+v, %v", parsed, err)
	}
	chain := signon.CertChain{Leaf: signon.Certificate{Details: []byte{1}, Signature: []byte{2}}, Intermediate: signon.Certificate{Details: []byte{3}, Signature: []byte{4}}}
	got, err := signon.ParseCertChain(signon.EncodeCertChain(chain))
	if err != nil || !bytes.Equal(got.Leaf.Details, []byte{1}) || !bytes.Equal(got.Leaf.Signature, []byte{2}) ||
		!bytes.Equal(got.Intermediate.Details, []byte{3}) || !bytes.Equal(got.Intermediate.Signature, []byte{4}) {
		t.Fatalf("chain = %+v, %v", got, err)
	}
}

func TestParseRejectsMalformed(t *testing.T) {
	overflow := protowire.AppendVarint(protowire.AppendTag(nil, 1, protowire.VarintType), 1<<32)
	tests := []struct {
		name  string
		parse func() error
	}{
		{name: "truncated tag", parse: func() error { _, err := signon.ParseServerHello([]byte{0x80}); return err }},
		{name: "truncated bytes", parse: func() error { _, err := signon.ParseServerHello([]byte{0x1a, 0x05, 0x01}); return err }},
		{name: "truncated varint", parse: func() error { _, err := signon.ParseCertificateDetails([]byte{0x08, 0x80}); return err }},
		{name: "truncated fixed field", parse: func() error { _, err := signon.ParseCertChain([]byte{0x0d, 0x01}); return err }},
		{name: "serial overflows uint32", parse: func() error { _, err := signon.ParseCertificateDetails(overflow); return err }},
		{name: "issuer serial overflows uint32", parse: func() error {
			_, err := signon.ParseCertificateDetails(protowire.AppendVarint(protowire.AppendTag(nil, 2, protowire.VarintType), 1<<32))
			return err
		}},
		{name: "server hello without payload", parse: func() error {
			_, err := signon.ParseServerHello(signon.EncodeServerHello(signon.ServerHello{Ephemeral: []byte{1}, Static: []byte{2}}))
			return err
		}},
		{name: "client hello without ephemeral", parse: func() error { _, err := signon.ParseClientHello([]byte{0x12, 0x00}); return err }},
		{name: "client finish without static", parse: func() error {
			_, err := signon.ParseClientFinish(signon.EncodeClientFinish(signon.ClientFinish{Payload: []byte{1}}))
			return err
		}},
		{name: "broken leaf certificate", parse: func() error { _, err := signon.ParseCertChain([]byte{0x0a, 0x01, 0x80}); return err }},
		{name: "broken client hello", parse: func() error { _, err := signon.ParseClientHello([]byte{0x12, 0x01, 0x80}); return err }},
		{name: "broken client finish", parse: func() error { _, err := signon.ParseClientFinish([]byte{0x22, 0x01, 0x80}); return err }},
		{name: "broken server hello body", parse: func() error { _, err := signon.ParseServerHello([]byte{0x1a, 0x01, 0x80}); return err }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.parse(); !errors.Is(err, signon.ErrMalformed) {
				t.Fatalf("error = %v, want %v", err, signon.ErrMalformed)
			}
		})
	}
}

func TestParseSkipsUnknownFields(t *testing.T) {
	unknown := protowire.AppendFixed32(protowire.AppendTag(nil, 9, protowire.Fixed32Type), 7)
	hello := slices.Concat(unknown, signon.EncodeServerHello(signon.ServerHello{Ephemeral: []byte{1}, Static: []byte{2}, Payload: []byte{3}}))
	if _, err := signon.ParseServerHello(hello); err != nil {
		t.Fatalf("ParseServerHello with an unknown fixed32 field: %v", err)
	}
}
