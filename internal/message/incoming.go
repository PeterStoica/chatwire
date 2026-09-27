package message

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/PeterStoica/chatwire/internal/node"
)

var ErrIncoming = errors.New("message: malformed incoming message")

type Enc struct {
	Type       string
	Ciphertext []byte
	Retry      int
	MediaType  string
}

type Edit int

const (
	EditNone         Edit = -1
	EditMessage      Edit = 1
	EditPin          Edit = 2
	EditNewsletter   Edit = 3
	EditSenderRevoke Edit = 7
	EditAdminRevoke  Edit = 8
)

type Incoming struct {
	ID             string
	Edit           Edit
	From           node.JID
	Category       string
	Chat           node.JID
	Author         node.JID
	Timestamp      time.Time
	PushName       string
	Type           string
	Encs           []Enc
	DeviceIdentity []byte
	Pairs          map[node.JID]node.JID
}

func ParseIncoming(me func(node.JID) bool, n node.Node) (Incoming, error) {
	if n.Tag != tagMessage {
		return Incoming{}, fmt.Errorf("%w: <%s>", ErrIncoming, n.Tag)
	}
	if _, ok := n.Child("plaintext"); ok {
		return Incoming{}, fmt.Errorf("%w: plaintext in an end-to-end encrypted message", ErrIncoming)
	}
	id, _ := n.Attr("id").Text()
	stamp, err := strconv.ParseInt(text(n, "t"), 10, 64)
	from, ok := n.Attr("from").JID()
	if id == "" || err != nil || !ok {
		return Incoming{}, fmt.Errorf("%w: id, t and from are required in %s", ErrIncoming, n)
	}
	in := Incoming{ID: id, Edit: EditNone, From: from, Category: text(n, "category"), Timestamp: time.Unix(stamp, 0), PushName: text(n, "notify"), Type: text(n, attrType)}
	if raw, ok := n.Attr("edit").Text(); ok {
		edit, err := strconv.Atoi(raw)
		if err != nil {
			return Incoming{}, fmt.Errorf("%w: edit %q", ErrIncoming, raw)
		}
		in.Edit = Edit(edit)
	}
	if in.Chat, in.Author, err = addressing(me, n, from); err != nil {
		return Incoming{}, err
	}
	in.Pairs = Pairs(n)
	for _, child := range n.Children {
		switch child.Tag {
		case "enc":
			enc, err := parseEnc(child)
			if err != nil {
				return Incoming{}, err
			}
			in.Encs = append(in.Encs, enc)
		case tagIdentity:
			in.DeviceIdentity = child.Bytes
		}
	}
	return in, nil
}

func addressing(me func(node.JID) bool, n node.Node, from node.JID) (node.JID, node.JID, error) {
	participant, hasParticipant := n.Attr(attrParticipant).JID()
	recipient, hasRecipient := n.Attr("recipient").JID()
	switch from.Server {
	case node.ServerGroup, node.ServerBroadcast:
		if !hasParticipant {
			return node.JID{}, node.JID{}, fmt.Errorf("%w: group message without a participant", ErrIncoming)
		}
		if participant.Server == node.ServerHosted || participant.Server == node.ServerHostedLID {
			return node.JID{}, node.JID{}, fmt.Errorf("%w: hosted participant in a group", ErrIncoming)
		}
		if hasRecipient && !me(participant) {
			return node.JID{}, node.JID{}, fmt.Errorf("%w: recipient from a device that is not ours", ErrIncoming)
		}
		return from, participant, nil
	case node.ServerUser, node.ServerLID, node.ServerHosted, node.ServerHostedLID:
		chat := from.WithoutDevice()
		if hasRecipient {
			if !me(from) {
				return node.JID{}, node.JID{}, fmt.Errorf("%w: recipient from a device that is not ours", ErrIncoming)
			}
			chat = recipient.WithoutDevice()
		}
		return chat, from, nil
	default:
		return node.JID{}, node.JID{}, fmt.Errorf("%w: from %s", ErrIncoming, from)
	}
}

func parseEnc(n node.Node) (Enc, error) {
	enc := Enc{Type: text(n, attrType), Ciphertext: n.Bytes, MediaType: text(n, "mediatype")}
	switch enc.Type {
	case "msg", "pkmsg", "skmsg", "msmsg":
	default:
		return Enc{}, fmt.Errorf("%w: enc type %q", ErrIncoming, enc.Type)
	}
	if raw := text(n, "count"); raw != "" {
		count, err := strconv.Atoi(raw)
		if err != nil {
			return Enc{}, fmt.Errorf("%w: retry count %q", ErrIncoming, raw)
		}
		enc.Retry = count
	}
	return enc, nil
}

func text(n node.Node, key string) string {
	value, _ := n.Attr(key).Text()
	return value
}

func Pairs(n node.Node) map[node.JID]node.JID {
	var out map[node.JID]node.JID
	for _, names := range [][2]string{{"from", "sender_"}, {"recipient", "peer_recipient_"}, {attrParticipant, "participant_"}} {
		a, ok := n.Attr(names[0]).JID()
		if !ok {
			continue
		}
		for _, kind := range []string{"lid", "pn"} {
			b, ok := n.Attr(names[1] + kind).JID()
			if !ok {
				continue
			}
			lid, pn, ok := pairOf(a, b)
			if !ok {
				continue
			}
			if out == nil {
				out = map[node.JID]node.JID{}
			}
			out[lid] = pn
		}
	}
	return out
}

func pairOf(a, b node.JID) (node.JID, node.JID, bool) {
	a, b = a.WithoutDevice(), b.WithoutDevice()
	switch {
	case a.Server == node.ServerLID && b.Server == node.ServerUser:
		return a, b, true
	case a.Server == node.ServerUser && b.Server == node.ServerLID:
		return b, a, true
	default:
		return node.JID{}, node.JID{}, false
	}
}
