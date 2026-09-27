package signaltest

import (
	"encoding"

	"github.com/PeterStoica/chatwire/internal/signal"
)

func reload[T any, P interface {
	*T
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
}](value P) P {
	raw, err := value.MarshalBinary()
	if err != nil {
		panic(err)
	}
	restored := P(new(T))
	if err := restored.UnmarshalBinary(raw); err != nil {
		panic(err)
	}
	return restored
}

type persistingOurs struct {
	*ours
}

func NewPersistingOurs(self, peer Profile) Party {
	o, _ := NewOurs(self, peer).(*ours)
	return persistingOurs{o}
}

func (p persistingOurs) keep() {
	if p.session != nil {
		p.session = reload(p.session)
	}
}

func (p persistingOurs) Initiate(peer signal.Bundle) error {
	defer p.keep()
	return p.ours.Initiate(peer)
}

func (p persistingOurs) Send(text string) (Wire, error) {
	defer p.keep()
	return p.ours.Send(text)
}

func (p persistingOurs) Receive(w Wire) ([]byte, error) {
	defer p.keep()
	return p.ours.Receive(w)
}

type persistingMember struct {
	*ourMember
}

func NewPersistingOurMember(name string) Member {
	m, _ := NewOurMember(name).(*ourMember)
	return persistingMember{m}
}

func (p persistingMember) keep() {
	if p.own != nil {
		p.own = reload(p.own)
	}
	for sender, keys := range p.received {
		p.received[sender] = reload(keys)
	}
}

func (p persistingMember) Rotate() ([]byte, error) {
	defer p.keep()
	return p.ourMember.Rotate()
}

func (p persistingMember) Distribution() ([]byte, error) {
	defer p.keep()
	return p.ourMember.Distribution()
}

func (p persistingMember) Encrypt(text string) ([]byte, error) {
	defer p.keep()
	return p.ourMember.Encrypt(text)
}

func (p persistingMember) Process(sender string, distribution []byte) error {
	defer p.keep()
	return p.ourMember.Process(sender, distribution)
}

func (p persistingMember) Decrypt(sender string, raw []byte) ([]byte, error) {
	defer p.keep()
	return p.ourMember.Decrypt(sender, raw)
}
