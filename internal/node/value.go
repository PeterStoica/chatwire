package node

type valueKind uint8

const (
	kindNone valueKind = iota
	kindText
	kindJID
	kindDevice
)

type Value struct {
	kind valueKind
	text string
	jid  JID
}

func Text(s string) Value {
	return Value{kind: kindText, text: s}
}

func Address(j JID) Value {
	return Value{kind: kindJID, jid: j}
}

func Device(j JID) Value {
	return Value{kind: kindDevice, jid: j}
}

func (v Value) IsZero() bool {
	return v.kind == kindNone
}

func (v Value) Text() (string, bool) {
	return v.text, v.kind == kindText
}

func (v Value) JID() (JID, bool) {
	return v.jid, v.kind == kindJID || v.kind == kindDevice
}

func (v Value) String() string {
	if jid, ok := v.JID(); ok {
		return jid.String()
	}
	return v.text
}
