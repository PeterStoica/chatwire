package cert

import (
	"encoding/hex"
	"errors"
	"math"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/PeterStoica/chatwire/internal/curve"
	"github.com/PeterStoica/chatwire/internal/signon"
)

type fixture struct {
	random       *rand.ChaCha8
	authority    *Authority
	server       curve.KeyPair
	now          time.Time
	intermediate Details
	leaf         Details
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	random := rand.NewChaCha8([32]byte{9})
	authority, err := NewAuthority(random)
	if err != nil {
		t.Fatal(err)
	}
	server, err := curve.NewKeyPair(random)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_800_000_000, 0)
	return &fixture{
		random: random, authority: authority, server: server, now: now,
		intermediate: Details{Serial: intermediateSerial, IssuerSerial: rootSerial, Key: authority.intermediate.Public(), NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour)},
		leaf:         Details{Serial: leafSerial, IssuerSerial: intermediateSerial, Key: server.Public(), NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour)},
	}
}

func (f *fixture) chain(t *testing.T) []byte {
	t.Helper()
	intermediate, err := sign(f.random, f.authority.root, f.intermediate)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := sign(f.random, f.authority.intermediate, f.leaf)
	if err != nil {
		t.Fatal(err)
	}
	return signon.EncodeCertChain(signon.CertChain{Leaf: leaf, Intermediate: intermediate})
}

func TestVerify(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(f *fixture)
		now    func(f *fixture) time.Time
		valid  bool
	}{
		{name: "valid", valid: true},
		{name: "valid at not-before", now: func(f *fixture) time.Time { return f.leaf.NotBefore }, valid: true},
		{name: "valid at not-after", now: func(f *fixture) time.Time { return f.leaf.NotAfter }, valid: true},
		{name: "a second before", now: func(f *fixture) time.Time { return f.leaf.NotBefore.Add(-time.Second) }},
		{name: "a second after", now: func(f *fixture) time.Time { return f.leaf.NotAfter.Add(time.Second) }},
		{name: "intermediate issued by a non-root serial", mutate: func(f *fixture) { f.intermediate.IssuerSerial = 1 }},
		{name: "leaf issuer is not the intermediate", mutate: func(f *fixture) { f.leaf.IssuerSerial = intermediateSerial + 5 }},
		{name: "leaf for another key", mutate: func(f *fixture) { f.leaf.Key = f.authority.root.Public() }},
		{name: "leaf expired while intermediate valid", mutate: func(f *fixture) { f.leaf.NotAfter = f.now.Add(-time.Second) }},
		{name: "intermediate expired while leaf valid", mutate: func(f *fixture) { f.intermediate.NotAfter = f.now.Add(-time.Second) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			if tt.mutate != nil {
				tt.mutate(f)
			}
			now := f.now
			if tt.now != nil {
				now = tt.now(f)
			}
			err := Verify(f.chain(t), f.server.Public(), f.authority.Root(), now)
			if tt.valid != (err == nil) {
				t.Fatalf("Verify() = %v, valid %v", err, tt.valid)
			}
			if err != nil && !errors.Is(err, ErrRejected) {
				t.Fatalf("Verify() = %v, want %v", err, ErrRejected)
			}
		})
	}
}

func TestVerifyRejectsMalformedChains(t *testing.T) {
	f := newFixture(t)
	valid, err := signon.ParseCertChain(f.chain(t))
	if err != nil {
		t.Fatal(err)
	}
	shortSignature := valid
	shortSignature.Leaf.Signature = shortSignature.Leaf.Signature[:63]
	flipped := valid
	flipped.Intermediate.Signature = append([]byte(nil), valid.Intermediate.Signature...)
	flipped.Intermediate.Signature[0] ^= 1
	for name, raw := range map[string][]byte{
		"garbage":         {0xff, 0xff},
		"short signature": signon.EncodeCertChain(shortSignature),
		"bad signature":   signon.EncodeCertChain(flipped),
		"other root":      f.chain(t),
	} {
		root := f.authority.Root()
		if name == "other root" {
			root = f.server.Public()
		}
		if err := Verify(raw, f.server.Public(), root, f.now); !errors.Is(err, ErrRejected) {
			t.Errorf("%s: Verify() = %v, want %v", name, err, ErrRejected)
		}
	}
}

func TestParseDetailsRejectsBadValues(t *testing.T) {
	encode := func(key []byte, notBefore, notAfter uint64) []byte {
		return signon.EncodeCertificateDetails(signon.CertificateDetails{Key: key, NotBefore: notBefore, NotAfter: notAfter})
	}
	key := make([]byte, curve.KeySize)
	if _, err := parseDetails(encode(key, 0, math.MaxInt64)); err != nil {
		t.Fatalf("MaxInt64 not-after: %v", err)
	}
	for name, raw := range map[string][]byte{
		"short key":              encode(key[:31], 0, 1),
		"not-before past int64":  encode(key, math.MaxInt64+1, math.MaxInt64+1),
		"not-after past int64":   encode(key, 0, math.MaxInt64+1),
		"truncated wire details": {0x08},
	} {
		if _, err := parseDetails(raw); err == nil {
			t.Errorf("%s: parseDetails accepted it", name)
		}
	}
}

func TestIssueRejectsTimesBefore1970(t *testing.T) {
	f := newFixture(t)
	for _, validity := range []Validity{
		{NotBefore: time.Unix(-1, 0), NotAfter: f.now},
		{NotBefore: f.now, NotAfter: time.Unix(-1, 0)},
	} {
		if _, err := f.authority.Issue(f.random, f.server.Public(), validity); err == nil {
			t.Fatalf("Issue(%v) accepted a time before 1970", validity)
		}
	}
	if _, err := f.authority.Issue(f.random, f.server.Public(), Validity{NotBefore: time.Unix(0, 0), NotAfter: time.Unix(0, 0)}); err != nil {
		t.Fatalf("Issue at the epoch: %v", err)
	}
}

func TestIssuedChainVerifies(t *testing.T) {
	f := newFixture(t)
	chain, err := f.authority.Issue(f.random, f.server.Public(), Validity{NotBefore: f.now, NotAfter: f.now})
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(chain, f.server.Public(), f.authority.Root(), f.now); err != nil {
		t.Fatalf("Verify(issued chain) = %v", err)
	}
}

func TestWhatsAppRoot(t *testing.T) {
	root := WhatsAppRoot()
	if got := hex.EncodeToString(root[:]); got != "142375574d0a587166aae71ebe516437c4a28b73e3695c6ce1f7f9545da8ee6b" {
		t.Fatalf("WhatsAppRoot() = %s", got)
	}
}

func TestNewAuthorityFailsOnShortRandom(t *testing.T) {
	for _, size := range []int{0, 40} {
		if _, err := NewAuthority(&limited{remaining: size}); err == nil {
			t.Fatalf("NewAuthority with %d random bytes succeeded", size)
		}
	}
}

type limited struct{ remaining int }

func (l *limited) Read(p []byte) (int, error) {
	if l.remaining == 0 {
		return 0, errors.New("exhausted")
	}
	n := min(len(p), l.remaining)
	l.remaining -= n
	return n, nil
}
