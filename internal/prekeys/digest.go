package prekeys

import (
	"crypto/sha1"
	"errors"
	"fmt"

	"github.com/PeterStoica/chatwire/internal/curve"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/signon"
)

var (
	ErrDigest   = errors.New("prekeys: malformed key digest")
	ErrMismatch = errors.New("prekeys: WhatsApp holds a different key bundle")
)

type Digest struct {
	RegistrationID uint32
	Identity       curve.PublicKey
	SignedPreKey   signon.SignedPreKey
	KeyIDs         []uint32
	Hash           [sha1.Size]byte
}

func DigestRequest() node.Node {
	return node.Node{
		Tag: "iq",
		Attrs: []node.Attr{
			{Key: attrXMLNS, Value: node.Text(namespace)}, {Key: attrType, Value: node.Text("get")},
			{Key: "to", Value: server()}, {Key: "id", Value: node.Value{}},
		},
		Children: []node.Node{{Tag: "digest"}},
	}
}

func NeedsUpload(reply node.Node) bool {
	kind, _ := reply.Attr(attrType).Text()
	failure, _ := reply.Child("error")
	code, _ := failure.Attr("code").Text()
	return reply.Tag == "iq" && kind == "error" && code == "404"
}

func ParseDigest(reply node.Node) (Digest, error) {
	digest, ok := reply.Child("digest")
	if !ok {
		return Digest{}, fmt.Errorf("%w: no digest in %s", ErrDigest, reply)
	}
	registration, keyType, identity := fieldBytes(digest, "registration"), fieldBytes(digest, attrType), fieldBytes(digest, "identity")
	skeyID, skey, signature, hash := fieldBytes(digest, "skey", "id"), fieldBytes(digest, "skey", "value"), fieldBytes(digest, "skey", "signature"), fieldBytes(digest, "hash")
	if len(registration) != 4 || len(keyType) != 1 || len(identity) != curve.KeySize || len(skeyID) != 3 ||
		len(skey) != curve.KeySize || len(signature) != curve.SignatureSize || len(hash) != sha1.Size {
		return Digest{}, fmt.Errorf("%w: field sizes in %s", ErrDigest, reply)
	}
	d := Digest{
		RegistrationID: bigEndian(registration),
		Identity:       curve.PublicKey(identity),
		SignedPreKey:   signon.SignedPreKey{ID: bigEndian(skeyID), Key: curve.PublicKey(skey), Signature: curve.Signature(signature)},
		Hash:           [sha1.Size]byte(hash),
	}
	list, _ := digest.Child("list")
	for _, id := range list.Children {
		if len(id.Bytes) != 3 {
			return Digest{}, fmt.Errorf("%w: key id of %d bytes", ErrDigest, len(id.Bytes))
		}
		d.KeyIDs = append(d.KeyIDs, bigEndian(id.Bytes))
	}
	return d, nil
}

func bigEndian(b []byte) uint32 {
	var v uint32
	for _, c := range b {
		v = v<<8 | uint32(c)
	}
	return v
}

func (d Digest) Verify(registration signon.Registration, lookup func(id uint32) (curve.PublicKey, bool)) error {
	if d.RegistrationID != registration.RegistrationID {
		return fmt.Errorf("%w: registration id %d, ours %d", ErrMismatch, d.RegistrationID, registration.RegistrationID)
	}
	identity, signed := registration.Identity, registration.SignedPreKey
	hash := sha1.New()
	hash.Write(identity[:])
	hash.Write(signed.Key[:])
	hash.Write(signed.Signature[:])
	for _, id := range d.KeyIDs {
		key, ok := lookup(id)
		if !ok {
			return fmt.Errorf("%w: WhatsApp lists prekey %d, which we do not hold", ErrMismatch, id)
		}
		hash.Write(key[:])
	}
	if [sha1.Size]byte(hash.Sum(nil)) != d.Hash {
		return fmt.Errorf("%w: hash differs", ErrMismatch)
	}
	return nil
}
