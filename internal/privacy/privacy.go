package privacy

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/PeterStoica/chatwire/internal/node"
)

const (
	bucket     = 7 * 24 * time.Hour
	buckets    = 4
	kindTrust  = "trusted_contact"
	tagToken   = "token"
	tagTokens  = "tokens"
	attrType   = "type"
	attrStamp  = "t"
	attrHolder = "jid"
)

var ErrNotification = errors.New("privacy: not a token notification")

type Token struct {
	Contact node.JID
	Theirs  []byte
	Given   time.Time
	Ours    time.Time
}

func (t Token) Usable(now time.Time) bool {
	if len(t.Theirs) == 0 || t.Given.IsZero() {
		return false
	}
	return t.Given.Unix() >= (period(now)-(buckets-1))*int64(bucket/time.Second)
}

func (t Token) Due(now time.Time) bool {
	return t.Ours.IsZero() || period(now) > period(t.Ours)
}

func period(at time.Time) int64 {
	return at.Unix() / int64(bucket/time.Second)
}

func (t Token) Node() node.Node {
	return node.Node{Tag: "tctoken", Bytes: t.Theirs}
}

func Give(to node.JID, at time.Time) node.Node {
	return node.Node{
		Tag: "iq",
		Attrs: []node.Attr{
			{Key: "id", Value: node.Value{}}, {Key: "to", Value: node.Address(node.JID{Server: node.ServerUser})},
			{Key: attrType, Value: node.Text("set")}, {Key: "xmlns", Value: node.Text("privacy")},
		},
		Children: []node.Node{{Tag: tagTokens, Children: []node.Node{{Tag: tagToken, Attrs: []node.Attr{
			{Key: attrHolder, Value: node.Address(to.WithoutDevice())}, {Key: attrStamp, Value: node.Text(strconv.FormatInt(at.Unix(), 10))},
			{Key: attrType, Value: node.Text(kindTrust)},
		}}}}},
	}
}

func Received(mine func(node.JID) bool, n node.Node) ([]Token, error) {
	if kind, _ := n.Attr(attrType).Text(); n.Tag != "notification" || kind != "privacy_token" {
		return nil, ErrNotification
	}
	from, ok := n.Attr("from").JID()
	if !ok {
		return nil, fmt.Errorf("%w: no sender in %s", ErrNotification, n)
	}
	if lid, ok := n.Attr("sender_lid").JID(); ok && lid.Server == node.ServerLID {
		from = lid
	}
	list, _ := n.Child(tagTokens)
	var out []Token
	for _, child := range list.Children {
		kind, _ := child.Attr(attrType).Text()
		if child.Tag != tagToken || kind != kindTrust || len(child.Bytes) == 0 {
			continue
		}
		if holder, ok := child.Attr(attrHolder).JID(); ok && !mine(holder) {
			continue
		}
		raw, _ := child.Attr(attrStamp).Text()
		stamp, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || stamp <= 0 {
			continue
		}
		out = append(out, Token{Contact: from.WithoutDevice(), Theirs: child.Bytes, Given: time.Unix(stamp, 0)})
	}
	return out, nil
}
