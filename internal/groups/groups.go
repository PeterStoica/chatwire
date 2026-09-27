package groups

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/PeterStoica/chatwire/internal/node"
)

var ErrReply = errors.New("groups: unexpected reply")

type Participant struct {
	JID   node.JID
	LID   node.JID
	Phone node.JID
	Admin bool
}

func (p Participant) Pair() (lid, phone node.JID, ok bool) {
	switch {
	case p.JID.Server == node.ServerLID && p.Phone.Server == node.ServerUser:
		return p.JID.WithoutDevice(), p.Phone.WithoutDevice(), true
	case p.JID.Server == node.ServerUser && p.LID.Server == node.ServerLID:
		return p.LID.WithoutDevice(), p.JID.WithoutDevice(), true
	}
	return node.JID{}, node.JID{}, false
}

type Group struct {
	JID            node.JID
	Subject        string
	Created        time.Time
	AddressingMode string
	Participants   []Participant
	Description    string
	DescriptionID  string
}

func ParticipatingRequest() node.Node {
	return node.Node{
		Tag: "iq",
		Attrs: []node.Attr{
			{Key: "to", Value: node.Address(node.JID{Server: node.ServerGroup})}, {Key: "xmlns", Value: node.Text("w:g2")},
			{Key: "id", Value: node.Value{}}, {Key: "type", Value: node.Text("get")},
		},
		Children: []node.Node{{Tag: "participating", Children: []node.Node{{Tag: "participants"}, {Tag: "description"}}}},
	}
}

func InfoRequest(jids ...node.JID) node.Node {
	wanted := make([]node.Node, 0, len(jids))
	for _, j := range jids {
		wanted = append(wanted, node.Node{Tag: "group", Attrs: []node.Attr{{Key: "jid", Value: node.Address(j.WithoutDevice())}}})
	}
	return node.Node{
		Tag: "iq",
		Attrs: []node.Attr{
			{Key: "to", Value: node.Address(node.JID{Server: node.ServerGroup})}, {Key: "xmlns", Value: node.Text("w:g2")},
			{Key: "id", Value: node.Value{}}, {Key: "type", Value: node.Text("get")},
		},
		Children: []node.Node{{Tag: "query", Children: wanted}},
	}
}

func ParseParticipating(reply node.Node) ([]Group, error) {
	return parseList(reply)
}

func ParseInfo(reply node.Node) ([]Group, error) {
	return parseList(reply)
}

func parseList(reply node.Node) ([]Group, error) {
	kind, _ := reply.Attr("type").Text()
	list, ok := reply.Child("groups")
	if reply.Tag != "iq" || kind != "result" || !ok {
		return nil, fmt.Errorf("%w: %s", ErrReply, reply)
	}
	groups := make([]Group, 0, len(list.Children))
	for _, entry := range list.Children {
		if entry.Tag != "group" {
			continue
		}
		if g, err := parseGroup(entry); err == nil {
			groups = append(groups, g)
		}
	}
	return groups, nil
}

func parseGroup(entry node.Node) (Group, error) {
	id, _ := entry.Attr("id").Text()
	if _, failed := entry.Child("error"); id == "" || failed {
		return Group{}, fmt.Errorf("%w: group without id: %s", ErrReply, entry)
	}
	g := Group{JID: node.JID{User: id, Server: node.ServerGroup}, Subject: text(entry, "subject"), AddressingMode: text(entry, "addressing_mode")}
	if created, err := strconv.ParseInt(text(entry, "creation"), 10, 64); err == nil && created > 0 {
		g.Created = time.Unix(created, 0)
	}
	for _, child := range entry.Children {
		switch child.Tag {
		case "participant":
			jid, ok := child.Attr("jid").JID()
			if !ok {
				continue
			}
			lid, _ := child.Attr("lid").JID()
			phone, _ := child.Attr("phone_number").JID()
			role := text(child, "type")
			g.Participants = append(g.Participants, Participant{JID: jid, LID: lid, Phone: phone, Admin: role == "admin" || role == "superadmin"})
		case "description":
			g.DescriptionID = text(child, "id")
			if body, ok := child.Child("body"); ok {
				g.Description = string(body.Bytes)
			}
		}
	}
	return g, nil
}

func text(n node.Node, key string) string {
	value, _ := n.Attr(key).Text()
	return value
}
