package signal_test

import (
	"crypto/rand"
	"errors"
	"fmt"
	"testing"
	"testing/cryptotest"

	"google.golang.org/protobuf/proto"

	"github.com/PeterStoica/chatwire/internal/signal"
	"github.com/PeterStoica/chatwire/internal/testkit/signaltest"
	"github.com/PeterStoica/chatwire/internal/wire"
)

func TestGroupTranscriptsMatchKnownAnswers(t *testing.T) {
	golden, err := signaltest.LoadGolden("testdata/group-transcripts.json", signaltest.GroupRounds)
	if err != nil {
		t.Fatal(err)
	}
	for name, factory := range map[string]signaltest.MemberFactory{"in memory": signaltest.NewOurMember, "stored after every step": signaltest.NewPersistingOurMember} {
		ours := []signaltest.MemberFactory{factory, factory, factory}
		for seed, want := range golden.Digests {
			t.Run(fmt.Sprintf("%s/%d", name, seed), func(t *testing.T) {
				cryptotest.SetGlobalRandom(t, uint64(seed))
				result, err := signaltest.GroupConverse(uint64(seed), ours)
				if err != nil {
					t.Fatal(err)
				}
				if got := result.Digest(); got != want {
					t.Fatalf("group transcript digest %s, the known answer is %s", got, want)
				}
			})
		}
	}
}

type groupPair struct {
	t        *testing.T
	sender   *signal.SenderKey
	receiver *signal.SenderKeys
}

func newGroupPair(t *testing.T) *groupPair {
	t.Helper()
	sender, err := signal.NewSenderKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	g := &groupPair{t: t, sender: sender, receiver: &signal.SenderKeys{}}
	g.process(g.distribution())
	return g
}

func (g *groupPair) distribution() []byte {
	g.t.Helper()
	raw, err := g.sender.Distribution()
	if err != nil {
		g.t.Fatal(err)
	}
	return raw
}

func (g *groupPair) process(raw []byte) {
	g.t.Helper()
	if err := g.receiver.Process(raw); err != nil {
		g.t.Fatal(err)
	}
}

func (g *groupPair) send(text string) []byte {
	g.t.Helper()
	raw, err := g.sender.Encrypt(rand.Reader, []byte(text))
	if err != nil {
		g.t.Fatal(err)
	}
	return raw
}

func (g *groupPair) deliver(raw []byte, want string) {
	g.t.Helper()
	plaintext, err := g.receiver.Decrypt(raw)
	if err != nil || string(plaintext) != want {
		g.t.Fatalf("decrypted %q, %v; want %q", plaintext, err, want)
	}
}

func (g *groupPair) reject(raw []byte, want error) {
	g.t.Helper()
	if _, err := g.receiver.Decrypt(raw); !errors.Is(err, want) {
		g.t.Fatalf("Decrypt = %v, want %v", err, want)
	}
}

func TestGroupPlaintextLengths(t *testing.T) {
	g := newGroupPair(t)
	for n := range 41 {
		text := string(make([]byte, n))
		g.deliver(g.send(text), text)
	}
}

func TestGroupTamperingIsAlwaysRefusedAndNeverMovesTheState(t *testing.T) {
	g := newGroupPair(t)
	target := g.send("target")
	for i := range len(target) * 8 {
		tampered := append([]byte(nil), target...)
		tampered[i/8] ^= 1 << (i % 8)
		if _, err := g.receiver.Decrypt(tampered); err == nil {
			t.Fatalf("bit %d flipped and the message still decrypted", i)
		}
	}
	for n := range target {
		if _, err := g.receiver.Decrypt(target[:n]); err == nil {
			t.Fatalf("truncated to %d bytes and still decrypted", n)
		}
	}
	g.deliver(target, "target")
	g.reject(target, signal.ErrDuplicate)
}

func TestGroupRejectionClasses(t *testing.T) {
	g := newGroupPair(t)
	target := g.send("target")
	body := target[:len(target)-64]
	resign := func(message *wire.SenderKeyMessage) []byte {
		encoded, err := proto.Marshal(message)
		if err != nil {
			t.Fatal(err)
		}
		return append(append([]byte{0x33}, encoded...), make([]byte, 64)...)
	}
	for _, tt := range []struct {
		name string
		raw  []byte
		want error
	}{
		{"signature only", target[len(target)-64:], signal.ErrMalformed},
		{"version 2", append([]byte{0x23}, target[1:]...), signal.ErrVersion},
		{"not protobuf", append(append([]byte{0x33}, 0xff, 0xff), target[len(target)-64:]...), signal.ErrMalformed},
		{"no iteration", resign(&wire.SenderKeyMessage{Id: new(uint32(1)), Ciphertext: []byte{1}}), signal.ErrMalformed},
		{"no ciphertext", resign(&wire.SenderKeyMessage{Id: new(uint32(1)), Iteration: new(uint32(0))}), signal.ErrMalformed},
		{"unknown key id", resign(&wire.SenderKeyMessage{Id: new(uint32(1)), Iteration: new(uint32(0)), Ciphertext: []byte{1}}), signal.ErrNoSenderKey},
		{"forged signature", append(append([]byte(nil), body...), make([]byte, 64)...), signal.ErrGroupSignature},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := g.receiver.Decrypt(tt.raw); !errors.Is(err, tt.want) {
				t.Fatalf("Decrypt = %v, want %v", err, tt.want)
			}
		})
	}
	g.deliver(target, "target")
}

func TestGroupDistributionRejectionClasses(t *testing.T) {
	g := newGroupPair(t)
	valid := g.distribution()
	encode := func(message *wire.SenderKeyDistributionMessage) []byte {
		encoded, err := proto.Marshal(message)
		if err != nil {
			t.Fatal(err)
		}
		return append([]byte{0x33}, encoded...)
	}
	var parsed wire.SenderKeyDistributionMessage
	if err := proto.Unmarshal(valid[1:], &parsed); err != nil {
		t.Fatal(err)
	}
	edit := func(change func(*wire.SenderKeyDistributionMessage)) []byte {
		copied := proto.CloneOf(&parsed)
		change(copied)
		return encode(copied)
	}
	for _, tt := range []struct {
		name string
		raw  []byte
		want error
	}{
		{"empty", nil, signal.ErrVersion},
		{"version 2", append([]byte{0x23}, valid[1:]...), signal.ErrVersion},
		{"not protobuf", []byte{0x33, 0xff, 0xff}, signal.ErrMalformed},
		{"untyped signing key", edit(func(m *wire.SenderKeyDistributionMessage) { m.SigningKey = m.SigningKey[1:] }), signal.ErrMalformed},
		{"no id", edit(func(m *wire.SenderKeyDistributionMessage) { m.Id = nil }), signal.ErrMalformed},
		{"no iteration", edit(func(m *wire.SenderKeyDistributionMessage) { m.Iteration = nil }), signal.ErrMalformed},
		{"short chain key", edit(func(m *wire.SenderKeyDistributionMessage) { m.ChainKey = m.ChainKey[1:] }), signal.ErrMalformed},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := g.receiver.Process(tt.raw); !errors.Is(err, tt.want) {
				t.Fatalf("Process = %v, want %v", err, tt.want)
			}
		})
	}
	g.deliver(g.send("still works"), "still works")
}

func TestRepeatedDistributionKeepsProgress(t *testing.T) {
	for _, fresher := range []bool{false, true} {
		t.Run(fmt.Sprintf("fresher=%v", fresher), func(t *testing.T) {
			g := newGroupPair(t)
			original := g.distribution()
			sent := [][]byte{g.send("0"), g.send("1"), g.send("2")}
			g.deliver(sent[0], "0")
			g.deliver(sent[2], "2")
			repeated := original
			if fresher {
				repeated = g.distribution()
			}
			g.process(repeated)
			g.deliver(sent[1], "1")
			g.reject(sent[0], signal.ErrDuplicate)
			g.reject(sent[2], signal.ErrDuplicate)
		})
	}
}

func TestLateJoinerStartsAtTheAdvertisedIteration(t *testing.T) {
	g := newGroupPair(t)
	before := [][]byte{g.send("0"), g.send("1"), g.send("2")}
	joiner := &signal.SenderKeys{}
	if err := joiner.Process(g.distribution()); err != nil {
		t.Fatal(err)
	}
	after := g.send("3")
	if plaintext, err := joiner.Decrypt(after); err != nil || string(plaintext) != "3" {
		t.Fatalf("late joiner decrypted %q, %v", plaintext, err)
	}
	for _, raw := range before {
		if _, err := joiner.Decrypt(raw); !errors.Is(err, signal.ErrDuplicate) {
			t.Fatalf("late joiner read a message from before it joined: %v", err)
		}
	}
}

func TestSameKeyIDWithAnotherSigningKeyReplacesTheState(t *testing.T) {
	g := newGroupPair(t)
	before := g.send("before")
	impostor, err := signal.NewSenderKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := impostor.Distribution()
	if err != nil {
		t.Fatal(err)
	}
	var theirs, ours wire.SenderKeyDistributionMessage
	if err := proto.Unmarshal(raw[1:], &theirs); err != nil {
		t.Fatal(err)
	}
	if err := proto.Unmarshal(g.distribution()[1:], &ours); err != nil {
		t.Fatal(err)
	}
	theirs.Id = ours.Id
	encoded, err := proto.Marshal(&theirs)
	if err != nil {
		t.Fatal(err)
	}
	g.process(append([]byte{0x33}, encoded...))
	g.reject(before, signal.ErrGroupSignature)
}

func TestSenderKeyStateLimit(t *testing.T) {
	for _, tt := range []struct {
		rotations int
		want      error
	}{{4, nil}, {5, signal.ErrNoSenderKey}} {
		t.Run(fmt.Sprint(tt.rotations), func(t *testing.T) {
			g := newGroupPair(t)
			late := g.send("late")
			for range tt.rotations {
				sender, err := signal.NewSenderKey(rand.Reader)
				if err != nil {
					t.Fatal(err)
				}
				g.sender = sender
				g.process(g.distribution())
			}
			if _, err := g.receiver.Decrypt(late); !errors.Is(err, tt.want) {
				t.Fatalf("after %d rotations: %v, want %v", tt.rotations, err, tt.want)
			}
		})
	}
}

func TestGroupFutureAndSkippedKeyLimits(t *testing.T) {
	g := newGroupPair(t)
	chain := make([][]byte, 2003)
	for i := range chain {
		chain[i] = g.send(fmt.Sprint(i))
	}
	g.reject(chain[2001], signal.ErrTooFarAhead)
	g.deliver(chain[2000], "2000")
	g.deliver(chain[2002], "2002")
	g.reject(chain[0], signal.ErrDuplicate)
	g.deliver(chain[1], "1")
	g.deliver(chain[2001], "2001")
}

func TestDecryptWithoutAnyState(t *testing.T) {
	g := newGroupPair(t)
	var empty signal.SenderKeys
	if _, err := empty.Decrypt(g.send("x")); !errors.Is(err, signal.ErrNoSenderKey) {
		t.Fatalf("Decrypt with no states = %v", err)
	}
}

func FuzzGroupDecryptNeverMovesTheState(f *testing.F) {
	sender, err := signal.NewSenderKey(rand.Reader)
	if err != nil {
		f.Fatal(err)
	}
	distribution, err := sender.Distribution()
	if err != nil {
		f.Fatal(err)
	}
	first, err := sender.Encrypt(rand.Reader, []byte("first"))
	if err != nil {
		f.Fatal(err)
	}
	next, err := sender.Encrypt(rand.Reader, []byte("next"))
	if err != nil {
		f.Fatal(err)
	}
	f.Add(first)
	f.Add(next)
	f.Add(distribution)
	f.Fuzz(func(t *testing.T, raw []byte) {
		receiver := &signal.SenderKeys{}
		if err := receiver.Process(distribution); err != nil {
			t.Fatal(err)
		}
		if _, err := receiver.Decrypt(raw); err != nil {
			if plaintext, err := receiver.Decrypt(next); err != nil || string(plaintext) != "next" {
				t.Fatalf("a rejected input moved the state: %q, %v", plaintext, err)
			}
		}
	})
}
