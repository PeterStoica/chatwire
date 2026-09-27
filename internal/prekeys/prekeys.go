package prekeys

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/PeterStoica/chatwire/internal/curve"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/signon"
)

const (
	Batch      = 812
	MaxID      = 1<<24 - 1
	attrType   = "type"
	attrXMLNS  = "xmlns"
	namespace  = "encrypt"
	typeResult = "result"
)

var (
	ErrID       = errors.New("prekeys: key ids run from 1 to 16777215")
	ErrRejected = errors.New("prekeys: WhatsApp rejected the uploaded keys")
	ErrBackoff  = errors.New("prekeys: WhatsApp asked to retry later")
	ErrReply    = errors.New("prekeys: unexpected upload reply")
)

type PreKey struct {
	ID  uint32
	Key curve.KeyPair
}

func Generate(random io.Reader, first uint32, count int) ([]PreKey, uint32, error) {
	if first == 0 || first > MaxID {
		return nil, first, fmt.Errorf("%w: first id %d", ErrID, first)
	}
	keys := make([]PreKey, 0, count)
	next := first
	for range count {
		pair, err := curve.NewKeyPair(random)
		if err != nil {
			return nil, first, err
		}
		keys = append(keys, PreKey{ID: next, Key: pair})
		next = next%MaxID + 1
	}
	return keys, next, nil
}

func Upload(registration signon.Registration, keys []PreKey) node.Node {
	list := make([]node.Node, len(keys))
	for i, k := range keys {
		public := k.Key.Public()
		list[i] = node.Node{Tag: "key", Children: []node.Node{{Tag: "id", Bytes: keyID(k.ID)}, {Tag: "value", Bytes: public[:]}}}
	}
	identity, signed := registration.Identity, registration.SignedPreKey
	return node.Node{
		Tag: "iq",
		Attrs: []node.Attr{
			{Key: "id", Value: node.Value{}}, {Key: attrXMLNS, Value: node.Text(namespace)},
			{Key: attrType, Value: node.Text("set")}, {Key: "to", Value: server()},
		},
		Children: []node.Node{
			{Tag: "registration", Bytes: binary.BigEndian.AppendUint32(nil, registration.RegistrationID)},
			{Tag: attrType, Bytes: []byte{curve.KeyType}},
			{Tag: "identity", Bytes: identity[:]},
			{Tag: "list", Children: list},
			{Tag: "skey", Children: []node.Node{
				{Tag: "id", Bytes: keyID(signed.ID)},
				{Tag: "value", Bytes: signed.Key[:]},
				{Tag: "signature", Bytes: signed.Signature[:]},
			}},
		},
	}
}

func server() node.Value {
	return node.Address(node.JID{Server: node.ServerUser})
}

func keyID(id uint32) []byte {
	return binary.BigEndian.AppendUint32(nil, id)[1:]
}

func Result(reply node.Node) error {
	kind, _ := reply.Attr(attrType).Text()
	if reply.Tag != "iq" || kind != typeResult && kind != "error" {
		return fmt.Errorf("%w: %s", ErrReply, reply)
	}
	if kind == typeResult {
		return nil
	}
	failure, _ := reply.Child("error")
	text, _ := failure.Attr("text").Text()
	raw, _ := failure.Attr("code").Text()
	code, err := strconv.Atoi(raw)
	switch {
	case err != nil:
		return fmt.Errorf("%w: error without a numeric code: %s", ErrReply, reply)
	case code == 406:
		return fmt.Errorf("%w: %d %s", ErrRejected, code, text)
	case code >= 500:
		return fmt.Errorf("%w: %d %s", ErrBackoff, code, text)
	default:
		return fmt.Errorf("%w: error %d %s", ErrReply, code, text)
	}
}
