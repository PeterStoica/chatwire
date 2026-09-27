package node

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
)

const (
	packedMax    = 127
	binary8Max   = 1<<8 - 1
	binary20Max  = 1<<20 - 1
	list8Max     = 1<<8 - 1
	maxBinary    = 1 << 24
	initialFrame = 512
)

func (d Dictionary) Marshal(n Node) ([]byte, error) {
	enc := &encoder{dict: d, out: make([]byte, 1, initialFrame)}
	if err := enc.node(n); err != nil {
		return nil, err
	}
	return enc.out, nil
}

type encoder struct {
	dict Dictionary
	out  []byte
}

func (e *encoder) node(n Node) error {
	kinds := n.contentKinds()
	if kinds > 1 {
		return fmt.Errorf("%w: <%s>", ErrMixedContent, n.Tag)
	}
	attrs := make([]Attr, 0, len(n.Attrs))
	seen := make(map[string]struct{}, len(n.Attrs))
	for _, a := range n.Attrs {
		if _, duplicate := seen[a.Key]; duplicate {
			return fmt.Errorf("%w: attribute %q twice in <%s>", ErrUnencodable, a.Key, n.Tag)
		}
		seen[a.Key] = struct{}{}
		if !a.Value.IsZero() {
			attrs = append(attrs, a)
		}
	}
	if err := e.listStart(1 + 2*len(attrs) + kinds); err != nil {
		return err
	}
	if err := e.text(n.Tag); err != nil {
		return err
	}
	for _, a := range attrs {
		if err := e.text(a.Key); err != nil {
			return err
		}
		if err := e.value(a.Value); err != nil {
			return err
		}
	}
	return e.content(n)
}

func (e *encoder) content(n Node) error {
	switch {
	case len(n.Children) > 0:
		if err := e.listStart(len(n.Children)); err != nil {
			return err
		}
		for _, child := range n.Children {
			if err := e.node(child); err != nil {
				return err
			}
		}
		return nil
	case n.Bytes != nil:
		return e.binary(n.Bytes)
	default:
		return nil
	}
}

func (e *encoder) value(v Value) error {
	switch v.kind {
	case kindDevice:
		return e.device(v.jid)
	case kindJID:
		return e.jid(v.jid)
	default:
		return e.text(v.text)
	}
}

func (e *encoder) device(j JID) error {
	switch j.Server {
	case ServerUser, ServerLID, ServerHosted, ServerHostedLID:
		e.out = append(e.out, tagADJID, j.domain(), j.Device)
		return e.text(j.User)
	default:
		return fmt.Errorf("%w: %s is not addressed by device", ErrUnencodable, j)
	}
}

func (e *encoder) text(s string) error {
	if index, ok := e.dict.singleIndex[s]; ok {
		e.out = append(e.out, index)
		return nil
	}
	if index, ok := e.dict.doubleIndex[s]; ok {
		e.out = append(e.out, tagDictionary0+index.table, index.index)
		return nil
	}
	if packable(s, nibbleDigits) {
		e.pack(s, tagNibble8, nibbleDigits)
		return nil
	}
	if packable(s, hexDigits) {
		e.pack(s, tagHex8, hexDigits)
		return nil
	}
	return e.binary([]byte(s))
}

func (e *encoder) jid(j JID) error {
	if err := j.encodable(); err != nil {
		return err
	}
	if j.addressedByDevice() {
		e.out = append(e.out, tagADJID, j.domain(), j.Device)
		return e.text(j.User)
	}
	e.out = append(e.out, tagJIDPair)
	if j.User == "" {
		e.out = append(e.out, tagListEmpty)
	} else if err := e.text(j.User); err != nil {
		return err
	}
	return e.text(string(j.Server))
}

func (e *encoder) binary(raw []byte) error {
	size := len(raw)
	if size > maxBinary {
		return fmt.Errorf("%w: %d bytes of binary content", ErrUnencodable, size)
	}
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(size))
	switch {
	case size <= binary8Max:
		e.out = append(e.out, tagBinary8, length[3])
	case size <= binary20Max:
		e.out = append(e.out, tagBinary20, length[1], length[2], length[3])
	default:
		e.out = append(e.out, tagBinary32, length[0], length[1], length[2], length[3])
	}
	e.out = append(e.out, raw...)
	return nil
}

func (e *encoder) listStart(size int) error {
	if size > math.MaxUint16 {
		return fmt.Errorf("%w: list of %d", ErrUnencodable, size)
	}
	var length [2]byte
	binary.BigEndian.PutUint16(length[:], uint16(size))
	switch {
	case size == 0:
		e.out = append(e.out, tagListEmpty)
	case size <= list8Max:
		e.out = append(e.out, tagList8, length[1])
	default:
		e.out = append(e.out, tagList16, length[0], length[1])
	}
	return nil
}

func (e *encoder) pack(s string, tag byte, alphabet string) {
	e.out = append(e.out, tag, 0)
	head := len(e.out) - 1
	var pairs byte
	for i := 0; i < len(s); i += 2 {
		low := byte(paddingNibble)
		if i+1 < len(s) {
			low = byte(strings.IndexByte(alphabet, s[i+1]))
		}
		e.out = append(e.out, byte(strings.IndexByte(alphabet, s[i]))<<4|low)
		pairs++
	}
	if len(s)%2 == 1 {
		pairs |= 0x80
	}
	e.out[head] = pairs
}

func packable(s, alphabet string) bool {
	if s == "" || len(s) > packedMax {
		return false
	}
	for i := range len(s) {
		if strings.IndexByte(alphabet, s[i]) < 0 {
			return false
		}
	}
	return true
}
