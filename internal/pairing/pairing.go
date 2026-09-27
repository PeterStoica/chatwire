package pairing

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"

	"google.golang.org/protobuf/proto"

	"github.com/PeterStoica/chatwire/internal/curve"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/wire"
)

type ClientType string

const ClientOtherWeb ClientType = "9"

const (
	qrPrefix      = "https://wa.me/settings/linked_devices#"
	advSecretSize = 32
)

var (
	ErrMalformed        = errors.New("pairing: malformed pair-success")
	ErrHMAC             = errors.New("pairing: device identity HMAC mismatch")
	ErrAccountSignature = errors.New("pairing: account signature mismatch")
)

type Companion struct {
	Noise     curve.KeyPair
	Identity  curve.KeyPair
	AdvSecret []byte
}

type Account struct {
	JID            node.JID
	LID            node.JID
	Platform       string
	BusinessName   string
	KeyIndex       uint32
	AccountKey     curve.PublicKey
	SignedIdentity []byte
}

func (a Account) Owns(j node.JID) bool {
	j = j.WithoutDevice()
	return j == a.JID.WithoutDevice() || a.LID.User != "" && j == a.LID.WithoutDevice()
}

func NewAdvSecret(random io.Reader) ([]byte, error) {
	secret := make([]byte, advSecretSize)
	if _, err := io.ReadFull(random, secret); err != nil {
		return nil, fmt.Errorf("pairing: adv secret: %w", err)
	}
	return secret, nil
}

func QRData(ref []byte, c Companion, client ClientType) string {
	noise, identity := c.Noise.Public(), c.Identity.Public()
	return qrPrefix + string(ref) + "," + base64.StdEncoding.EncodeToString(noise[:]) + "," +
		base64.StdEncoding.EncodeToString(identity[:]) + "," + base64.StdEncoding.EncodeToString(c.AdvSecret) + "," + string(client)
}

func Refs(n node.Node) [][]byte {
	pair, _ := n.Child("pair-device")
	var refs [][]byte
	for _, child := range pair.Children {
		if child.Tag == "ref" && len(child.Bytes) > 0 {
			refs = append(refs, child.Bytes)
		}
	}
	return refs
}

func Ack(request node.Node) node.Node {
	return node.Node{Tag: "iq", Attrs: []node.Attr{
		{Key: attrID, Value: request.Attr(attrID)},
		{Key: attrTo, Value: request.Attr("from")},
		{Key: attrType, Value: node.Text("result")},
	}}
}

func HandlePairSuccess(request node.Node, c Companion, random io.Reader) (node.Node, Account, error) {
	id := request.Attr(attrID)
	account, identity, err := verify(request, c)
	if err != nil {
		return notAuthorized(id), Account{}, err
	}
	message := bytes.Join([][]byte{{6, 1}, identity.GetDetails(), identityPublic(c), account.AccountKey[:]}, nil)
	signature, err := c.Identity.Sign(random, message)
	if err != nil {
		return notAuthorized(id), Account{}, err
	}
	identity.DeviceSignature = signature[:]
	identity.AccountSignatureKey = nil
	signed, err := proto.MarshalOptions{Deterministic: true}.Marshal(identity)
	if err != nil {
		return notAuthorized(id), Account{}, fmt.Errorf("pairing: marshal identity: %w", err)
	}
	account.SignedIdentity = signed
	signReply := reply("result", id, node.Node{Tag: "pair-device-sign", Children: []node.Node{{
		Tag:   "device-identity",
		Attrs: []node.Attr{{Key: "key-index", Value: node.Text(strconv.FormatUint(uint64(account.KeyIndex), 10))}},
		Bytes: signed,
	}}})
	return signReply, account, nil
}

func verify(request node.Node, c Companion) (Account, *wire.ADVSignedDeviceIdentity, error) {
	success, ok := request.Child("pair-success")
	if !ok {
		return Account{}, nil, fmt.Errorf("%w: no pair-success", ErrMalformed)
	}
	container, ok := success.Child("device-identity")
	if !ok || len(container.Bytes) == 0 {
		return Account{}, nil, fmt.Errorf("%w: no device-identity", ErrMalformed)
	}
	var wrapped wire.ADVSignedDeviceIdentityHMAC
	if err := proto.Unmarshal(container.Bytes, &wrapped); err != nil {
		return Account{}, nil, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	if !hmac.Equal(IdentityHMAC(c.AdvSecret, wrapped.GetDetails(), wrapped.GetAccountType() == wire.ADVEncryptionType_HOSTED), wrapped.GetHmac()) {
		return Account{}, nil, ErrHMAC
	}
	var identity wire.ADVSignedDeviceIdentity
	if err := proto.Unmarshal(wrapped.GetDetails(), &identity); err != nil {
		return Account{}, nil, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	var details wire.ADVDeviceIdentity
	if err := proto.Unmarshal(identity.GetDetails(), &details); err != nil {
		return Account{}, nil, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	accountKey, err := curve.ParsePublicKey(identity.GetAccountSignatureKey())
	if err != nil || len(identity.GetAccountSignature()) != curve.SignatureSize {
		return Account{}, nil, ErrAccountSignature
	}
	prefix := []byte{6, 0}
	if details.GetDeviceType() == wire.ADVEncryptionType_HOSTED {
		prefix = []byte{6, 5}
	}
	message := bytes.Join([][]byte{prefix, identity.GetDetails(), identityPublic(c)}, nil)
	if err := curve.Verify(accountKey, message, curve.Signature(identity.GetAccountSignature())); err != nil {
		return Account{}, nil, ErrAccountSignature
	}
	account := Account{KeyIndex: details.GetKeyIndex(), AccountKey: accountKey}
	if device, ok := success.Child("device"); ok {
		account.JID, _ = device.Attr("jid").JID()
		account.LID, _ = device.Attr("lid").JID()
	}
	if platform, ok := success.Child("platform"); ok {
		account.Platform, _ = platform.Attr("name").Text()
	}
	if biz, ok := success.Child("biz"); ok {
		account.BusinessName, _ = biz.Attr("name").Text()
	}
	return account, &identity, nil
}

func IdentityHMAC(advSecret, details []byte, hosted bool) []byte {
	mac := hmac.New(sha256.New, advSecret)
	if hosted {
		mac.Write([]byte{6, 5})
	}
	mac.Write(details)
	return mac.Sum(nil)
}

func identityPublic(c Companion) []byte {
	public := c.Identity.Public()
	return public[:]
}

func notAuthorized(id node.Value) node.Node {
	return reply("error", id, node.Node{Tag: "error", Attrs: []node.Attr{
		{Key: "text", Value: node.Text("not-authorized")},
		{Key: "code", Value: node.Text("401")},
	}})
}
