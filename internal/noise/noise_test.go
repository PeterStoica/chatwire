package noise_test

import (
	"bytes"
	"math/rand/v2"
	"testing"

	"github.com/PeterStoica/chatwire/internal/curve"
	"github.com/PeterStoica/chatwire/internal/noise"
)

type pair struct {
	initiator *noise.Initiator
	responder *noise.Responder
}

func newPair(t *testing.T, initiatorVariant, responderVariant noise.Variant) pair {
	t.Helper()
	random := rand.NewChaCha8([32]byte{3})
	prologue := []byte("prologue")
	initiator, err := noise.NewInitiator(initiatorVariant, prologue, keyPair(t, bytes.Repeat([]byte{1}, 32)), random)
	if err != nil {
		t.Fatal(err)
	}
	responder, err := noise.NewResponder(responderVariant, prologue, keyPair(t, bytes.Repeat([]byte{2}, 32)), random)
	if err != nil {
		t.Fatal(err)
	}
	return pair{initiator: initiator, responder: responder}
}

func (p pair) handshake(t *testing.T) (*noise.Transport, *noise.Transport, error) {
	t.Helper()
	p.responder.ReadHello(p.initiator.WriteHello([]byte("hi")))
	reply, err := p.responder.WriteReply([]byte("chain"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.initiator.ReadReply(reply); err != nil {
		return nil, nil, err
	}
	finish, err := p.initiator.WriteFinish([]byte("payload"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.responder.ReadFinish(finish); err != nil {
		return nil, nil, err
	}
	initiator, err := p.initiator.Split()
	if err != nil {
		t.Fatal(err)
	}
	responder, err := p.responder.Split()
	if err != nil {
		t.Fatal(err)
	}
	return initiator, responder, nil
}

func TestWhatsAppVariantInteroperatesWithItself(t *testing.T) {
	initiator, responder, err := newPair(t, noise.WhatsApp, noise.WhatsApp).handshake(t)
	if err != nil {
		t.Fatal(err)
	}
	first, second := initiator.Seal([]byte("one")), initiator.Seal([]byte("two"))
	if got, err := responder.OpenInPlace(first); err != nil || string(got) != "one" {
		t.Fatalf("first message = %q, %v", got, err)
	}
	if got, err := responder.OpenInPlace(second); err != nil || string(got) != "two" {
		t.Fatalf("second message = %q, %v", got, err)
	}
	reply := responder.Seal([]byte("back"))
	if got, err := initiator.OpenInPlace(reply); err != nil || string(got) != "back" {
		t.Fatalf("reply = %q, %v", got, err)
	}
}

func TestVariantsDoNotInteroperate(t *testing.T) {
	for _, variants := range [][2]noise.Variant{{noise.Standard, noise.WhatsApp}, {noise.WhatsApp, noise.Standard}} {
		if _, _, err := newPair(t, variants[0], variants[1]).handshake(t); err == nil {
			t.Fatalf("initiator %d completed a handshake with responder %d", variants[0], variants[1])
		}
	}
	if _, _, err := newPair(t, noise.Standard, noise.Standard).handshake(t); err != nil {
		t.Fatalf("standard with standard: %v", err)
	}
}

func TestTransportRejectsOutOfOrderAndTampered(t *testing.T) {
	initiator, responder, err := newPair(t, noise.WhatsApp, noise.WhatsApp).handshake(t)
	if err != nil {
		t.Fatal(err)
	}
	_ = initiator.Seal([]byte("skipped"))
	if _, err := responder.OpenInPlace(initiator.Seal([]byte("second"))); err == nil {
		t.Fatal("opened a message sealed with a later counter")
	}
	initiator, responder, err = newPair(t, noise.WhatsApp, noise.WhatsApp).handshake(t)
	if err != nil {
		t.Fatal(err)
	}
	tampered := initiator.Seal([]byte("message"))
	tampered[0] ^= 1
	if _, err := responder.OpenInPlace(tampered); err == nil {
		t.Fatal("opened a tampered message")
	}
}

func TestReadReplyRejectsTamperedStatic(t *testing.T) {
	p := newPair(t, noise.WhatsApp, noise.WhatsApp)
	p.responder.ReadHello(p.initiator.WriteHello(nil))
	reply, err := p.responder.WriteReply([]byte("chain"))
	if err != nil {
		t.Fatal(err)
	}
	reply.Static[0] ^= 1
	if _, err := p.initiator.ReadReply(reply); err == nil {
		t.Fatal("accepted a tampered responder static key")
	}
}

func TestReadFinishRejectsTamperedPayload(t *testing.T) {
	p := newPair(t, noise.WhatsApp, noise.WhatsApp)
	p.responder.ReadHello(p.initiator.WriteHello(nil))
	reply, err := p.responder.WriteReply(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.initiator.ReadReply(reply); err != nil {
		t.Fatal(err)
	}
	finish, err := p.initiator.WriteFinish([]byte("payload"))
	if err != nil {
		t.Fatal(err)
	}
	finish.Payload[0] ^= 1
	if _, err := p.responder.ReadFinish(finish); err == nil {
		t.Fatal("accepted a tampered initiator payload")
	}
}

func TestMixDHRejectsLowOrderPoint(t *testing.T) {
	state, err := noise.NewSymmetricState(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.MixDH(keyPair(t, bytes.Repeat([]byte{5}, 32)), curve.PublicKey{}); err == nil {
		t.Fatal("mixed a shared secret with the all-zero point")
	}
}

func TestNewPartiesNeedRandomness(t *testing.T) {
	static := keyPair(t, bytes.Repeat([]byte{1}, 32))
	if _, err := noise.NewInitiator(noise.WhatsApp, nil, static, bytes.NewReader(nil)); err == nil {
		t.Fatal("NewInitiator without randomness succeeded")
	}
	if _, err := noise.NewResponder(noise.WhatsApp, nil, static, bytes.NewReader(nil)); err == nil {
		t.Fatal("NewResponder without randomness succeeded")
	}
}
