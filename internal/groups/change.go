package groups

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/PeterStoica/chatwire/internal/node"
)

const (
	MaxSubject     = 100
	MaxDescription = 2048
)

type Change string

const (
	Add     Change = "add"
	Remove  Change = "remove"
	Promote Change = "promote"
	Demote  Change = "demote"
)

var ErrRefused = errors.New("groups: WhatsApp refused the change")

type Refused struct {
	Code int
	Text string
}

func (r Refused) Error() string {
	return fmt.Sprintf("%v: %d %s", ErrRefused, r.Code, r.Text)
}

func (r Refused) Is(target error) bool {
	return target == ErrRefused
}

type Member struct {
	JID   node.JID
	Phone node.JID
}

type Outcome struct {
	JID  node.JID
	Code int
}

func set(group node.JID, children ...node.Node) node.Node {
	return node.Node{
		Tag: "iq",
		Attrs: []node.Attr{
			{Key: "to", Value: node.Address(group)}, {Key: "xmlns", Value: node.Text("w:g2")},
			{Key: "id", Value: node.Value{}}, {Key: "type", Value: node.Text("set")},
		},
		Children: children,
	}
}

func SetSubject(group node.JID, subject string) node.Node {
	return set(group, node.Node{Tag: "subject", Bytes: []byte(subject)})
}

func SetDescription(group node.JID, id, prev, text string) node.Node {
	attrs := []node.Attr{{Key: "id", Value: node.Text(id)}}
	if prev != "" {
		attrs = append(attrs, node.Attr{Key: "prev", Value: node.Text(prev)})
	}
	description := node.Node{Tag: "description", Attrs: attrs}
	if text == "" {
		description.Attrs = append(description.Attrs, node.Attr{Key: "delete", Value: node.Text("true")})
	} else {
		description.Children = []node.Node{{Tag: "body", Bytes: []byte(text)}}
	}
	return set(group, description)
}

func ChangeMembers(group node.JID, change Change, members []Member) node.Node {
	people := make([]node.Node, 0, len(members))
	for _, m := range members {
		attrs := []node.Attr{{Key: "jid", Value: node.Address(m.JID.WithoutDevice())}}
		if change == Add && m.Phone.Server != "" && m.Phone != m.JID {
			attrs = append(attrs, node.Attr{Key: "phone_number", Value: node.Address(m.Phone.WithoutDevice())})
		}
		people = append(people, node.Node{Tag: "participant", Attrs: attrs})
	}
	return set(group, node.Node{Tag: string(change), Children: people})
}

func Leave(group node.JID) node.Node {
	return set(node.JID{Server: node.ServerGroup}, node.Node{Tag: "leave", Children: []node.Node{
		{Tag: "group", Attrs: []node.Attr{{Key: "id", Value: node.Address(group)}}},
	}})
}

func RefusedBy(reply node.Node) error {
	if kind, _ := reply.Attr("type").Text(); reply.Tag != "iq" || kind != "error" {
		return nil
	}
	failure, _ := reply.Child("error")
	code, _ := strconv.Atoi(text(failure, "code"))
	return Refused{Code: code, Text: text(failure, "text")}
}

func ParseChange(reply node.Node, change Change) ([]Outcome, error) {
	if err := RefusedBy(reply); err != nil {
		return nil, err
	}
	list, ok := reply.Child(string(change))
	if kind, _ := reply.Attr("type").Text(); reply.Tag != "iq" || kind != "result" || !ok {
		return nil, fmt.Errorf("%w: %s", ErrReply, reply)
	}
	out := make([]Outcome, 0, len(list.Children))
	for _, p := range list.Children {
		jid, ok := p.Attr("jid").JID()
		if p.Tag != "participant" || !ok {
			continue
		}
		code, _ := strconv.Atoi(text(p, "error"))
		out = append(out, Outcome{JID: jid, Code: code})
	}
	return out, nil
}
