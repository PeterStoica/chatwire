package message

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"

	"google.golang.org/protobuf/proto"

	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/wire"
)

const editUseCase = "Message Edit"

var ErrEdit = errors.New("message: encrypted edit does not decrypt")

type Addon struct {
	Secret   []byte
	ID       string
	Original node.JID
	Sender   node.JID
}

func addonCipher(secret []byte, id string, original, sender node.JID, useCase string) (cipher.AEAD, error) {
	if len(secret) != SecretSize {
		return nil, fmt.Errorf("%w: %d bytes", ErrSecret, len(secret))
	}
	info := id + original.WithoutDevice().String() + sender.WithoutDevice().String() + useCase
	key, err := hkdf.Key(sha256.New, secret, nil, info, 32)
	if err != nil {
		return nil, fmt.Errorf("message: %s key: %w", useCase, err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("message: %s key: %w", useCase, err)
	}
	return cipher.NewGCM(block)
}

func SealEdit(random io.Reader, a Addon, edited *wire.Message) (*wire.Message, error) {
	aead, err := addonCipher(a.Secret, a.ID, a.Original, a.Sender, editUseCase)
	if err != nil {
		return nil, err
	}
	plain, err := proto.Marshal(edited)
	if err != nil {
		return nil, fmt.Errorf("message: encode edit: %w", err)
	}
	iv := make([]byte, voteIVSize)
	if _, err := io.ReadFull(random, iv); err != nil {
		return nil, fmt.Errorf("message: edit iv: %w", err)
	}
	return &wire.Message{SecretEncryptedMessage: &wire.Message_SecretEncryptedMessage{
		TargetMessageKey: &wire.MessageKey{Id: &a.ID},
		EncPayload:       aead.Seal(nil, iv, plain, nil),
		EncIv:            iv,
		SecretEncType:    wire.Message_SecretEncryptedMessage_MESSAGE_EDIT.Enum(),
	}}, nil
}

func NeedsSecret(m *wire.Message) bool {
	return m.GetMessageContextInfo().GetMessageSecret() == nil && m.GetProtocolMessage() == nil && m.GetReactionMessage() == nil &&
		m.GetEncReactionMessage() == nil && m.GetPollUpdateMessage() == nil && m.GetSecretEncryptedMessage() == nil
}

func WithSecret(m *wire.Message, secret []byte) *wire.Message {
	out := proto.CloneOf(m)
	if out.MessageContextInfo == nil {
		out.MessageContextInfo = &wire.MessageContextInfo{}
	}
	out.MessageContextInfo.MessageSecret = secret
	return out
}

func EncryptedEdit(m *wire.Message) *wire.Message_SecretEncryptedMessage {
	secret := m.GetSecretEncryptedMessage()
	if secret.GetSecretEncType() != wire.Message_SecretEncryptedMessage_MESSAGE_EDIT || secret.GetTargetMessageKey().GetId() == "" {
		return nil
	}
	return secret
}

func OpenEdit(a Addon, enc *wire.Message_SecretEncryptedMessage) (*wire.Message, error) {
	aead, err := addonCipher(a.Secret, a.ID, a.Original, a.Sender, editUseCase)
	if err != nil {
		return nil, err
	}
	if len(enc.GetEncIv()) != aead.NonceSize() {
		return nil, fmt.Errorf("%w: iv of %d bytes", ErrEdit, len(enc.GetEncIv()))
	}
	plain, err := aead.Open(nil, enc.GetEncIv(), enc.GetEncPayload(), nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrEdit, err)
	}
	edited := &wire.Message{}
	if err := proto.Unmarshal(plain, edited); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrEdit, err)
	}
	return edited, nil
}
