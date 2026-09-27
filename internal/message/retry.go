package message

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"

	"github.com/PeterStoica/chatwire/internal/curve"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/prekeys"
	"github.com/PeterStoica/chatwire/internal/signon"
)

const retryWithKeys = 2

var ErrRetry = errors.New("message: cannot address a retry receipt")

type RetryReason int

const (
	RetryUnknown RetryReason = iota
	RetryNoSession
	RetryInvalidKey
	RetryInvalidKeyID
	RetryInvalidMessage
	RetryInvalidSignature
	RetryFutureMessage
	RetryBadMAC
	RetryInvalidSession
	RetryInvalidMessageKey
)

type Retry struct {
	Count          int
	Reason         *RetryReason
	Registration   signon.Registration
	PreKey         prekeys.PreKey
	DeviceIdentity []byte
}

func RetryReceipt(mine func(node.JID) bool, in Incoming, r Retry) (node.Node, error) {
	group := in.From.Server == node.ServerGroup || in.From.Server == node.ServerBroadcast
	to := node.Device(in.From)
	if group {
		to = node.Address(in.From)
	}
	attrs := []node.Attr{{Key: "id", Value: node.Text(in.ID)}, {Key: "to", Value: to}}
	var category []node.Attr
	switch {
	case group:
		attrs = append(attrs, node.Attr{Key: attrParticipant, Value: node.Device(in.Author)})
	case mine(in.From) && in.Category == categoryPeer:
		category = []node.Attr{{Key: "category", Value: node.Text(categoryPeer)}}
	case mine(in.From) && in.Chat != in.From.WithoutDevice():
		attrs = append(attrs, node.Attr{Key: "recipient", Value: node.Address(in.Chat)})
	case mine(in.From):
		return node.Node{}, fmt.Errorf("%w: a retry to our own device needs a recipient", ErrRetry)
	}
	attrs = append(attrs, node.Attr{Key: attrType, Value: node.Text("retry")})
	retry := []node.Attr{
		{Key: "v", Value: node.Text("1")}, {Key: "count", Value: node.Text(strconv.Itoa(r.Count))},
		{Key: "id", Value: node.Text(in.ID)}, {Key: "t", Value: node.Text(strconv.FormatInt(in.Timestamp.Unix(), 10))},
	}
	if r.Reason != nil {
		retry = append(retry, node.Attr{Key: "error", Value: node.Text(strconv.Itoa(int(*r.Reason)))})
	}
	children := []node.Node{
		{Tag: "retry", Attrs: retry},
		{Tag: "registration", Bytes: binary.BigEndian.AppendUint32(nil, r.Registration.RegistrationID)},
	}
	if r.Count >= retryWithKeys {
		children = append(children, keys(r))
	}
	return node.Node{Tag: tagReceipt, Attrs: append(attrs, category...), Children: children}, nil
}

func keys(r Retry) node.Node {
	identity, signed, oneTime := r.Registration.Identity, r.Registration.SignedPreKey, r.PreKey.Key.Public()
	id := func(v uint32) []byte { return binary.BigEndian.AppendUint32(nil, v)[1:] }
	return node.Node{Tag: "keys", Children: []node.Node{
		{Tag: attrType, Bytes: []byte{curve.KeyType}},
		{Tag: "identity", Bytes: identity[:]},
		{Tag: "key", Children: []node.Node{{Tag: "id", Bytes: id(r.PreKey.ID)}, {Tag: "value", Bytes: oneTime[:]}}},
		{Tag: "skey", Children: []node.Node{{Tag: "id", Bytes: id(signed.ID)}, {Tag: "value", Bytes: signed.Key[:]}, {Tag: "signature", Bytes: signed.Signature[:]}}},
		{Tag: tagIdentity, Bytes: r.DeviceIdentity},
	}}
}
