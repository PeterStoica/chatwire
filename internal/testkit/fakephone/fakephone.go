package fakephone

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"google.golang.org/protobuf/proto"

	"github.com/PeterStoica/chatwire/internal/curve"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/pairing"
	"github.com/PeterStoica/chatwire/internal/wire"
)

const (
	attrType = "type"
	attrFrom = "from"
)

func setIQ(id string, child node.Node) node.Node {
	return node.Node{
		Tag: "iq",
		Attrs: []node.Attr{
			{Key: attrFrom, Value: node.Address(node.JID{Server: node.ServerUser})}, {Key: attrType, Value: node.Text("set")},
			{Key: "id", Value: node.Text(id)}, {Key: "xmlns", Value: node.Text("md")},
		},
		Children: []node.Node{child},
	}
}

func PairDevice(id string, refs int) node.Node {
	children := make([]node.Node, refs)
	for i := range children {
		children[i] = node.Node{Tag: "ref", Bytes: []byte(fmt.Sprintf("2@ref-%d", i+1))}
	}
	return setIQ(id, node.Node{Tag: "pair-device", Children: children})
}

func Result(request node.Node, children ...node.Node) node.Node {
	return node.Node{
		Tag:      "iq",
		Attrs:    []node.Attr{{Key: attrFrom, Value: request.Attr("to")}, {Key: attrType, Value: node.Text("result")}, {Key: "id", Value: request.Attr("id")}},
		Children: children,
	}
}

var ErrRejected = errors.New("fakephone: rejected")

type Phone struct {
	Account  curve.KeyPair
	JID      node.JID
	LID      node.JID
	KeyIndex uint32
	Hosted   bool
	Random   io.Reader

	ref                []byte
	wrappedCompanion   []byte
	ephemeral          curve.KeyPair
	companionEphemeral curve.PublicKey
}

type Scan struct {
	Ref       []byte
	Noise     curve.PublicKey
	Identity  curve.PublicKey
	AdvSecret []byte
	Client    string
}

func ParseQR(data string) (Scan, error) {
	rest, ok := strings.CutPrefix(data, "https://wa.me/settings/linked_devices#")
	if !ok {
		return Scan{}, fmt.Errorf("%w: not a linking QR", ErrRejected)
	}
	parts := strings.Split(rest, ",")
	if len(parts) != 5 {
		return Scan{}, fmt.Errorf("%w: %d QR fields", ErrRejected, len(parts))
	}
	decoded := make([][]byte, 3)
	for i, field := range parts[1:4] {
		raw, err := base64.StdEncoding.DecodeString(field)
		if err != nil {
			return Scan{}, fmt.Errorf("%w: field %d: %w", ErrRejected, i+1, err)
		}
		decoded[i] = raw
	}
	noise, err := curve.ParsePublicKey(decoded[0])
	if err != nil {
		return Scan{}, err
	}
	identity, err := curve.ParsePublicKey(decoded[1])
	if err != nil {
		return Scan{}, err
	}
	return Scan{Ref: []byte(parts[0]), Noise: noise, Identity: identity, AdvSecret: decoded[2], Client: parts[4]}, nil
}

func (p *Phone) PairSuccess(id string, companion curve.PublicKey, advSecret []byte) (node.Node, error) {
	encryption := wire.ADVEncryptionType_E2EE
	prefix := []byte{6, 0}
	if p.Hosted {
		encryption, prefix = wire.ADVEncryptionType_HOSTED, []byte{6, 5}
	}
	details, err := proto.Marshal(&wire.ADVDeviceIdentity{
		RawId: new(uint32(7)), Timestamp: new(uint64(1_800_000_000)), KeyIndex: new(p.KeyIndex),
		AccountType: new(encryption), DeviceType: new(encryption),
	})
	if err != nil {
		return node.Node{}, err
	}
	signature, err := p.Account.Sign(p.Random, bytes.Join([][]byte{prefix, details, companion[:]}, nil))
	if err != nil {
		return node.Node{}, err
	}
	accountKey := p.Account.Public()
	signed, err := proto.Marshal(&wire.ADVSignedDeviceIdentity{Details: details, AccountSignatureKey: accountKey[:], AccountSignature: signature[:]})
	if err != nil {
		return node.Node{}, err
	}
	container, err := proto.Marshal(&wire.ADVSignedDeviceIdentityHMAC{
		Details: signed, Hmac: pairing.IdentityHMAC(advSecret, signed, p.Hosted), AccountType: new(encryption),
	})
	if err != nil {
		return node.Node{}, err
	}
	return setIQ(id, node.Node{Tag: "pair-success", Children: []node.Node{
		{Tag: "jurisdiction", Attrs: []node.Attr{{Key: "iso", Value: node.Text("RO")}, {Key: "cc", Value: node.Text("40")}}},
		{Tag: "device-identity", Bytes: container},
		{Tag: "platform", Attrs: []node.Attr{{Key: "name", Value: node.Text("android")}}},
		{Tag: "device", Attrs: []node.Attr{{Key: "jid", Value: node.Address(p.JID)}, {Key: "lid", Value: node.Address(p.LID)}}},
	}}), nil
}

func (p *Phone) VerifyDeviceSign(reply node.Node, companion curve.PublicKey) error {
	sign, ok := reply.Child("pair-device-sign")
	if !ok {
		return fmt.Errorf("%w: reply has no pair-device-sign: %s", ErrRejected, reply)
	}
	identityNode, ok := sign.Child("device-identity")
	if !ok {
		return fmt.Errorf("%w: no device-identity", ErrRejected)
	}
	var identity wire.ADVSignedDeviceIdentity
	if err := proto.Unmarshal(identityNode.Bytes, &identity); err != nil {
		return fmt.Errorf("%w: %w", ErrRejected, err)
	}
	if identity.AccountSignatureKey != nil {
		return fmt.Errorf("%w: account signature key sent back", ErrRejected)
	}
	accountKey := p.Account.Public()
	message := bytes.Join([][]byte{{6, 1}, identity.GetDetails(), companion[:], accountKey[:]}, nil)
	if err := curve.Verify(companion, message, curve.Signature(identity.GetDeviceSignature())); err != nil {
		return fmt.Errorf("%w: device signature: %w", ErrRejected, err)
	}
	return nil
}

func (p *Phone) ReceiveHello(hello node.Node) (node.Node, error) {
	reg, ok := hello.Child("link_code_companion_reg")
	if !ok {
		return node.Node{}, fmt.Errorf("%w: no link_code_companion_reg", ErrRejected)
	}
	wrapped, _ := reg.Child("link_code_pairing_wrapped_companion_ephemeral_pub")
	p.wrappedCompanion = wrapped.Bytes
	material := make([]byte, 6)
	if _, err := io.ReadFull(p.Random, material); err != nil {
		return node.Node{}, err
	}
	p.ref = []byte("ref-" + base64.RawStdEncoding.EncodeToString(material))
	return Result(hello, node.Node{
		Tag:      "link_code_companion_reg",
		Attrs:    []node.Attr{{Key: "stage", Value: node.Text("companion_hello")}},
		Children: []node.Node{{Tag: "link_code_pairing_ref", Bytes: p.ref}},
	}), nil
}

func (p *Phone) TypeCode(typed string) (node.Node, error) {
	companionEphemeral, err := pairing.UnwrapWithCode(strings.ReplaceAll(typed, "-", ""), p.wrappedCompanion)
	if err != nil {
		return node.Node{}, err
	}
	p.companionEphemeral = companionEphemeral
	if p.ephemeral, err = curve.NewKeyPair(p.Random); err != nil {
		return node.Node{}, err
	}
	material := make([]byte, 48)
	if _, err := io.ReadFull(p.Random, material); err != nil {
		return node.Node{}, err
	}
	primaryWrapped, err := pairing.WrapWithCode(strings.ReplaceAll(typed, "-", ""), p.ephemeral.Public(), material[:32], material[32:48])
	if err != nil {
		return node.Node{}, err
	}
	accountKey := p.Account.Public()
	return Notification("n1", "link_code_companion_reg", node.Node{Tag: "link_code_companion_reg", Attrs: []node.Attr{{Key: "stage", Value: node.Text("primary_hello")}}, Children: []node.Node{
		{Tag: "link_code_pairing_ref", Bytes: p.ref},
		{Tag: "link_code_pairing_wrapped_primary_ephemeral_pub", Bytes: primaryWrapped},
		{Tag: "primary_identity_pub", Bytes: accountKey[:]},
	}}), nil
}

func (p *Phone) Ref() []byte {
	return p.ref
}

func Notification(id, kind string, children ...node.Node) node.Node {
	return node.Node{Tag: "notification", Attrs: []node.Attr{
		{Key: attrType, Value: node.Text(kind)},
		{Key: "id", Value: node.Text(id)},
		{Key: attrFrom, Value: node.Address(node.JID{Server: node.ServerUser})},
	}, Children: children}
}

func RefreshCode(id string, ref []byte, forced bool) node.Node {
	attrs := []node.Attr{{Key: "stage", Value: node.Text("refresh_code")}}
	if forced {
		attrs = append(attrs, node.Attr{Key: "force_manual_refresh", Value: node.Text("true")})
	}
	return Notification(id, "link_code_companion_reg", node.Node{Tag: "link_code_companion_reg", Attrs: attrs, Children: []node.Node{{Tag: "link_code_pairing_ref", Bytes: ref}}})
}

func RotateQR(id string) node.Node {
	return Notification(id, "companion_reg_refresh", node.Node{Tag: "pair-device-rotate-qr"})
}

func (p *Phone) CompanionFinish(finish node.Node) ([]byte, curve.PublicKey, error) {
	reg, ok := finish.Child("link_code_companion_reg")
	if !ok {
		return nil, curve.PublicKey{}, fmt.Errorf("%w: no link_code_companion_reg", ErrRejected)
	}
	bundleNode, _ := reg.Child("link_code_pairing_wrapped_key_bundle")
	identityNode, _ := reg.Child("companion_identity_public")
	companionIdentity, err := curve.ParsePublicKey(identityNode.Bytes)
	if err != nil {
		return nil, curve.PublicKey{}, err
	}
	if len(bundleNode.Bytes) < 44+16 {
		return nil, curve.PublicKey{}, fmt.Errorf("%w: short key bundle", ErrRejected)
	}
	ephemeralShared, err := p.ephemeral.SharedSecret(p.companionEphemeral)
	if err != nil {
		return nil, curve.PublicKey{}, err
	}
	key, err := pairing.BundleKey(ephemeralShared, bundleNode.Bytes[:32])
	if err != nil {
		return nil, curve.PublicKey{}, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, curve.PublicKey{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, curve.PublicKey{}, err
	}
	plain, err := gcm.Open(nil, bundleNode.Bytes[32:44], bundleNode.Bytes[44:], nil)
	if err != nil {
		return nil, curve.PublicKey{}, fmt.Errorf("%w: key bundle: %w", ErrRejected, err)
	}
	accountKey := p.Account.Public()
	if len(plain) != 96 || !bytes.Equal(plain[:32], companionIdentity[:]) || !bytes.Equal(plain[32:64], accountKey[:]) {
		return nil, curve.PublicKey{}, fmt.Errorf("%w: key bundle contents", ErrRejected)
	}
	identityShared, err := p.Account.SharedSecret(companionIdentity)
	if err != nil {
		return nil, curve.PublicKey{}, err
	}
	secret, err := pairing.AdvSecretFrom(ephemeralShared, identityShared, plain[64:])
	return secret, companionIdentity, err
}
