package mediaretry

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"strconv"

	"google.golang.org/protobuf/proto"

	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/wire"
)

const (
	info         = "WhatsApp Media Retry Notification"
	ivSize       = 12
	mediaKeySize = 32
)

var (
	ErrNotification = errors.New("mediaretry: malformed notification")
	ErrDecrypt      = errors.New("mediaretry: notification does not decrypt")
	ErrKey          = errors.New("mediaretry: media key must be 32 bytes")
)

type Target struct {
	Chat        node.JID
	FromMe      bool
	Participant node.JID
}

type Notification struct {
	MessageID  string
	Target     Target
	Ciphertext []byte
	IV         []byte
	ErrorCode  string
}

func sealer(mediaKey []byte) (cipher.AEAD, error) {
	if len(mediaKey) != mediaKeySize {
		return nil, fmt.Errorf("%w: %d bytes", ErrKey, len(mediaKey))
	}
	key, err := hkdf.Key(sha256.New, mediaKey, nil, info, 32)
	if err != nil {
		return nil, fmt.Errorf("mediaretry: key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("mediaretry: key: %w", err)
	}
	return cipher.NewGCM(block)
}

func Seal(random io.Reader, mediaKey []byte, messageID string, m proto.Message) (ciphertext, iv []byte, err error) {
	aead, err := sealer(mediaKey)
	if err != nil {
		return nil, nil, err
	}
	plain, err := proto.Marshal(m)
	if err != nil {
		return nil, nil, fmt.Errorf("mediaretry: encode: %w", err)
	}
	iv = make([]byte, ivSize)
	if _, err := io.ReadFull(random, iv); err != nil {
		return nil, nil, fmt.Errorf("mediaretry: iv: %w", err)
	}
	return aead.Seal(nil, iv, plain, []byte(messageID)), iv, nil
}

func Open(mediaKey []byte, messageID string, ciphertext, iv []byte, into proto.Message) error {
	aead, err := sealer(mediaKey)
	if err != nil {
		return err
	}
	if len(iv) != aead.NonceSize() {
		return fmt.Errorf("%w: iv of %d bytes", ErrDecrypt, len(iv))
	}
	plain, err := aead.Open(nil, iv, ciphertext, []byte(messageID))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrDecrypt, err)
	}
	if err := proto.Unmarshal(plain, into); err != nil {
		return fmt.Errorf("%w: %w", ErrDecrypt, err)
	}
	return nil
}

func Encrypt(random io.Reader, mediaKey []byte, messageID string) (ciphertext, iv []byte, err error) {
	return Seal(random, mediaKey, messageID, &wire.ServerErrorReceipt{StanzaId: new(messageID)})
}

func Decrypt(mediaKey []byte, n Notification) (*wire.MediaRetryNotification, error) {
	var out wire.MediaRetryNotification
	if err := Open(mediaKey, n.MessageID, n.Ciphertext, n.IV, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func encrypted(ciphertext, iv []byte) node.Node {
	return node.Node{Tag: "encrypt", Children: []node.Node{{Tag: "enc_p", Bytes: ciphertext}, {Tag: "enc_iv", Bytes: iv}}}
}

func Request(ownLID node.JID, messageID string, target Target, ciphertext, iv []byte) node.Node {
	rmr := []node.Attr{{Key: "jid", Value: node.Address(target.Chat.WithoutDevice())}, {Key: "from_me", Value: node.Text(strconv.FormatBool(target.FromMe))}}
	if target.Participant.Server != "" && (target.Chat.Server == node.ServerGroup || target.Chat.Server == node.ServerBroadcast) {
		rmr = append(rmr, node.Attr{Key: "participant", Value: node.Address(target.Participant.WithoutDevice())})
	}
	return node.Node{Tag: "receipt", Attrs: []node.Attr{
		{Key: "type", Value: node.Text("server-error")}, {Key: "to", Value: node.Address(ownLID.WithoutDevice())}, {Key: "id", Value: node.Text(messageID)},
	}, Children: []node.Node{encrypted(ciphertext, iv), {Tag: "rmr", Attrs: rmr}}}
}

func HistoryRequest(to node.JID, messageID string, ciphertext, iv []byte) node.Node {
	return node.Node{Tag: "receipt", Attrs: []node.Attr{
		{Key: "type", Value: node.Text("server-error")}, {Key: "to", Value: node.Address(to.WithoutDevice())},
		{Key: "id", Value: node.Text(messageID)}, {Key: "category", Value: node.Text("peer")},
	}, Children: []node.Node{encrypted(ciphertext, iv)}}
}

func Parse(n node.Node) (Notification, error) {
	kind, _ := n.Attr("type").Text()
	id, _ := n.Attr("id").Text()
	if n.Tag != "notification" || kind != "mediaretry" || id == "" {
		return Notification{}, fmt.Errorf("%w: %s", ErrNotification, n)
	}
	out := Notification{MessageID: id}
	rmr, ok := n.Child("rmr")
	if !ok {
		return Notification{}, fmt.Errorf("%w: no rmr in %s", ErrNotification, n)
	}
	out.Target.Chat, _ = rmr.Attr("jid").JID()
	out.Target.Participant, _ = rmr.Attr("participant").JID()
	fromMe, _ := rmr.Attr("from_me").Text()
	out.Target.FromMe = fromMe == "true"
	if failure, ok := n.Child("error"); ok {
		out.ErrorCode, _ = failure.Attr("code").Text()
		return out, nil
	}
	enc, ok := n.Child("encrypt")
	if !ok {
		return Notification{}, fmt.Errorf("%w: neither encrypt nor error in %s", ErrNotification, n)
	}
	payload, hasPayload := enc.Child("enc_p")
	iv, hasIV := enc.Child("enc_iv")
	if !hasPayload || !hasIV {
		return Notification{}, fmt.Errorf("%w: incomplete encrypt in %s", ErrNotification, n)
	}
	out.Ciphertext, out.IV = payload.Bytes, iv.Bytes
	return out, nil
}
