package signon

import (
	"crypto/md5"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"google.golang.org/protobuf/encoding/protowire"

	"github.com/PeterStoica/chatwire/internal/curve"
)

var ErrMalformed = errors.New("wa6: malformed message")

type Version struct {
	Primary   uint32
	Secondary uint32
	Tertiary  uint32
}

func ParseVersion(text string) (Version, error) {
	parts := strings.Split(text, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("wa6: version %q is not three dot-separated numbers", text)
	}
	var numbers [3]uint32
	for i, part := range parts {
		n, err := strconv.ParseUint(part, 10, 32)
		if err != nil {
			return Version{}, fmt.Errorf("wa6: version %q: %w", text, err)
		}
		numbers[i] = uint32(n)
	}
	return Version{Primary: numbers[0], Secondary: numbers[1], Tertiary: numbers[2]}, nil
}

func (v Version) String() string {
	return strconv.FormatUint(uint64(v.Primary), 10) + "." +
		strconv.FormatUint(uint64(v.Secondary), 10) + "." +
		strconv.FormatUint(uint64(v.Tertiary), 10)
}

type ServerHello struct {
	Ephemeral []byte
	Static    []byte
	Payload   []byte
}

type ClientFinish struct {
	Static  []byte
	Payload []byte
}

type Certificate struct {
	Details   []byte
	Signature []byte
}

type CertChain struct {
	Leaf         Certificate
	Intermediate Certificate
}

type CertificateDetails struct {
	Serial       uint32
	IssuerSerial uint32
	Key          []byte
	NotBefore    uint64
	NotAfter     uint64
}

type SignedPreKey struct {
	ID        uint32
	Key       curve.PublicKey
	Signature curve.Signature
}

type DeviceProps struct {
	OS      string
	Version Version
}

type Registration struct {
	RegistrationID uint32
	Identity       curve.PublicKey
	SignedPreKey   SignedPreKey
	Device         DeviceProps
}

const (
	platformWeb           = 14
	releaseChannelRelease = 0
	webSubPlatformBrowser = 0
	connectTypeWifi       = 1
	connectReasonUser     = 1
	historyQuotaMB        = 10240
)

func EncodeClientHello(ephemeral []byte) []byte {
	return appendBytes(nil, 2, appendBytes(nil, 1, ephemeral))
}

func ParseClientHello(raw []byte) ([]byte, error) {
	hello, err := message(raw, 2)
	if err != nil {
		return nil, err
	}
	ephemeral, err := message(hello, 1)
	if err != nil {
		return nil, err
	}
	if len(ephemeral) == 0 {
		return nil, fmt.Errorf("%w: client hello has no ephemeral key", ErrMalformed)
	}
	return ephemeral, nil
}

func EncodeServerHello(hello ServerHello) []byte {
	inner := appendBytes(nil, 1, hello.Ephemeral)
	inner = appendBytes(inner, 2, hello.Static)
	inner = appendBytes(inner, 3, hello.Payload)
	return appendBytes(nil, 3, inner)
}

func ParseServerHello(raw []byte) (ServerHello, error) {
	hello, err := message(raw, 3)
	if err != nil {
		return ServerHello{}, err
	}
	var out ServerHello
	err = walk(hello, func(f field) error {
		switch f.number {
		case 1:
			out.Ephemeral = f.bytes
		case 2:
			out.Static = f.bytes
		case 3:
			out.Payload = f.bytes
		default:
		}
		return nil
	})
	if err != nil {
		return ServerHello{}, err
	}
	if len(out.Ephemeral) == 0 || len(out.Static) == 0 || len(out.Payload) == 0 {
		return ServerHello{}, fmt.Errorf("%w: server hello is missing ephemeral, static or payload", ErrMalformed)
	}
	return out, nil
}

func EncodeClientFinish(finish ClientFinish) []byte {
	inner := appendBytes(nil, 1, finish.Static)
	inner = appendBytes(inner, 2, finish.Payload)
	return appendBytes(nil, 4, inner)
}

func ParseClientFinish(raw []byte) (ClientFinish, error) {
	inner, err := message(raw, 4)
	if err != nil {
		return ClientFinish{}, err
	}
	var out ClientFinish
	err = walk(inner, func(f field) error {
		switch f.number {
		case 1:
			out.Static = f.bytes
		case 2:
			out.Payload = f.bytes
		default:
		}
		return nil
	})
	if err != nil {
		return ClientFinish{}, err
	}
	if len(out.Static) == 0 || len(out.Payload) == 0 {
		return ClientFinish{}, fmt.Errorf("%w: client finish is missing static or payload", ErrMalformed)
	}
	return out, nil
}

func EncodeCertChain(chain CertChain) []byte {
	out := appendBytes(nil, 1, encodeCertificate(chain.Leaf))
	return appendBytes(out, 2, encodeCertificate(chain.Intermediate))
}

func ParseCertChain(raw []byte) (CertChain, error) {
	var chain CertChain
	err := walk(raw, func(f field) error {
		var target *Certificate
		switch f.number {
		case 1:
			target = &chain.Leaf
		case 2:
			target = &chain.Intermediate
		default:
			return nil
		}
		certificate, err := parseCertificate(f.bytes)
		*target = certificate
		return err
	})
	return chain, err
}

func EncodeCertificateDetails(details CertificateDetails) []byte {
	out := appendVarint(nil, 1, uint64(details.Serial))
	out = appendVarint(out, 2, uint64(details.IssuerSerial))
	out = appendBytes(out, 3, details.Key)
	out = appendVarint(out, 4, details.NotBefore)
	return appendVarint(out, 5, details.NotAfter)
}

func ParseCertificateDetails(raw []byte) (CertificateDetails, error) {
	var details CertificateDetails
	err := walk(raw, func(f field) error {
		var err error
		switch f.number {
		case 1:
			details.Serial, err = toUint32(f.varint)
		case 2:
			details.IssuerSerial, err = toUint32(f.varint)
		case 3:
			details.Key = f.bytes
		case 4:
			details.NotBefore = f.varint
		case 5:
			details.NotAfter = f.varint
		default:
		}
		return err
	})
	return details, err
}

func EncodeRegistration(version Version, registration Registration) []byte {
	osVersion := registration.Device.Version.String()
	userAgent := appendVarint(nil, 1, platformWeb)
	userAgent = appendBytes(userAgent, 2, encodeVersion(version))
	userAgent = appendString(userAgent, 3, "000")
	userAgent = appendString(userAgent, 4, "000")
	userAgent = appendString(userAgent, 5, osVersion)
	userAgent = appendString(userAgent, 6, "")
	userAgent = appendString(userAgent, 7, "Desktop")
	userAgent = appendString(userAgent, 8, osVersion)
	userAgent = appendVarint(userAgent, 10, releaseChannelRelease)
	userAgent = appendString(userAgent, 11, "en")
	userAgent = appendString(userAgent, 12, "US")

	buildHash := md5.Sum([]byte(version.String()))
	preKey := registration.SignedPreKey
	pairing := appendBytes(nil, 1, bigEndian(registration.RegistrationID, 4))
	pairing = appendBytes(pairing, 2, []byte{curve.KeyType})
	pairing = appendBytes(pairing, 3, registration.Identity[:])
	pairing = appendBytes(pairing, 4, bigEndian(preKey.ID, 3))
	pairing = appendBytes(pairing, 5, preKey.Key[:])
	pairing = appendBytes(pairing, 6, preKey.Signature[:])
	pairing = appendBytes(pairing, 7, buildHash[:])
	pairing = appendBytes(pairing, 8, encodeDeviceProps(registration.Device))

	payload := appendVarint(nil, 3, 0)
	payload = appendBytes(payload, 5, userAgent)
	payload = appendBytes(payload, 6, appendVarint(nil, 4, webSubPlatformBrowser))
	payload = appendVarint(payload, 12, connectTypeWifi)
	payload = appendVarint(payload, 13, connectReasonUser)
	payload = appendBytes(payload, 19, pairing)
	return appendVarint(payload, 33, 0)
}

type Login struct {
	Username uint64
	Device   uint32
	OS       Version
}

func EncodeLogin(version Version, login Login) []byte {
	osVersion := login.OS.String()
	userAgent := appendVarint(nil, 1, platformWeb)
	userAgent = appendBytes(userAgent, 2, encodeVersion(version))
	userAgent = appendString(userAgent, 3, "000")
	userAgent = appendString(userAgent, 4, "000")
	userAgent = appendString(userAgent, 5, osVersion)
	userAgent = appendString(userAgent, 6, "")
	userAgent = appendString(userAgent, 7, "Desktop")
	userAgent = appendString(userAgent, 8, osVersion)
	userAgent = appendVarint(userAgent, 10, releaseChannelRelease)
	userAgent = appendString(userAgent, 11, "en")
	userAgent = appendString(userAgent, 12, "US")

	payload := appendVarint(nil, 1, login.Username)
	payload = appendVarint(payload, 3, 1)
	payload = appendBytes(payload, 5, userAgent)
	payload = appendBytes(payload, 6, appendVarint(nil, 4, webSubPlatformBrowser))
	payload = appendVarint(payload, 12, connectTypeWifi)
	payload = appendVarint(payload, 13, connectReasonUser)
	payload = appendVarint(payload, 18, uint64(login.Device))
	payload = appendVarint(payload, 24, 1)
	payload = appendVarint(payload, 33, 1)
	return appendVarint(payload, 41, 1)
}

func encodeDeviceProps(props DeviceProps) []byte {
	history := appendVarint(nil, 3, historyQuotaMB)
	history = appendVarint(history, 4, 1)
	out := appendString(nil, 1, props.OS)
	out = appendBytes(out, 2, encodeVersion(props.Version))
	out = appendVarint(out, 3, 0)
	out = appendVarint(out, 4, 0)
	return appendBytes(out, 5, history)
}

func encodeVersion(v Version) []byte {
	out := appendVarint(nil, 1, uint64(v.Primary))
	out = appendVarint(out, 2, uint64(v.Secondary))
	return appendVarint(out, 3, uint64(v.Tertiary))
}

func encodeCertificate(certificate Certificate) []byte {
	return appendBytes(appendBytes(nil, 1, certificate.Details), 2, certificate.Signature)
}

func parseCertificate(raw []byte) (Certificate, error) {
	var certificate Certificate
	err := walk(raw, func(f field) error {
		switch f.number {
		case 1:
			certificate.Details = f.bytes
		case 2:
			certificate.Signature = f.bytes
		default:
		}
		return nil
	})
	return certificate, err
}

func toUint32(value uint64) (uint32, error) {
	if value > math.MaxUint32 {
		return 0, fmt.Errorf("%w: %d overflows uint32", ErrMalformed, value)
	}
	return uint32(value), nil
}

type field struct {
	number protowire.Number
	bytes  []byte
	varint uint64
}

func message(raw []byte, number protowire.Number) ([]byte, error) {
	var found []byte
	err := walk(raw, func(f field) error {
		if f.number == number {
			found = f.bytes
		}
		return nil
	})
	return found, err
}

func walk(raw []byte, visit func(field) error) error {
	for len(raw) > 0 {
		number, kind, n := protowire.ConsumeTag(raw)
		if n < 1 {
			return fmt.Errorf("%w: %w", ErrMalformed, protowire.ParseError(n))
		}
		raw = raw[n:]
		f := field{number: number}
		switch kind {
		case protowire.BytesType:
			f.bytes, n = protowire.ConsumeBytes(raw)
		case protowire.VarintType:
			f.varint, n = protowire.ConsumeVarint(raw)
		default:
			n = protowire.ConsumeFieldValue(number, kind, raw)
		}
		if n < 1 {
			return fmt.Errorf("%w: %w", ErrMalformed, protowire.ParseError(n))
		}
		raw = raw[n:]
		if err := visit(f); err != nil {
			return err
		}
	}
	return nil
}

func appendBytes(b []byte, number protowire.Number, value []byte) []byte {
	b = protowire.AppendTag(b, number, protowire.BytesType)
	return protowire.AppendBytes(b, value)
}

func appendString(b []byte, number protowire.Number, value string) []byte {
	b = protowire.AppendTag(b, number, protowire.BytesType)
	return protowire.AppendString(b, value)
}

func appendVarint(b []byte, number protowire.Number, value uint64) []byte {
	b = protowire.AppendTag(b, number, protowire.VarintType)
	return protowire.AppendVarint(b, value)
}

func bigEndian(value uint32, size int) []byte {
	return binary.BigEndian.AppendUint32(nil, value)[4-size:]
}
