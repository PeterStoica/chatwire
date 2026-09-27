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
	AckRetry
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
	case "retry":
		return AckRetry, false, true
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

type RetryRequest struct {
	ID           string
	From         node.JID
	Participant  node.JID
	Recipient    node.JID
	Count        int
	Registration []byte
	Keys         *node.Node
	echo         []node.Attr
}

func (r RetryRequest) Group() bool {
	return r.From.Server == node.ServerGroup || r.From.Server == node.ServerBroadcast
}

func (r RetryRequest) Device() node.JID {
	if r.Group() {
		return r.Participant
	}
	return r.From
}

func ParseRetryRequest(n node.Node) (RetryRequest, error) {
	id, _ := n.Attr("id").Text()
	from, ok := n.Attr("from").JID()
	kind, _ := n.Attr(attrType).Text()
	retry, hasRetry := n.Child("retry")
	registration, hasRegistration := n.Child("registration")
	if n.Tag != tagReceipt || kind != "retry" || id == "" || !ok || !hasRetry || !hasRegistration {
		return RetryRequest{}, fmt.Errorf("%w: not a retry request: %s", ErrIncoming, n)
	}
	count, err := strconv.Atoi(text(retry, "count"))
	if err != nil || count < 1 {
		return RetryRequest{}, fmt.Errorf("%w: retry count %q", ErrIncoming, text(retry, "count"))
	}
	r := RetryRequest{ID: id, From: from, Count: count, Registration: registration.Bytes}
	r.Participant, _ = n.Attr(attrParticipant).JID()
	r.Recipient, _ = n.Attr("recipient").JID()
	if r.Group() && r.Participant.Server == "" {
		return RetryRequest{}, fmt.Errorf("%w: group retry request without a participant", ErrIncoming)
	}
	if keys, ok := n.Child("keys"); ok {
		r.Keys = &keys
	}
	r.echo = []node.Attr{{Key: "to", Value: n.Attr("from")}}
	for _, key := range []string{attrParticipant, "recipient", "edit"} {
		if v := n.Attr(key); !v.IsZero() {
			r.echo = append(r.echo, node.Attr{Key: key, Value: v})
		}
	}
	return r, nil
}
