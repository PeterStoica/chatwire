package pairing

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/pbkdf2"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/PeterStoica/chatwire/internal/curve"
	"github.com/PeterStoica/chatwire/internal/node"
)

const (
	codeAlphabet   = "123456789ABCDEFGHJKLMNPQRSTVWXYZ"
	CodeIterations = 2 << 16
	codeBytes      = 5
	saltSize       = 32
	ivSize         = 16
	nonceSize      = 12
	wrappedKeySize = saltSize + ivSize + curve.KeySize
	bundleInfo     = "link_code_pairing_key_bundle_encryption_key"
	advSecretInfo  = "adv_secret"
	minPhoneDigits = 7
)

var (
	ErrPhone        = errors.New("pairing: phone number must be international, digits only after the +")
	ErrNotification = errors.New("pairing: malformed code pairing notification")
)

type CodeRequest struct {
	JID       node.JID
	Code      string
	ephemeral curve.KeyPair
	ref       []byte
}

func EncodeLinkingCode(raw []byte) string {
	return base32.NewEncoding(codeAlphabet).WithPadding(base32.NoPadding).EncodeToString(raw)
}

func LinkingKey(code string, salt []byte) ([]byte, error) {
	key, err := pbkdf2.Key(sha256.New, code, salt, CodeIterations, 32)
	if err != nil {
		return nil, fmt.Errorf("pairing: pbkdf2: %w", err)
	}
	return key, nil
}

func WrapWithCode(code string, public curve.PublicKey, salt, iv []byte) ([]byte, error) {
	key, err := LinkingKey(code, salt)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("pairing: aes: %w", err)
	}
	encrypted := make([]byte, curve.KeySize)
	cipher.NewCTR(block, iv).XORKeyStream(encrypted, public[:])
	return bytes.Join([][]byte{salt, iv, encrypted}, nil), nil
}

func UnwrapWithCode(code string, wrapped []byte) (curve.PublicKey, error) {
	if len(wrapped) < wrappedKeySize {
		return curve.PublicKey{}, fmt.Errorf("%w: wrapped key is %d bytes", ErrNotification, len(wrapped))
	}
	key, err := LinkingKey(code, wrapped[:saltSize])
	if err != nil {
		return curve.PublicKey{}, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return curve.PublicKey{}, fmt.Errorf("pairing: aes: %w", err)
	}
	var public curve.PublicKey
	cipher.NewCTR(block, wrapped[saltSize:saltSize+ivSize]).XORKeyStream(public[:], wrapped[saltSize+ivSize:wrappedKeySize])
	return public, nil
}

func AdvSecretFrom(ephemeralShared, identityShared, random []byte) ([]byte, error) {
	secret, err := hkdf.Key(sha256.New, bytes.Join([][]byte{ephemeralShared, identityShared, random}, nil), nil, advSecretInfo, advSecretSize)
	if err != nil {
		return nil, fmt.Errorf("pairing: hkdf: %w", err)
	}
	return secret, nil
}

func BundleKey(ephemeralShared, salt []byte) ([]byte, error) {
	key, err := hkdf.Key(sha256.New, ephemeralShared, salt, bundleInfo, 32)
	if err != nil {
		return nil, fmt.Errorf("pairing: hkdf: %w", err)
	}
	return key, nil
}

type Phone string

func ParsePhone(s string) (Phone, error) {
	digits := strings.TrimPrefix(strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(s), "+")
	if len(digits) < minPhoneDigits || strings.HasPrefix(digits, "0") || strings.Trim(digits, "0123456789") != "" {
		return "", fmt.Errorf("%w: %q", ErrPhone, s)
	}
	return Phone(digits), nil
}

func StartCode(random io.Reader, phone Phone, c Companion, client ClientType, display string) (*CodeRequest, node.Node, error) {
	ephemeral, err := curve.NewKeyPair(random)
	if err != nil {
		return nil, node.Node{}, err
	}
	material := make([]byte, saltSize+ivSize+codeBytes)
	if _, err := io.ReadFull(random, material); err != nil {
		return nil, node.Node{}, fmt.Errorf("pairing: code randomness: %w", err)
	}
	salt, iv, raw := material[:saltSize], material[saltSize:saltSize+ivSize], material[saltSize+ivSize:]
	request := &CodeRequest{JID: node.JID{User: string(phone), Server: node.ServerUser}, Code: EncodeLinkingCode(raw), ephemeral: ephemeral}
	wrapped, err := WrapWithCode(request.Code, ephemeral.Public(), salt, iv)
	if err != nil {
		return nil, node.Node{}, err
	}
	noise := c.Noise.Public()
	hello := iqSet(node.Node{
		Tag: "link_code_companion_reg",
		Attrs: []node.Attr{
			{Key: "stage", Value: node.Text("companion_hello")},
			{Key: "should_show_push_notification", Value: node.Text("false")},
			{Key: "jid", Value: node.Address(request.JID)},
		},
		Children: []node.Node{
			{Tag: "link_code_pairing_wrapped_companion_ephemeral_pub", Bytes: wrapped},
			{Tag: "companion_server_auth_key_pub", Bytes: noise[:]},
			{Tag: "companion_platform_id", Bytes: []byte(client)},
			{Tag: "companion_platform_display", Bytes: []byte(display)},
			{Tag: "link_code_pairing_nonce", Bytes: []byte{0}},
		},
	})
	return request, hello, nil
}

func (r *CodeRequest) Display() string {
	return r.Code[:4] + "-" + r.Code[4:]
}

func (r *CodeRequest) AcceptRef(response node.Node) error {
	reg, ok := response.Child("link_code_companion_reg")
	if !ok {
		return fmt.Errorf("%w: no link_code_companion_reg in response", ErrNotification)
	}
	ref, ok := reg.Child("link_code_pairing_ref")
	if !ok || len(ref.Bytes) == 0 {
		return fmt.Errorf("%w: no link_code_pairing_ref", ErrNotification)
	}
	r.ref = ref.Bytes
	return nil
}

func (r *CodeRequest) Finish(notification node.Node, c Companion, random io.Reader) (node.Node, []byte, error) {
	reg, ok := notification.Child("link_code_companion_reg")
	if !ok {
		return node.Node{}, nil, fmt.Errorf("%w: no link_code_companion_reg", ErrNotification)
	}
	ref, _ := reg.Child("link_code_pairing_ref")
	if !bytes.Equal(ref.Bytes, r.ref) {
		return node.Node{}, nil, fmt.Errorf("%w: pairing ref mismatch", ErrNotification)
	}
	wrapped, _ := reg.Child("link_code_pairing_wrapped_primary_ephemeral_pub")
	primaryEphemeral, err := UnwrapWithCode(r.Code, wrapped.Bytes)
	if err != nil {
		return node.Node{}, nil, err
	}
	identityNode, _ := reg.Child("primary_identity_pub")
	primaryIdentity, err := curve.ParsePublicKey(identityNode.Bytes)
	if err != nil {
		return node.Node{}, nil, fmt.Errorf("%w: primary identity: %w", ErrNotification, err)
	}
	material := make([]byte, advSecretSize+saltSize+nonceSize)
	if _, err := io.ReadFull(random, material); err != nil {
		return node.Node{}, nil, fmt.Errorf("pairing: bundle randomness: %w", err)
	}
	advRandom, bundleSalt, bundleNonce := material[:advSecretSize], material[advSecretSize:advSecretSize+saltSize], material[advSecretSize+saltSize:]
	ephemeralShared, err := r.ephemeral.SharedSecret(primaryEphemeral)
	if err != nil {
		return node.Node{}, nil, err
	}
	key, err := BundleKey(ephemeralShared, bundleSalt)
	if err != nil {
		return node.Node{}, nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return node.Node{}, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return node.Node{}, nil, err
	}
	identity := c.Identity.Public()
	bundle := gcm.Seal(nil, bundleNonce, bytes.Join([][]byte{identity[:], primaryIdentity[:], advRandom}, nil), nil)
	identityShared, err := c.Identity.SharedSecret(primaryIdentity)
	if err != nil {
		return node.Node{}, nil, err
	}
	advSecret, err := AdvSecretFrom(ephemeralShared, identityShared, advRandom)
	if err != nil {
		return node.Node{}, nil, err
	}
	finish := iqSet(node.Node{
		Tag:   "link_code_companion_reg",
		Attrs: []node.Attr{{Key: "stage", Value: node.Text("companion_finish")}, {Key: "jid", Value: node.Address(r.JID)}},
		Children: []node.Node{
			{Tag: "link_code_pairing_wrapped_key_bundle", Bytes: bytes.Join([][]byte{bundleSalt, bundleNonce, bundle}, nil)},
			{Tag: "companion_identity_public", Bytes: identity[:]},
			{Tag: "link_code_pairing_ref", Bytes: r.ref},
		},
	})
	return finish, advSecret, nil
}
