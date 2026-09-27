package signal

import (
	"bytes"
	"crypto/aes"
	"encoding/json/v2"
	"errors"
	"fmt"

	"github.com/PeterStoica/chatwire/internal/curve"
)

const recordVersion = 1

var ErrRecord = errors.New("signal: unreadable stored record")

type chainRecord struct {
	Key   [keySize]byte `json:"key"`
	Index uint32        `json:"index"`
}

type keysRecord struct {
	Cipher [keySize]byte       `json:"cipher"`
	MAC    [keySize]byte       `json:"mac"`
	IV     [aes.BlockSize]byte `json:"iv"`
	Index  uint32              `json:"index"`
}

type skippingRecord struct {
	Chain   chainRecord  `json:"chain"`
	Skipped []keysRecord `json:"skipped,omitempty"`
}

type receiverRecord struct {
	Ratchet curve.PublicKey `json:"ratchet"`
	Chain   skippingRecord  `json:"chain"`
}

type pendingRecord struct {
	PreKeyID       *uint32 `json:"pre_key_id,omitempty"`
	SignedPreKeyID uint32  `json:"signed_pre_key_id"`
}

type stateRecord struct {
	Local             curve.PublicKey  `json:"local"`
	Remote            curve.PublicKey  `json:"remote"`
	Root              [keySize]byte    `json:"root"`
	SenderRatchet     [keySize]byte    `json:"sender_ratchet"`
	Sender            chainRecord      `json:"sender"`
	Receivers         []receiverRecord `json:"receivers"`
	PreviousCounter   uint32           `json:"previous_counter"`
	Pending           *pendingRecord   `json:"pending,omitempty"`
	LocalRegistration uint32           `json:"local_registration"`
	BaseKey           curve.PublicKey  `json:"base_key"`
}

type sessionRecord struct {
	Version int           `json:"version"`
	States  []stateRecord `json:"states"`
}

type senderKeyRecord struct {
	Version int           `json:"version"`
	ID      uint32        `json:"id"`
	Chain   chainRecord   `json:"chain"`
	Signing [keySize]byte `json:"signing"`
}

type senderStateRecord struct {
	ID      uint32          `json:"id"`
	Signing curve.PublicKey `json:"signing"`
	Chain   skippingRecord  `json:"chain"`
}

type senderKeysRecord struct {
	Version int                 `json:"version"`
	States  []senderStateRecord `json:"states"`
}

func (c chainKey) record() chainRecord {
	return chainRecord{Key: c.key, Index: c.index}
}

func (r chainRecord) chain() chainKey {
	return chainKey{key: r.Key, index: r.Index}
}

func (c *skippingChain) record() skippingRecord {
	out := skippingRecord{Chain: c.chain.record(), Skipped: make([]keysRecord, len(c.skipped))}
	for i, k := range c.skipped {
		out.Skipped[i] = keysRecord{Cipher: k.cipher, MAC: k.mac, IV: k.iv, Index: k.index}
	}
	return out
}

func (r skippingRecord) chain() skippingChain {
	out := skippingChain{chain: r.Chain.chain(), skipped: make([]messageKeys, len(r.Skipped))}
	for i, k := range r.Skipped {
		out.skipped[i] = messageKeys{cipher: k.Cipher, mac: k.MAC, iv: k.IV, index: k.Index}
	}
	return out
}

func keyPair(seed [keySize]byte) (curve.KeyPair, error) {
	pair, err := curve.NewKeyPair(bytes.NewReader(seed[:]))
	if err != nil {
		return curve.KeyPair{}, fmt.Errorf("%w: %w", ErrRecord, err)
	}
	return pair, nil
}

func (s *Session) MarshalBinary() ([]byte, error) {
	record := sessionRecord{Version: recordVersion}
	if s != nil {
		for _, st := range s.states {
			record.States = append(record.States, st.record())
		}
	}
	return json.Marshal(record)
}

func (st *state) record() stateRecord {
	out := stateRecord{
		Local: st.local, Remote: st.remote, Root: st.root, SenderRatchet: st.senderRatchet.Seed(), Sender: st.sender.record(),
		PreviousCounter: st.previousCounter, LocalRegistration: st.localRegistration, BaseKey: st.baseKey,
		Receivers: make([]receiverRecord, len(st.receivers)),
	}
	for i, r := range st.receivers {
		out.Receivers[i] = receiverRecord{Ratchet: r.ratchet, Chain: r.skippingChain.record()}
	}
	if st.pending != nil {
		out.Pending = &pendingRecord{PreKeyID: st.pending.preKeyID, SignedPreKeyID: st.pending.signedPreKeyID}
	}
	return out
}

func (s *Session) UnmarshalBinary(data []byte) error {
	var record sessionRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return fmt.Errorf("%w: %w", ErrRecord, err)
	}
	if record.Version != recordVersion {
		return fmt.Errorf("%w: session record version %d", ErrRecord, record.Version)
	}
	states := make([]*state, 0, len(record.States))
	for _, r := range record.States {
		st, err := r.state()
		if err != nil {
			return err
		}
		states = append(states, st)
	}
	s.states = states
	return nil
}

func (r stateRecord) state() (*state, error) {
	ratchet, err := keyPair(r.SenderRatchet)
	if err != nil {
		return nil, err
	}
	st := &state{
		local: r.Local, remote: r.Remote, root: r.Root, senderRatchet: ratchet, sender: r.Sender.chain(),
		previousCounter: r.PreviousCounter, localRegistration: r.LocalRegistration, baseKey: r.BaseKey,
		receivers: make([]receiverChain, len(r.Receivers)),
	}
	for i, receiver := range r.Receivers {
		st.receivers[i] = receiverChain{ratchet: receiver.Ratchet, skippingChain: receiver.Chain.chain()}
	}
	if r.Pending != nil {
		st.pending = &pending{preKeyID: r.Pending.PreKeyID, signedPreKeyID: r.Pending.SignedPreKeyID}
	}
	return st, nil
}

func (k *SenderKey) MarshalBinary() ([]byte, error) {
	return json.Marshal(senderKeyRecord{Version: recordVersion, ID: k.id, Chain: k.chain.record(), Signing: k.signing.Seed()})
}

func (k *SenderKey) UnmarshalBinary(data []byte) error {
	var record senderKeyRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return fmt.Errorf("%w: %w", ErrRecord, err)
	}
	if record.Version != recordVersion {
		return fmt.Errorf("%w: sender key record version %d", ErrRecord, record.Version)
	}
	signing, err := keyPair(record.Signing)
	if err != nil {
		return err
	}
	k.id, k.chain, k.signing = record.ID, record.Chain.chain(), signing
	return nil
}

func (r *SenderKeys) MarshalBinary() ([]byte, error) {
	record := senderKeysRecord{Version: recordVersion, States: make([]senderStateRecord, len(r.states))}
	for i, st := range r.states {
		record.States[i] = senderStateRecord{ID: st.id, Signing: st.signing, Chain: st.skippingChain.record()}
	}
	return json.Marshal(record)
}

func (r *SenderKeys) UnmarshalBinary(data []byte) error {
	var record senderKeysRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return fmt.Errorf("%w: %w", ErrRecord, err)
	}
	if record.Version != recordVersion {
		return fmt.Errorf("%w: sender keys record version %d", ErrRecord, record.Version)
	}
	r.states = make([]senderKeyState, len(record.States))
	for i, st := range record.States {
		r.states[i] = senderKeyState{id: st.ID, signing: st.Signing, skippingChain: st.Chain.chain()}
	}
	return nil
}
