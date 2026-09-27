package noise

import (
	"fmt"
	"io"

	"github.com/PeterStoica/chatwire/internal/curve"
)

type Variant uint8

const (
	Standard Variant = iota
	WhatsApp
)

type Hello struct {
	Ephemeral curve.PublicKey
	Payload   []byte
}

type Reply struct {
	Ephemeral curve.PublicKey
	Static    []byte
	Payload   []byte
}

type Finish struct {
	Static  []byte
	Payload []byte
}

type party struct {
	state         *SymmetricState
	variant       Variant
	static        curve.KeyPair
	ephemeral     curve.KeyPair
	peerEphemeral curve.PublicKey
	peerStatic    curve.PublicKey
}

func newParty(variant Variant, prologue []byte, static curve.KeyPair, random io.Reader) (party, error) {
	state, err := NewSymmetricState(prologue)
	if err != nil {
		return party{}, err
	}
	ephemeral, err := curve.NewKeyPair(random)
	if err != nil {
		return party{}, err
	}
	return party{state: state, variant: variant, static: static, ephemeral: ephemeral}, nil
}

func (p *party) hashHello(hello Hello) {
	p.state.MixHash(hello.Ephemeral[:])
	if p.variant == Standard {
		p.state.MixHash(hello.Payload)
	}
}

func (p *party) HandshakeHash() []byte {
	return p.state.HandshakeHash()
}

func (p *party) PeerStatic() curve.PublicKey {
	return p.peerStatic
}

type Initiator struct {
	party
}

func NewInitiator(variant Variant, prologue []byte, static curve.KeyPair, random io.Reader) (*Initiator, error) {
	p, err := newParty(variant, prologue, static, random)
	if err != nil {
		return nil, err
	}
	return &Initiator{party: p}, nil
}

func (i *Initiator) WriteHello(payload []byte) Hello {
	hello := Hello{Ephemeral: i.ephemeral.Public(), Payload: payload}
	i.hashHello(hello)
	return hello
}

func (i *Initiator) ReadReply(reply Reply) ([]byte, error) {
	i.peerEphemeral = reply.Ephemeral
	i.state.MixHash(reply.Ephemeral[:])
	if err := i.state.MixDH(i.ephemeral, i.peerEphemeral); err != nil {
		return nil, err
	}
	static, err := i.state.DecryptAndHash(reply.Static)
	if err != nil {
		return nil, err
	}
	if i.peerStatic, err = curve.ParsePublicKey(static); err != nil {
		return nil, fmt.Errorf("noise: responder static: %w", err)
	}
	if err := i.state.MixDH(i.ephemeral, i.peerStatic); err != nil {
		return nil, err
	}
	return i.state.DecryptAndHash(reply.Payload)
}

func (i *Initiator) WriteFinish(payload []byte) (Finish, error) {
	static := i.static.Public()
	encryptedStatic := i.state.EncryptAndHash(static[:])
	if err := i.state.MixDH(i.static, i.peerEphemeral); err != nil {
		return Finish{}, err
	}
	return Finish{Static: encryptedStatic, Payload: i.state.EncryptAndHash(payload)}, nil
}

func (i *Initiator) Split() (*Transport, error) {
	return i.state.split(false)
}

type Responder struct {
	party
}

func NewResponder(variant Variant, prologue []byte, static curve.KeyPair, random io.Reader) (*Responder, error) {
	p, err := newParty(variant, prologue, static, random)
	if err != nil {
		return nil, err
	}
	return &Responder{party: p}, nil
}

func (r *Responder) ReadHello(hello Hello) {
	r.peerEphemeral = hello.Ephemeral
	r.hashHello(hello)
}

func (r *Responder) WriteReply(payload []byte) (Reply, error) {
	ephemeral, static := r.ephemeral.Public(), r.static.Public()
	r.state.MixHash(ephemeral[:])
	if err := r.state.MixDH(r.ephemeral, r.peerEphemeral); err != nil {
		return Reply{}, err
	}
	encryptedStatic := r.state.EncryptAndHash(static[:])
	if err := r.state.MixDH(r.static, r.peerEphemeral); err != nil {
		return Reply{}, err
	}
	return Reply{Ephemeral: ephemeral, Static: encryptedStatic, Payload: r.state.EncryptAndHash(payload)}, nil
}

func (r *Responder) ReadFinish(finish Finish) ([]byte, error) {
	static, err := r.state.DecryptAndHash(finish.Static)
	if err != nil {
		return nil, err
	}
	if r.peerStatic, err = curve.ParsePublicKey(static); err != nil {
		return nil, fmt.Errorf("noise: initiator static: %w", err)
	}
	if err := r.state.MixDH(r.ephemeral, r.peerStatic); err != nil {
		return nil, err
	}
	return r.state.DecryptAndHash(finish.Payload)
}

func (r *Responder) Split() (*Transport, error) {
	return r.state.split(true)
}
