package cert

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/PeterStoica/chatwire/internal/curve"
	"github.com/PeterStoica/chatwire/internal/signon"
)

const (
	rootSerial         = 0
	intermediateSerial = 7
	leafSerial         = 8
)

var ErrRejected = errors.New("cert: certificate chain rejected")

func WhatsAppRoot() curve.PublicKey {
	return curve.PublicKey{
		0x14, 0x23, 0x75, 0x57, 0x4d, 0x0a, 0x58, 0x71,
		0x66, 0xaa, 0xe7, 0x1e, 0xbe, 0x51, 0x64, 0x37,
		0xc4, 0xa2, 0x8b, 0x73, 0xe3, 0x69, 0x5c, 0x6c,
		0xe1, 0xf7, 0xf9, 0x54, 0x5d, 0xa8, 0xee, 0x6b,
	}
}

type Details struct {
	Serial       uint32
	IssuerSerial uint32
	Key          curve.PublicKey
	NotBefore    time.Time
	NotAfter     time.Time
}

type Validity struct {
	NotBefore time.Time
	NotAfter  time.Time
}

func Verify(raw []byte, serverStatic, root curve.PublicKey, now time.Time) error {
	chain, err := signon.ParseCertChain(raw)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrRejected, err)
	}
	intermediate, err := verifyCertificate(chain.Intermediate, root, now)
	if err != nil {
		return fmt.Errorf("%w: intermediate: %w", ErrRejected, err)
	}
	if intermediate.IssuerSerial != rootSerial {
		return fmt.Errorf("%w: intermediate issuer serial %d", ErrRejected, intermediate.IssuerSerial)
	}
	leaf, err := verifyCertificate(chain.Leaf, intermediate.Key, now)
	if err != nil {
		return fmt.Errorf("%w: leaf: %w", ErrRejected, err)
	}
	if leaf.IssuerSerial != intermediate.Serial {
		return fmt.Errorf("%w: leaf issuer %d, intermediate serial %d", ErrRejected, leaf.IssuerSerial, intermediate.Serial)
	}
	if subtle.ConstantTimeCompare(leaf.Key[:], serverStatic[:]) != 1 {
		return fmt.Errorf("%w: leaf key does not match the server static key", ErrRejected)
	}
	return nil
}

func verifyCertificate(certificate signon.Certificate, issuer curve.PublicKey, now time.Time) (Details, error) {
	if len(certificate.Signature) != curve.SignatureSize {
		return Details{}, fmt.Errorf("signature is %d bytes", len(certificate.Signature))
	}
	if err := curve.Verify(issuer, certificate.Details, curve.Signature(certificate.Signature)); err != nil {
		return Details{}, err
	}
	details, err := parseDetails(certificate.Details)
	if err != nil {
		return Details{}, err
	}
	if now.Before(details.NotBefore) || now.After(details.NotAfter) {
		return Details{}, fmt.Errorf("valid %s to %s, now %s", details.NotBefore, details.NotAfter, now)
	}
	return details, nil
}

func parseDetails(raw []byte) (Details, error) {
	wire, err := signon.ParseCertificateDetails(raw)
	if err != nil {
		return Details{}, err
	}
	key, err := curve.ParsePublicKey(wire.Key)
	if err != nil {
		return Details{}, err
	}
	notBefore, err := fromUnix(wire.NotBefore)
	if err != nil {
		return Details{}, err
	}
	notAfter, err := fromUnix(wire.NotAfter)
	if err != nil {
		return Details{}, err
	}
	return Details{Serial: wire.Serial, IssuerSerial: wire.IssuerSerial, Key: key, NotBefore: notBefore, NotAfter: notAfter}, nil
}

func fromUnix(seconds uint64) (time.Time, error) {
	if seconds > math.MaxInt64 {
		return time.Time{}, fmt.Errorf("%w: timestamp %d out of range", signon.ErrMalformed, seconds)
	}
	return time.Unix(int64(seconds), 0), nil
}

func toUnix(t time.Time) (uint64, error) {
	seconds := t.Unix()
	if seconds < 0 {
		return 0, fmt.Errorf("cert: %s is before 1970", t)
	}
	return uint64(seconds), nil
}

type Authority struct {
	root         curve.KeyPair
	intermediate curve.KeyPair
}

func NewAuthority(random io.Reader) (*Authority, error) {
	root, err := curve.NewKeyPair(random)
	if err != nil {
		return nil, err
	}
	intermediate, err := curve.NewKeyPair(random)
	if err != nil {
		return nil, err
	}
	return &Authority{root: root, intermediate: intermediate}, nil
}

func (a *Authority) Root() curve.PublicKey {
	return a.root.Public()
}

func (a *Authority) Issue(random io.Reader, leaf curve.PublicKey, validity Validity) ([]byte, error) {
	intermediate, err := sign(random, a.root, Details{
		Serial: intermediateSerial, IssuerSerial: rootSerial, Key: a.intermediate.Public(),
		NotBefore: validity.NotBefore, NotAfter: validity.NotAfter,
	})
	if err != nil {
		return nil, err
	}
	leafCertificate, err := sign(random, a.intermediate, Details{
		Serial: leafSerial, IssuerSerial: intermediateSerial, Key: leaf,
		NotBefore: validity.NotBefore, NotAfter: validity.NotAfter,
	})
	if err != nil {
		return nil, err
	}
	return signon.EncodeCertChain(signon.CertChain{Leaf: leafCertificate, Intermediate: intermediate}), nil
}

func sign(random io.Reader, issuer curve.KeyPair, details Details) (signon.Certificate, error) {
	notBefore, err := toUnix(details.NotBefore)
	if err != nil {
		return signon.Certificate{}, err
	}
	notAfter, err := toUnix(details.NotAfter)
	if err != nil {
		return signon.Certificate{}, err
	}
	raw := signon.EncodeCertificateDetails(signon.CertificateDetails{
		Serial: details.Serial, IssuerSerial: details.IssuerSerial, Key: details.Key[:], NotBefore: notBefore, NotAfter: notAfter,
	})
	signature, err := issuer.Sign(random, raw)
	if err != nil {
		return signon.Certificate{}, err
	}
	return signon.Certificate{Details: raw, Signature: signature[:]}, nil
}
