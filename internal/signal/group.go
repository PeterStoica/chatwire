package signal

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"google.golang.org/protobuf/proto"

	"github.com/PeterStoica/chatwire/internal/curve"
	"github.com/PeterStoica/chatwire/internal/wire"
)

const maxSenderKeyStates = 5

var (
	ErrNoSenderKey    = errors.New("signal: no sender key with that id")
	ErrGroupSignature = errors.New("signal: sender key message signature invalid")
)

type SenderKey struct {
	id      uint32
	chain   chainKey
	signing curve.KeyPair
}

func NewSenderKey(random io.Reader) (*SenderKey, error) {
	signing, err := curve.NewKeyPair(random)
	if err != nil {
		return nil, err
	}
	var id [4]byte
	var seed [keySize]byte
	if _, err := io.ReadFull(random, id[:]); err != nil {
		return nil, fmt.Errorf("signal: sender key id: %w", err)
	}
	if _, err := io.ReadFull(random, seed[:]); err != nil {
		return nil, fmt.Errorf("signal: sender chain key: %w", err)
	}
	return &SenderKey{id: binary.LittleEndian.Uint32(id[:]), chain: chainKey{key: seed}, signing: signing}, nil
}

func (k *SenderKey) Distribution() ([]byte, error) {
	message, err := proto.Marshal(&wire.SenderKeyDistributionMessage{
		Id: new(k.id), Iteration: new(k.chain.index), ChainKey: k.chain.key[:], SigningKey: k.signing.Public().Typed(),
	})
	if err != nil {
		return nil, fmt.Errorf("signal: sender key distribution: %w", err)
	}
	return versioned(message), nil
}

func (k *SenderKey) Encrypt(random io.Reader, plaintext []byte) ([]byte, error) {
	keys, err := k.chain.groupKeys()
	if err != nil {
		return nil, err
	}
	body, err := encryptCBC(keys, plaintext)
	if err != nil {
		return nil, err
	}
	message, err := proto.Marshal(&wire.SenderKeyMessage{Id: new(k.id), Iteration: new(keys.index), Ciphertext: body})
	if err != nil {
		return nil, fmt.Errorf("signal: sender key message: %w", err)
	}
	serialized := versioned(message)
	signature, err := k.signing.Sign(random, serialized)
	if err != nil {
		return nil, err
	}
	k.chain = k.chain.next()
	return append(serialized, signature[:]...), nil
}

type SenderKeys struct {
	states []senderKeyState
}

type senderKeyState struct {
	id      uint32
	signing curve.PublicKey
	skippingChain
}

func (r *SenderKeys) Process(raw []byte) error {
	body, err := unversioned(raw)
	if err != nil {
		return err
	}
	var message wire.SenderKeyDistributionMessage
	if err := proto.Unmarshal(body, &message); err != nil {
		return fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	signing, err := untyped(message.GetSigningKey())
	if err != nil {
		return err
	}
	if message.Id == nil || message.Iteration == nil || len(message.GetChainKey()) != keySize {
		return fmt.Errorf("%w: incomplete sender key distribution", ErrMalformed)
	}
	fresh := senderKeyState{
		id: message.GetId(), signing: signing,
		chain: chainKey{key: [keySize]byte(message.GetChainKey()), index: message.GetIteration()},
	}
	kept := make([]senderKeyState, 0, maxSenderKeyStates)
	for _, st := range r.states {
		if st.id != fresh.id {
			kept = append(kept, st)
			continue
		}
		if st.signing == fresh.signing {
			fresh = st
		}
	}
	r.states = append([]senderKeyState{fresh}, kept...)
	r.states = r.states[:min(len(r.states), maxSenderKeyStates)]
	return nil
}

func (r *SenderKeys) Decrypt(raw []byte) ([]byte, error) {
	if len(raw) <= curve.SignatureSize {
		return nil, fmt.Errorf("%w: %d bytes", ErrMalformed, len(raw))
	}
	serialized, signature := raw[:len(raw)-curve.SignatureSize], curve.Signature(raw[len(raw)-curve.SignatureSize:])
	body, err := unversioned(serialized)
	if err != nil {
		return nil, err
	}
	var message wire.SenderKeyMessage
	if err := proto.Unmarshal(body, &message); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	if message.Id == nil || message.Iteration == nil || message.Ciphertext == nil {
		return nil, fmt.Errorf("%w: incomplete sender key message", ErrMalformed)
	}
	for i, st := range r.states {
		if st.id != message.GetId() {
			continue
		}
		if err := curve.Verify(st.signing, serialized, signature); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrGroupSignature, err)
		}
		trial := st.clone()
		keys, err := trial.keys(message.GetIteration(), chainKey.groupKeys)
		if err != nil {
			return nil, err
		}
		plaintext, err := decryptCBC(keys, message.GetCiphertext())
		if err != nil {
			return nil, err
		}
		r.states[i].skippingChain = trial
		return plaintext, nil
	}
	return nil, fmt.Errorf("%w: %d", ErrNoSenderKey, message.GetId())
}
