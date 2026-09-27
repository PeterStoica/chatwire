package signal

import (
	"bytes"
	"crypto/hmac"
	"errors"
	"fmt"
	"io"
	"slices"

	"google.golang.org/protobuf/proto"

	"github.com/PeterStoica/chatwire/internal/curve"
	"github.com/PeterStoica/chatwire/internal/wire"
)

const (
	version           = 3
	versionByte       = version<<4 | version
	maxFutureMessages = 2000
	maxReceiverChains = 5
	maxSkippedKeys    = 2000
	maxArchivedStates = 40
)

var (
	ErrMalformed     = errors.New("signal: malformed message")
	ErrVersion       = errors.New("signal: unsupported message version")
	ErrSignature     = errors.New("signal: signed prekey signature invalid")
	ErrDuplicate     = errors.New("signal: message already received")
	ErrTooFarAhead   = errors.New("signal: message too far in the future")
	ErrNoSession     = errors.New("signal: no session")
	ErrUnknownPreKey = errors.New("signal: unknown prekey")
)

type Local struct {
	Identity       curve.KeyPair
	RegistrationID uint32
}

type OneTimePreKey struct {
	ID  uint32
	Key curve.PublicKey
}

type Bundle struct {
	RegistrationID        uint32
	Identity              curve.PublicKey
	SignedPreKeyID        uint32
	SignedPreKey          curve.PublicKey
	SignedPreKeySignature curve.Signature
	PreKey                *OneTimePreKey
}

type PreKeys interface {
	PreKey(id uint32) (curve.KeyPair, bool)
	SignedPreKey(id uint32) (curve.KeyPair, bool)
}

type MessageType uint8

const (
	TypeMessage MessageType = iota + 1
	TypePreKeyMessage
)

type Ciphertext struct {
	Type  MessageType
	Bytes []byte
}

type receiverChain struct {
	ratchet curve.PublicKey
	skippingChain
}

type pending struct {
	preKeyID       *uint32
	signedPreKeyID uint32
}

type state struct {
	local             curve.PublicKey
	remote            curve.PublicKey
	root              [keySize]byte
	senderRatchet     curve.KeyPair
	sender            chainKey
	receivers         []receiverChain
	previousCounter   uint32
	pending           *pending
	localRegistration uint32
	baseKey           curve.PublicKey
}

type Session struct {
	states []*state
}

type exchange struct {
	ours   curve.KeyPair
	theirs curve.PublicKey
}

func agree(exchanges []exchange) ([keySize]byte, chainKey, error) {
	material := bytes.Repeat([]byte{0xff}, keySize)
	for _, e := range exchanges {
		shared, err := e.ours.SharedSecret(e.theirs)
		if err != nil {
			return [keySize]byte{}, chainKey{}, err
		}
		material = append(material, shared...)
	}
	return derive(material, nil, infoText)
}

func Initiate(random io.Reader, local Local, existing *Session, bundle Bundle) (*Session, error) {
	if err := curve.Verify(bundle.Identity, bundle.SignedPreKey.Typed(), bundle.SignedPreKeySignature); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSignature, err)
	}
	base, err := curve.NewKeyPair(random)
	if err != nil {
		return nil, err
	}
	exchanges := []exchange{{local.Identity, bundle.SignedPreKey}, {base, bundle.Identity}, {base, bundle.SignedPreKey}}
	p := &pending{signedPreKeyID: bundle.SignedPreKeyID}
	if bundle.PreKey != nil {
		exchanges = append(exchanges, exchange{base, bundle.PreKey.Key})
		p.preKeyID = new(bundle.PreKey.ID)
	}
	root, chain, err := agree(exchanges)
	if err != nil {
		return nil, err
	}
	sending, err := curve.NewKeyPair(random)
	if err != nil {
		return nil, err
	}
	root, sender, err := step(root, bundle.SignedPreKey, sending)
	if err != nil {
		return nil, err
	}
	return existing.replace(&state{
		local: local.Identity.Public(), remote: bundle.Identity, root: root,
		senderRatchet: sending, sender: sender,
		receivers: []receiverChain{{ratchet: bundle.SignedPreKey, chain: chain}},
		pending:   p, localRegistration: local.RegistrationID, baseKey: base.Public(),
	}), nil
}

func (s *Session) replace(st *state) *Session {
	if s == nil {
		return &Session{states: []*state{st}}
	}
	return &Session{states: append([]*state{st}, s.states[:min(len(s.states), maxArchivedStates)]...)}
}

func (s *Session) Merge(other *Session) *Session {
	switch {
	case other.empty():
		return s
	case s.empty():
		return other
	}
	states := append(append([]*state(nil), s.states...), other.states...)
	return &Session{states: states[:min(len(states), maxArchivedStates+1)]}
}

func (s *Session) empty() bool {
	return s == nil || len(s.states) == 0
}

func (s *Session) Encrypt(plaintext []byte) (Ciphertext, error) {
	if s.empty() {
		return Ciphertext{}, ErrNoSession
	}
	st := s.states[0]
	keys, err := st.sender.messageKeys()
	if err != nil {
		return Ciphertext{}, err
	}
	body, err := encryptCBC(keys, plaintext)
	if err != nil {
		return Ciphertext{}, err
	}
	message, err := proto.Marshal(&wire.SignalMessage{
		RatchetKey: st.senderRatchet.Public().Typed(), Counter: new(st.sender.index),
		PreviousCounter: new(st.previousCounter), Ciphertext: body,
	})
	if err != nil {
		return Ciphertext{}, err
	}
	serialized := versioned(message)
	serialized = append(serialized, mac(keys, st.local, st.remote, serialized)...)
	st.sender = st.sender.next()
	if st.pending == nil {
		return Ciphertext{Type: TypeMessage, Bytes: serialized}, nil
	}
	wrapper, err := proto.Marshal(&wire.PreKeySignalMessage{
		RegistrationId: new(st.localRegistration), PreKeyId: st.pending.preKeyID,
		SignedPreKeyId: new(st.pending.signedPreKeyID), BaseKey: st.baseKey.Typed(),
		IdentityKey: st.local.Typed(), Message: serialized,
	})
	if err != nil {
		return Ciphertext{}, err
	}
	return Ciphertext{Type: TypePreKeyMessage, Bytes: versioned(wrapper)}, nil
}

func (s *Session) Decrypt(random io.Reader, raw []byte) ([]byte, error) {
	if s.empty() {
		return nil, ErrNoSession
	}
	message, err := parseMessage(raw)
	if err != nil {
		return nil, err
	}
	var firstErr error
	for i, candidate := range s.states {
		trial := candidate.clone()
		plaintext, err := trial.decrypt(random, message)
		if err == nil {
			trial.pending = nil
			copy(s.states[1:i+1], s.states[:i])
			s.states[0] = trial
			return plaintext, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return nil, firstErr
}

func versioned(message []byte) []byte {
	return append([]byte{versionByte}, message...)
}

func unversioned(raw []byte) ([]byte, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("%w: empty message", ErrVersion)
	}
	if raw[0]>>4 != version {
		return nil, fmt.Errorf("%w: %#x", ErrVersion, raw[0])
	}
	return raw[1:], nil
}

type signalMessage struct {
	ratchet    curve.PublicKey
	counter    uint32
	body       []byte
	serialized []byte
	mac        []byte
}

func parseMessage(raw []byte) (signalMessage, error) {
	if len(raw) <= macSize {
		return signalMessage{}, fmt.Errorf("%w: %d bytes", ErrMalformed, len(raw))
	}
	serialized := raw[:len(raw)-macSize]
	body, err := unversioned(serialized)
	if err != nil {
		return signalMessage{}, err
	}
	var message wire.SignalMessage
	if err := proto.Unmarshal(body, &message); err != nil {
		return signalMessage{}, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	ratchet, err := untyped(message.GetRatchetKey())
	if err != nil {
		return signalMessage{}, err
	}
	if message.Counter == nil || message.Ciphertext == nil {
		return signalMessage{}, fmt.Errorf("%w: missing counter or ciphertext", ErrMalformed)
	}
	return signalMessage{
		ratchet: ratchet, counter: message.GetCounter(), body: message.GetCiphertext(),
		serialized: serialized, mac: raw[len(serialized):],
	}, nil
}

func (st *state) decrypt(random io.Reader, m signalMessage) ([]byte, error) {
	index, err := st.receiverChain(random, m.ratchet)
	if err != nil {
		return nil, err
	}
	keys, err := st.receivers[index].keys(m.counter, chainKey.messageKeys)
	if err != nil {
		return nil, err
	}
	if !hmac.Equal(mac(keys, st.remote, st.local, m.serialized), m.mac) {
		return nil, ErrMAC
	}
	return decryptCBC(keys, m.body)
}

func (st *state) receiverChain(random io.Reader, ratchet curve.PublicKey) (int, error) {
	for i, chain := range st.receivers {
		if chain.ratchet == ratchet {
			return i, nil
		}
	}
	root, receiving, err := step(st.root, ratchet, st.senderRatchet)
	if err != nil {
		return 0, err
	}
	fresh, err := curve.NewKeyPair(random)
	if err != nil {
		return 0, err
	}
	root, sending, err := step(root, ratchet, fresh)
	if err != nil {
		return 0, err
	}
	st.root = root
	st.receivers = append([]receiverChain{{ratchet: ratchet, chain: receiving}}, st.receivers...)
	st.receivers = st.receivers[:min(len(st.receivers), maxReceiverChains)]
	st.previousCounter = st.sender.index - 1
	st.senderRatchet, st.sender = fresh, sending
	return 0, nil
}

func (st *state) clone() *state {
	out := *st
	out.receivers = slices.Clone(st.receivers)
	for i := range out.receivers {
		out.receivers[i].skippingChain = out.receivers[i].clone()
	}
	return &out
}

func Respond(random io.Reader, local Local, keys PreKeys, existing *Session, raw []byte) (*Session, []byte, *uint32, error) {
	body, err := unversioned(raw)
	if err != nil {
		return nil, nil, nil, err
	}
	var message wire.PreKeySignalMessage
	if err := proto.Unmarshal(body, &message); err != nil {
		return nil, nil, nil, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	identity, err := untyped(message.GetIdentityKey())
	if err != nil {
		return nil, nil, nil, err
	}
	base, err := untyped(message.GetBaseKey())
	if err != nil {
		return nil, nil, nil, err
	}
	if existing.hasBase(base) {
		session := &Session{states: slices.Clone(existing.states)}
		plaintext, err := session.Decrypt(random, message.GetMessage())
		return session, plaintext, nil, err
	}
	fresh, usedPreKey, err := respond(local, keys, identity, base, &message)
	if err != nil {
		return nil, nil, nil, err
	}
	session := existing.replace(fresh)
	plaintext, err := session.Decrypt(random, message.GetMessage())
	if err != nil {
		return nil, nil, nil, err
	}
	return session, plaintext, usedPreKey, nil
}

func (s *Session) hasBase(base curve.PublicKey) bool {
	return s != nil && slices.ContainsFunc(s.states, func(st *state) bool { return st.baseKey == base })
}

func respond(local Local, keys PreKeys, identity, base curve.PublicKey, message *wire.PreKeySignalMessage) (*state, *uint32, error) {
	signed, ok := keys.SignedPreKey(message.GetSignedPreKeyId())
	if !ok {
		return nil, nil, fmt.Errorf("%w: signed prekey %d", ErrUnknownPreKey, message.GetSignedPreKeyId())
	}
	exchanges := []exchange{{signed, identity}, {local.Identity, base}, {signed, base}}
	var used *uint32
	if message.PreKeyId != nil {
		oneTime, ok := keys.PreKey(message.GetPreKeyId())
		if !ok {
			return nil, nil, fmt.Errorf("%w: prekey %d", ErrUnknownPreKey, message.GetPreKeyId())
		}
		exchanges = append(exchanges, exchange{oneTime, base})
		used = new(message.GetPreKeyId())
	}
	root, chain, err := agree(exchanges)
	if err != nil {
		return nil, nil, err
	}
	return &state{
		local: local.Identity.Public(), remote: identity, root: root,
		senderRatchet: signed, sender: chain,
		localRegistration: local.RegistrationID, baseKey: base,
	}, used, nil
}
