package signal_test

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"testing"
	"testing/cryptotest"

	"github.com/PeterStoica/chatwire/internal/curve"
	"github.com/PeterStoica/chatwire/internal/signal"
	"github.com/PeterStoica/chatwire/internal/signal/signaltest"
)

const crossSessionSeeds = 150

func profiles(t *testing.T, seed uint64) [2]signaltest.Profile {
	t.Helper()
	p, err := signaltest.Profiles(seed)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestTranscriptsMatchKnownAnswers(t *testing.T) {
	golden, err := signaltest.LoadGolden("testdata/transcripts.json", signaltest.Rounds)
	if err != nil {
		t.Fatal(err)
	}
	for name, factory := range map[string]signaltest.Factory{"in memory": signaltest.NewOurs, "stored after every step": signaltest.NewPersistingOurs} {
		for seed, want := range golden.Digests {
			t.Run(fmt.Sprintf("%s/%d", name, seed), func(t *testing.T) {
				cryptotest.SetGlobalRandom(t, uint64(seed))
				result, err := signaltest.Converse(uint64(seed), false, profiles(t, uint64(seed)), [2]signaltest.Factory{factory, factory})
				if err != nil {
					t.Fatal(err)
				}
				if got := result.Digest(); got != want {
					t.Fatalf("transcript digest %s, the known answer is %s", got, want)
				}
			})
		}
	}
}

func TestCrossSessionInterleavingsAlwaysDecrypt(t *testing.T) {
	crossed := 0
	for seed := range uint64(crossSessionSeeds) {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			cryptotest.SetGlobalRandom(t, seed)
			result, err := signaltest.Converse(seed, true, profiles(t, seed), [2]signaltest.Factory{signaltest.NewOurs, signaltest.NewOurs})
			if err != nil {
				t.Fatal(err)
			}
			if lost := result.Undecryptable(); len(lost) > 0 {
				t.Fatalf("undecryptable: %v", lost)
			}
			crossed += result.Stats["delivered across a re-initiation"]
		})
	}
	if crossed == 0 {
		t.Fatal("no message was delivered across a re-initiation")
	}
}

type pair struct {
	t          *testing.T
	p          [2]signaltest.Profile
	alice, bob signaltest.Party
}

func newPair(t *testing.T, oneTime *uint32) *pair {
	t.Helper()
	p := profiles(t, 1)
	c := &pair{t: t, p: p, alice: signaltest.NewOurs(p[0], p[1]), bob: signaltest.NewOurs(p[1], p[0])}
	if err := c.alice.Initiate(p[1].Bundle(oneTime)); err != nil {
		t.Fatal(err)
	}
	c.deliver(c.bob, c.send(c.alice, "hello"))
	return c
}

func (c *pair) send(from signaltest.Party, text string) signaltest.Wire {
	c.t.Helper()
	w, err := from.Send(text)
	if err != nil {
		c.t.Fatal(err)
	}
	return w
}

func (c *pair) deliver(to signaltest.Party, w signaltest.Wire) {
	c.t.Helper()
	plaintext, err := to.Receive(w)
	if err != nil || string(plaintext) != w.Text {
		c.t.Fatalf("received %q, %v; want %q", plaintext, err, w.Text)
	}
}

func (c *pair) reject(to signaltest.Party, w signaltest.Wire, want error) {
	c.t.Helper()
	if _, err := to.Receive(w); !errors.Is(err, want) {
		c.t.Fatalf("receiving %s: %v, want %v", w.Text, err, want)
	}
}

func (c *pair) pingPong() {
	c.t.Helper()
	c.deliver(c.alice, c.send(c.bob, "ping"))
	c.deliver(c.bob, c.send(c.alice, "pong"))
}

func TestForgedSignedPreKeyIsRefused(t *testing.T) {
	p := profiles(t, 2)
	bundle := p[1].Bundle(nil)
	bundle.SignedPreKeySignature[10] ^= 1
	if err := signaltest.NewOurs(p[0], p[1]).Initiate(bundle); !errors.Is(err, signal.ErrSignature) {
		t.Fatalf("Initiate with a forged signature: %v, want %v", err, signal.ErrSignature)
	}
	bundle = p[1].Bundle(nil)
	bundle.SignedPreKey = p[1].OneTime[100].Public()
	if err := signaltest.NewOurs(p[0], p[1]).Initiate(bundle); !errors.Is(err, signal.ErrSignature) {
		t.Fatalf("Initiate with a swapped signed prekey: %v, want %v", err, signal.ErrSignature)
	}
}

func TestWithoutASession(t *testing.T) {
	var s *signal.Session
	if _, err := s.Encrypt([]byte("x")); !errors.Is(err, signal.ErrNoSession) {
		t.Fatalf("Encrypt without a session: %v", err)
	}
	c := newPair(t, nil)
	c.deliver(c.alice, c.send(c.bob, "reply"))
	c.reject(signaltest.NewOurs(c.p[1], c.p[0]), c.send(c.alice, "regular"), signal.ErrNoSession)
}

func TestTamperingNeverMovesTheSession(t *testing.T) {
	for _, prekey := range []bool{true, false} {
		t.Run(fmt.Sprintf("prekey=%v", prekey), func(t *testing.T) {
			target := func() (*pair, signaltest.Wire) {
				c := newPair(t, new(uint32(100)))
				if !prekey {
					c.pingPong()
				}
				w := c.send(c.alice, "target")
				if w.PreKey != prekey {
					t.Fatalf("message is prekey=%v", w.PreKey)
				}
				return c, w
			}
			_, original := target()
			accepted := 0
			for i := range 2 * len(original.Bytes) {
				c, w := target()
				tampered := w
				if i < len(w.Bytes) {
					tampered.Bytes = append([]byte(nil), w.Bytes...)
					tampered.Bytes[i] ^= 1 << (i % 8)
				} else {
					tampered.Bytes = w.Bytes[:i-len(w.Bytes)]
				}
				plaintext, err := c.bob.Receive(tampered)
				if err == nil {
					if string(plaintext) != w.Text {
						t.Fatalf("edit %d decrypted to %q", i, plaintext)
					}
					accepted++
					continue
				}
				c.deliver(c.bob, w)
				c.pingPong()
			}
			if !prekey && accepted > 0 {
				t.Fatalf("%d edits of an authenticated message were accepted", accepted)
			}
		})
	}
}

func TestRejectionClasses(t *testing.T) {
	c := newPair(t, nil)
	c.pingPong()
	w := c.send(c.alice, "target")
	variant := func(edit func([]byte) []byte) signaltest.Wire {
		v := w
		v.Bytes = edit(append([]byte(nil), w.Bytes...))
		return v
	}
	c.reject(c.bob, variant(func(b []byte) []byte { b[0] = 0x23; return b }), signal.ErrVersion)
	c.reject(c.bob, variant(func(b []byte) []byte { b[len(b)-1] ^= 1; return b }), signal.ErrMAC)
	c.reject(c.bob, variant(func(b []byte) []byte { return b[:8] }), signal.ErrMalformed)
	c.reject(c.bob, variant(func(b []byte) []byte { return append(b[:1], make([]byte, 8)...) }), signal.ErrMalformed)
	c.reject(c.bob, variant(func(b []byte) []byte { return append([]byte{b[0], 0x0a, 0x01, 0x05}, b[len(b)-8:]...) }), signal.ErrMalformed)
	c.deliver(c.bob, w)
	c.reject(c.bob, w, signal.ErrDuplicate)
}

func TestPlaintextLengths(t *testing.T) {
	c := newPair(t, nil)
	for n := range 41 {
		text := string(make([]byte, n))
		c.deliver(c.bob, c.send(c.alice, text))
		c.deliver(c.alice, c.send(c.bob, text))
	}
}

func TestReplayAfterAnArchivedStateIsPromoted(t *testing.T) {
	for _, depth := range []int{1, 2} {
		t.Run(fmt.Sprint(depth), func(t *testing.T) {
			c := newPair(t, nil)
			late := c.send(c.alice, "late")
			for i := range depth {
				if err := c.alice.Initiate(c.p[1].Bundle(nil)); err != nil {
					t.Fatal(err)
				}
				c.deliver(c.bob, c.send(c.alice, fmt.Sprint("session ", i)))
			}
			c.deliver(c.bob, late)
			if _, err := c.bob.Receive(late); err == nil {
				t.Fatal("a replayed message decrypted twice")
			}
		})
	}
}

func TestInitiatorKeepsTheOldSessionForLateReplies(t *testing.T) {
	c := newPair(t, nil)
	late := c.send(c.bob, "reply on the first session")
	if err := c.alice.Initiate(c.p[1].Bundle(nil)); err != nil {
		t.Fatal(err)
	}
	c.deliver(c.bob, c.send(c.alice, "second session"))
	c.deliver(c.alice, late)
}

func TestForgeriesOfSkippedMessagesLeaveTheirKeys(t *testing.T) {
	c := newPair(t, nil)
	c.pingPong()
	sent := make([]signaltest.Wire, 4)
	for i := range sent {
		sent[i] = c.send(c.alice, fmt.Sprint(i))
	}
	c.deliver(c.bob, sent[3])
	for _, i := range []int{1, 0, 2} {
		forged := sent[i]
		forged.Bytes = bytes.Clone(sent[i].Bytes)
		forged.Bytes[len(forged.Bytes)-1] ^= 1
		c.reject(c.bob, forged, signal.ErrMAC)
	}
	for _, i := range []int{1, 0, 2} {
		c.deliver(c.bob, sent[i])
	}
}

func TestFutureAndSkippedKeyLimits(t *testing.T) {
	c := newPair(t, nil)
	c.pingPong()
	c.deliver(c.alice, c.send(c.bob, "switch alice to a fresh chain"))
	chain := make([]signaltest.Wire, 2004)
	for i := range chain {
		chain[i] = c.send(c.alice, fmt.Sprint(i))
	}
	c.reject(c.bob, chain[2001], signal.ErrTooFarAhead)
	c.deliver(c.bob, chain[2000])
	c.deliver(c.bob, chain[2001])
	c.deliver(c.bob, chain[2003])
	c.reject(c.bob, chain[0], signal.ErrDuplicate)
	c.deliver(c.bob, chain[1])
	c.deliver(c.bob, chain[1999])
	c.deliver(c.bob, chain[2002])
	c.reject(c.bob, chain[2002], signal.ErrDuplicate)
}

func TestReceiverChainLimit(t *testing.T) {
	for _, tt := range []struct {
		ratchets  int
		decrypted bool
	}{{4, true}, {5, false}} {
		t.Run(fmt.Sprint(tt.ratchets), func(t *testing.T) {
			c := newPair(t, nil)
			late := c.send(c.alice, "late")
			for range tt.ratchets {
				c.pingPong()
			}
			_, err := c.bob.Receive(late)
			if decrypted := err == nil; decrypted != tt.decrypted {
				t.Fatalf("after %d ratchets decrypted=%v (%v)", tt.ratchets, decrypted, err)
			}
		})
	}
}

func TestArchiveLimit(t *testing.T) {
	for _, tt := range []struct {
		reinitiations int
		want          error
	}{{40, nil}, {41, signal.ErrUnknownPreKey}} {
		t.Run(fmt.Sprint(tt.reinitiations), func(t *testing.T) {
			c := newPair(t, new(uint32(100)))
			late := c.send(c.alice, "late")
			for i := range tt.reinitiations {
				if err := c.alice.Initiate(c.p[1].Bundle(nil)); err != nil {
					t.Fatal(err)
				}
				c.deliver(c.bob, c.send(c.alice, fmt.Sprint("session ", i)))
			}
			if _, err := c.bob.Receive(late); !errors.Is(err, tt.want) {
				t.Fatalf("after %d re-initiations: %v, want %v", tt.reinitiations, err, tt.want)
			}
		})
	}
}

type preKeys struct {
	p       signaltest.Profile
	signed  uint32
	oneTime map[uint32]bool
}

func (k preKeys) PreKey(id uint32) (curve.KeyPair, bool) {
	pair, ok := k.p.OneTime[id]
	return pair, ok && k.oneTime[id]
}

func (k preKeys) SignedPreKey(id uint32) (curve.KeyPair, bool) {
	return k.p.Signed, id == k.signed
}

func TestRespondReportsTheConsumedOneTimePreKeyOnce(t *testing.T) {
	p := profiles(t, 3)
	alice := signaltest.NewOurs(p[0], p[1])
	if err := alice.Initiate(p[1].Bundle(new(uint32(101)))); err != nil {
		t.Fatal(err)
	}
	first, err := alice.Send("first")
	if err != nil {
		t.Fatal(err)
	}
	second, err := alice.Send("second")
	if err != nil {
		t.Fatal(err)
	}
	local := signal.Local{Identity: p[1].Identity, RegistrationID: p[1].Registration}
	keys := preKeys{p: p[1], signed: signaltest.SignedPreKeyID, oneTime: map[uint32]bool{101: true}}
	session, plaintext, used, err := signal.Respond(rand.Reader, local, keys, nil, first.Bytes)
	if err != nil || string(plaintext) != "first" || used == nil || *used != 101 {
		t.Fatalf("first prekey message: %q, used %v, %v", plaintext, used, err)
	}
	delete(keys.oneTime, 101)
	session, plaintext, used, err = signal.Respond(rand.Reader, local, keys, session, second.Bytes)
	if err != nil || string(plaintext) != "second" || used != nil || session == nil {
		t.Fatalf("second prekey message of the same session: %q, used %v, %v", plaintext, used, err)
	}
	if _, _, _, err := signal.Respond(rand.Reader, local, keys, nil, second.Bytes); !errors.Is(err, signal.ErrUnknownPreKey) {
		t.Fatalf("replay after the one-time prekey is gone: %v, want %v", err, signal.ErrUnknownPreKey)
	}
	unsigned := preKeys{p: p[1], signed: signaltest.SignedPreKeyID + 1, oneTime: map[uint32]bool{101: true}}
	if _, _, _, err := signal.Respond(rand.Reader, local, unsigned, nil, first.Bytes); !errors.Is(err, signal.ErrUnknownPreKey) {
		t.Fatalf("unknown signed prekey: %v, want %v", err, signal.ErrUnknownPreKey)
	}
}

func TestRespondRejectsMalformedPreKeyMessages(t *testing.T) {
	p := profiles(t, 4)
	local := signal.Local{Identity: p[1].Identity, RegistrationID: p[1].Registration}
	keys := preKeys{p: p[1], signed: signaltest.SignedPreKeyID}
	for _, tt := range []struct {
		name string
		raw  []byte
		want error
	}{
		{"empty", nil, signal.ErrVersion},
		{"version only", []byte{0x33}, signal.ErrMalformed},
		{"version 2", []byte{0x22, 0x08, 0x01}, signal.ErrVersion},
		{"not protobuf", []byte{0x33, 0xff, 0xff, 0xff}, signal.ErrMalformed},
		{"short identity", []byte{0x33, 0x2a, 0x01, 0x05}, signal.ErrMalformed},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, _, err := signal.Respond(rand.Reader, local, keys, nil, tt.raw); !errors.Is(err, tt.want) {
				t.Fatalf("Respond(%x) = %v, want %v", tt.raw, err, tt.want)
			}
		})
	}
}

func FuzzReceiveNeverMovesTheSessionOnFailure(f *testing.F) {
	p, err := signaltest.Profiles(5)
	if err != nil {
		f.Fatal(err)
	}
	seedPair := func() (signaltest.Party, signaltest.Party, signaltest.Wire, signaltest.Wire) {
		alice, bob := signaltest.NewOurs(p[0], p[1]), signaltest.NewOurs(p[1], p[0])
		if err := alice.Initiate(p[1].Bundle(nil)); err != nil {
			f.Fatal(err)
		}
		opening, err := alice.Send("opening")
		if err != nil {
			f.Fatal(err)
		}
		if _, err := bob.Receive(opening); err != nil {
			f.Fatal(err)
		}
		next, err := alice.Send("next")
		if err != nil {
			f.Fatal(err)
		}
		return alice, bob, opening, next
	}
	_, _, opening, next := seedPair()
	f.Add(opening.Bytes, true)
	f.Add(next.Bytes, false)
	f.Add(next.Bytes, true)
	f.Fuzz(func(t *testing.T, raw []byte, prekey bool) {
		_, bob, _, next := seedPair()
		if _, err := bob.Receive(signaltest.Wire{PreKey: prekey, Bytes: raw}); err != nil {
			if plaintext, err := bob.Receive(next); err != nil || string(plaintext) != "next" {
				t.Fatalf("a rejected input moved the session: %q, %v", plaintext, err)
			}
		}
	})
}
