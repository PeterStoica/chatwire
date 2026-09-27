package message

import (
	"fmt"
	"strconv"
	"time"

	"github.com/PeterStoica/chatwire/internal/node"
)

type Ack int

const (
	AckDelivered Ack = iota + 1
	AckRead
	AckPlayed
	AckInactive
	AckGone
	AckPeer
)

type Receipt struct {
	From        node.JID
	Participant node.JID
	Recipient   node.JID
	Ack         Ack
	Self        bool
	IDs         []string
	Time        time.Time
}

func ackOf(kind string) (Ack, bool, bool) {
	switch kind {
	case "", "delivery", "sender":
		return AckDelivered, false, true
	case "read":
		return AckRead, false, true
	case "read-self":
		return AckRead, true, true
	case "played":
		return AckPlayed, false, true
	case "played-self":
		return AckPlayed, true, true
	case "inactive":
		return AckInactive, false, true
	case "server-error":
		return AckGone, false, true
	case "peer_msg":
		return AckPeer, false, true
	default:
		return AckDelivered, false, false
	}
}

func ParseReceipt(n node.Node) (Receipt, error) {
	id, _ := n.Attr("id").Text()
	from, ok := n.Attr("from").JID()
	if n.Tag != tagReceipt || id == "" || !ok {
		return Receipt{}, fmt.Errorf("%w: %s", ErrIncoming, n)
	}
	if _, aggregated := n.Child("participants"); aggregated {
		return Receipt{}, fmt.Errorf("%w: aggregated receipts are not read", ErrIncoming)
	}
	kind, _ := n.Attr(attrType).Text()
	ack, self, _ := ackOf(kind)
	r := Receipt{From: from, Ack: ack, Self: self}
	r.Participant, _ = n.Attr(attrParticipant).JID()
	r.Recipient, _ = n.Attr("recipient").JID()
	if raw, ok := n.Attr("t").Text(); ok {
		seconds, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return Receipt{}, fmt.Errorf("%w: t %q", ErrIncoming, raw)
		}
		r.Time = time.Unix(seconds, 0)
	}
	view := kind == "view"
	if list, ok := n.Child("list"); ok {
		for _, item := range list.Children {
			key := "id"
			if view {
				key = "server_id"
			}
			if v, ok := item.Attr(key).Text(); item.Tag == "item" && ok {
				r.IDs = append(r.IDs, v)
			}
		}
	}
	if !view {
		r.IDs = append(r.IDs, id)
	}
	return r, nil
}
