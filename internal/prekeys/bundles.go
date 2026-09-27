package prekeys

import (
	"errors"
	"fmt"

	"github.com/PeterStoica/chatwire/internal/curve"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/signal"
)

var (
	ErrBundle    = errors.New("prekeys: malformed key bundle")
	ErrUnfetched = errors.New("prekeys: WhatsApp returned no bundle for the device")
)

type Bundle struct {
	Device         node.JID
	Keys           signal.Bundle
	DeviceIdentity []byte
}

func FetchRequest(devices []node.JID) node.Node {
	users := make([]node.Node, len(devices))
	for i, device := range devices {
		users[i] = node.Node{Tag: "user", Attrs: []node.Attr{{Key: "jid", Value: node.Address(device)}}}
	}
	return node.Node{
		Tag: "iq",
		Attrs: []node.Attr{
			{Key: attrType, Value: node.Text("get")}, {Key: "id", Value: node.Value{}},
			{Key: attrXMLNS, Value: node.Text(namespace)}, {Key: "to", Value: server()},
		},
		Children: []node.Node{{Tag: "key", Children: users}},
	}
}

func ParseBundles(reply node.Node) ([]Bundle, map[node.JID]error, error) {
	list, ok := reply.Child("list")
	if kind, _ := reply.Attr(attrType).Text(); reply.Tag != "iq" || kind != typeResult || !ok {
		return nil, nil, fmt.Errorf("%w: %s", ErrBundle, reply)
	}
	var bundles []Bundle
	failed := map[node.JID]error{}
	for _, user := range list.Children {
		device, ok := user.Attr("jid").JID()
		if user.Tag != "user" || !ok {
			return nil, nil, fmt.Errorf("%w: entry %s", ErrBundle, user)
		}
		if failure, ok := user.Child("error"); ok {
			code, _ := failure.Attr("code").Text()
			text, _ := failure.Attr("text").Text()
			failed[device] = fmt.Errorf("%w: %s %s %s", ErrUnfetched, device, code, text)
			continue
		}
		bundle, err := parseBundle(device, user)
		if err != nil {
			return nil, nil, err
		}
		bundles = append(bundles, bundle)
	}
	return bundles, failed, nil
}

func parseBundle(device node.JID, user node.Node) (Bundle, error) {
	field := func(path ...string) []byte {
		n := user
		for _, tag := range path {
			child, ok := n.Child(tag)
			if !ok {
				return nil
			}
			n = child
		}
		return n.Bytes
	}
	registration, identity := field("registration"), field("identity")
	skeyID, skey, signature := field("skey", "id"), field("skey", "value"), field("skey", "signature")
	if keyType, ok := user.Child(attrType); ok && (len(keyType.Bytes) != 1 || keyType.Bytes[0] != curve.KeyType) {
		return Bundle{}, fmt.Errorf("%w: %s key type %x", ErrBundle, device, keyType.Bytes)
	}
	if len(registration) != 4 || len(identity) != curve.KeySize || len(skeyID) != 3 || len(skey) != curve.KeySize || len(signature) != curve.SignatureSize {
		return Bundle{}, fmt.Errorf("%w: %s field sizes", ErrBundle, device)
	}
	b := Bundle{Device: device, Keys: signal.Bundle{
		RegistrationID: bigEndian(registration), Identity: curve.PublicKey(identity),
		SignedPreKeyID: bigEndian(skeyID), SignedPreKey: curve.PublicKey(skey), SignedPreKeySignature: curve.Signature(signature),
	}}
	if _, ok := user.Child("key"); ok {
		id, value := field("key", "id"), field("key", "value")
		if len(id) != 3 || len(value) != curve.KeySize {
			return Bundle{}, fmt.Errorf("%w: %s one-time prekey sizes", ErrBundle, device)
		}
		b.Keys.PreKey = &signal.OneTimePreKey{ID: bigEndian(id), Key: curve.PublicKey(value)}
	}
	if identity, ok := user.Child("device-identity"); ok {
		b.DeviceIdentity = identity.Bytes
	}
	return b, nil
}
