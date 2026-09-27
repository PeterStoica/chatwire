package device

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/PeterStoica/chatwire/internal/curve"
	"github.com/PeterStoica/chatwire/internal/signal"
	"github.com/PeterStoica/chatwire/internal/signon"
)

const (
	signedPreKeyID    = 1
	maxRegistrationID = 16380
)

type Identity struct {
	noise          curve.KeyPair
	identity       curve.KeyPair
	signedPreKey   curve.KeyPair
	preKeySig      curve.Signature
	registrationID uint32
}

func New(random io.Reader) (Identity, error) {
	var id Identity
	var err error
	for _, target := range []*curve.KeyPair{&id.noise, &id.identity, &id.signedPreKey} {
		if *target, err = curve.NewKeyPair(random); err != nil {
			return Identity{}, err
		}
	}
	if id.preKeySig, err = id.identity.SignKey(random, id.signedPreKey.Public()); err != nil {
		return Identity{}, err
	}
	var raw [2]byte
	if _, err := io.ReadFull(random, raw[:]); err != nil {
		return Identity{}, fmt.Errorf("device: registration id: %w", err)
	}
	id.registrationID = uint32(binary.BigEndian.Uint16(raw[:]))%maxRegistrationID + 1
	return id, nil
}

type Stored struct {
	Noise          [curve.KeySize]byte `json:"noise"`
	Identity       [curve.KeySize]byte `json:"identity"`
	SignedPreKey   [curve.KeySize]byte `json:"signed_pre_key"`
	PreKeySig      curve.Signature     `json:"pre_key_signature"`
	RegistrationID uint32              `json:"registration_id"`
}

func (i Identity) Store() Stored {
	return Stored{
		Noise: i.noise.Seed(), Identity: i.identity.Seed(), SignedPreKey: i.signedPreKey.Seed(),
		PreKeySig: i.preKeySig, RegistrationID: i.registrationID,
	}
}

func Restore(s Stored) (Identity, error) {
	var id Identity
	var err error
	for _, pair := range []struct {
		seed   [curve.KeySize]byte
		target *curve.KeyPair
	}{{s.Noise, &id.noise}, {s.Identity, &id.identity}, {s.SignedPreKey, &id.signedPreKey}} {
		if *pair.target, err = curve.NewKeyPair(bytes.NewReader(pair.seed[:])); err != nil {
			return Identity{}, err
		}
	}
	id.preKeySig, id.registrationID = s.PreKeySig, s.RegistrationID
	return id, nil
}

func (i Identity) IdentityKey() curve.KeyPair {
	return i.identity
}

func (i Identity) Signal() signal.Local {
	return signal.Local{Identity: i.identity, RegistrationID: i.registrationID}
}

func (i Identity) SignedPreKey() curve.KeyPair {
	return i.signedPreKey
}

func (i Identity) NoiseKey() curve.KeyPair {
	return i.noise
}

func (i Identity) Registration(props signon.DeviceProps) signon.Registration {
	return signon.Registration{
		RegistrationID: i.registrationID,
		Identity:       i.identity.Public(),
		SignedPreKey:   signon.SignedPreKey{ID: signedPreKeyID, Key: i.signedPreKey.Public(), Signature: i.preKeySig},
		Device:         props,
	}
}

func Props() signon.DeviceProps {
	return signon.DeviceProps{OS: "Chatwire", Version: signon.Version{Primary: 0, Secondary: 1, Tertiary: 0}}
}
