package node

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode/utf8"
)

const (
	tagListEmpty   = 0
	tagDictionary0 = 236
	tagDictionary3 = 239
	tagInteropJID  = 245
	tagFBJID       = 246
	tagADJID       = 247
	tagList8       = 248
	tagList16      = 249
	tagJIDPair     = 250
	tagHex8        = 251
	tagBinary8     = 252
	tagBinary20    = 253
	tagBinary32    = 254
	tagNibble8     = 255

	flagCompressed = 2
	maxInflated    = 64 << 20
	sizeHint       = 16
	maxDepth       = 256
	paddingNibble  = 0x0f
	nibbleDigits   = "0123456789-."
	hexDigits      = "0123456789ABCDEF"
)

func (d Dictionary) Unmarshal(frame []byte) (Node, error) {
	if len(frame) == 0 {
		return Node{}, ErrTruncated
	}
	body, err := inflate(frame)
	if err != nil {
		return Node{}, err
	}
	dec := &decoder{dict: d, data: body}
	return dec.node()
}

func inflate(frame []byte) ([]byte, error) {
	if frame[0]&flagCompressed == 0 {
		return frame[1:], nil
	}
	reader, err := zlib.NewReader(bytes.NewReader(frame[1:]))
	if err != nil {
		return nil, fmt.Errorf("node: zlib: %w", err)
	}
	body, err := io.ReadAll(io.LimitReader(reader, maxInflated+1))
	if err != nil {
		return nil, fmt.Errorf("node: inflate: %w", err)
	}
	if len(body) > maxInflated {
		return nil, fmt.Errorf("%w: over %d bytes", ErrTooLarge, maxInflated)
	}
	return body, nil
}

type decoder struct {
	dict  Dictionary
	data  []byte
	pos   int
	depth int
}

func (d *decoder) node() (Node, error) {
	if d.depth >= maxDepth {
		return Node{}, fmt.Errorf("%w: nested deeper than %d at %d", ErrInvalidNode, maxDepth, d.pos)
	}
	d.depth++
	defer func() { d.depth-- }()
	listTag, err := d.byte()
	if err != nil {
		return Node{}, err
	}
	size, err := d.listSize(listTag)
	if err != nil {
		return Node{}, err
	}
	if size == 0 {
		return Node{}, fmt.Errorf("%w: empty list at %d", ErrInvalidNode, d.pos)
	}
	tag, err := d.text()
	if err != nil {
		return Node{}, err
	}
	out := Node{Tag: tag}
	if err := d.attributes(&out, (size-1)/2); err != nil {
		return Node{}, err
	}
	if size%2 == 1 {
		return out, nil
	}
	if err := d.content(&out); err != nil {
		return Node{}, err
	}
	return out, nil
}

func (d *decoder) attributes(out *Node, pairs int) error {
	attrs := make([]Attr, 0, min(pairs, sizeHint))
	positions := make(map[string]int, min(pairs, sizeHint))
	for range pairs {
		key, err := d.text()
		if err != nil {
			return err
		}
		value, err := d.value()
		if err != nil {
			return err
		}
		if at, seen := positions[key]; seen {
			attrs[at].Value = value
			continue
		}
		positions[key] = len(attrs)
		attrs = append(attrs, Attr{Key: key, Value: value})
	}
	attrs = slices.DeleteFunc(attrs, func(a Attr) bool { return a.Value.IsZero() })
	if len(attrs) > 0 {
		out.Attrs = attrs
	}
	return nil
}

func (d *decoder) content(out *Node) error {
	tag, err := d.byte()
	if err != nil {
		return err
	}
	switch tag {
	case tagListEmpty:
		return nil
	case tagList8, tagList16:
		size, err := d.listSize(tag)
		if err != nil {
			return err
		}
		if size > 0 {
			out.Children = make([]Node, 0, min(size, sizeHint))
		}
		for range size {
			child, err := d.node()
			if err != nil {
				return err
			}
			out.Children = append(out.Children, child)
		}
		return nil
	case tagBinary8, tagBinary20, tagBinary32:
		raw, err := d.binary(tag)
		out.Bytes = bytes.Clone(raw)
		return err
	default:
		value, err := d.tagged(tag)
		if err != nil {
			return err
		}
		out.Bytes = []byte(value.String())
		return nil
	}
}

func (d *decoder) text() (string, error) {
	at := d.pos
	value, err := d.value()
	if err != nil {
		return "", err
	}
	text, ok := value.Text()
	if !ok {
		return "", fmt.Errorf("%w: expected text at %d", ErrInvalidToken, at)
	}
	return text, nil
}

func (d *decoder) optionalText() (string, error) {
	if d.pos < len(d.data) && d.data[d.pos] == tagListEmpty {
		d.pos++
		return "", nil
	}
	return d.text()
}

func (d *decoder) value() (Value, error) {
	tag, err := d.byte()
	if err != nil {
		return Value{}, err
	}
	if tag == tagListEmpty {
		return Value{}, nil
	}
	return d.tagged(tag)
}

func (d *decoder) tagged(tag byte) (Value, error) {
	switch {
	case tag == tagJIDPair:
		jid, err := d.jidPair()
		return Address(jid), err
	case tag == tagADJID:
		jid, err := d.adJID()
		return Address(jid), err
	case tag >= tagDictionary0 && tag <= tagDictionary3:
		text, err := d.doubleToken(tag)
		return Text(text), err
	case tag == tagBinary8 || tag == tagBinary20 || tag == tagBinary32:
		raw, err := d.binary(tag)
		return Text(string(raw)), err
	case tag == tagNibble8 || tag == tagHex8:
		text, err := d.packed(tag)
		return Text(text), err
	case tag == tagList8 || tag == tagList16:
		return Value{}, fmt.Errorf("%w at %d", ErrUnexpectedList, d.pos)
	case tag == tagFBJID || tag == tagInteropJID:
		return Value{}, fmt.Errorf("%w: jid kind %d is not supported", ErrInvalidToken, tag)
	case int(tag) < len(d.dict.single):
		return Text(d.dict.single[tag]), nil
	default:
		return Value{}, fmt.Errorf("%w: %d at %d", ErrInvalidToken, tag, d.pos)
	}
}

func (d *decoder) doubleToken(tag byte) (string, error) {
	index, err := d.byte()
	if err != nil {
		return "", err
	}
	table := int(tag - tagDictionary0)
	if table >= len(d.dict.double) || int(index) >= len(d.dict.double[table]) {
		return "", fmt.Errorf("%w: double %d/%d", ErrInvalidToken, table, index)
	}
	return d.dict.double[table][index], nil
}

func (d *decoder) jidPair() (JID, error) {
	user, err := d.optionalText()
	if err != nil {
		return JID{}, err
	}
	server, err := d.text()
	if err != nil {
		return JID{}, err
	}
	if server == "" {
		return JID{}, fmt.Errorf("%w at %d", ErrEmptyJIDServer, d.pos)
	}
	return JID{User: user, Server: Server(server)}, nil
}

func (d *decoder) adJID() (JID, error) {
	domain, err := d.byte()
	if err != nil {
		return JID{}, err
	}
	device, err := d.byte()
	if err != nil {
		return JID{}, err
	}
	user, err := d.text()
	if err != nil {
		return JID{}, err
	}
	return jidFromDomain(user, domain, device)
}

func (d *decoder) packed(tag byte) (string, error) {
	head, err := d.byte()
	if err != nil {
		return "", err
	}
	count := int(head & 0x7f)
	odd := head&0x80 != 0
	if count == 0 && odd {
		return "", fmt.Errorf("%w: odd packed string with no bytes at %d", ErrInvalidToken, d.pos)
	}
	raw, err := d.take(count)
	if err != nil {
		return "", err
	}
	length := 2 * count
	if odd {
		length--
	}
	var out strings.Builder
	out.Grow(length)
	for i := range length {
		nibble := raw[i/2] >> 4
		if i%2 == 1 {
			nibble = raw[i/2] & 0x0f
		}
		out.WriteString(unpack(tag, nibble))
	}
	return out.String(), nil
}

func unpack(tag, nibble byte) string {
	alphabet := nibbleDigits
	if tag == tagHex8 {
		alphabet = hexDigits
	}
	if int(nibble) < len(alphabet) {
		return alphabet[nibble : nibble+1]
	}
	return string(utf8.RuneError)
}

func (d *decoder) binary(tag byte) ([]byte, error) {
	width := 4
	switch tag {
	case tagBinary8:
		width = 1
	case tagBinary20:
		width = 3
	default:
	}
	size, err := d.length(width)
	if err != nil {
		return nil, err
	}
	if tag == tagBinary20 {
		size &= binary20Max
	}
	return d.take(size)
}

func (d *decoder) listSize(tag byte) (int, error) {
	switch tag {
	case tagListEmpty:
		return 0, nil
	case tagList8:
		return d.length(1)
	case tagList16:
		return d.length(2)
	default:
		return 0, fmt.Errorf("%w: list tag %d at %d", ErrInvalidNode, tag, d.pos)
	}
}

func (d *decoder) length(width int) (int, error) {
	raw, err := d.take(width)
	if err != nil {
		return 0, err
	}
	size := 0
	for _, b := range raw {
		size = size<<8 | int(b)
	}
	return size, nil
}

func (d *decoder) byte() (byte, error) {
	raw, err := d.take(1)
	if err != nil {
		return 0, err
	}
	return raw[0], nil
}

func (d *decoder) take(n int) ([]byte, error) {
	if n < 0 || d.pos+n > len(d.data) {
		return nil, fmt.Errorf("%w: need %d bytes at %d of %d", ErrTruncated, n, d.pos, len(d.data))
	}
	out := d.data[d.pos : d.pos+n : d.pos+n]
	d.pos += n
	return out, nil
}
