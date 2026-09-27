package usync

import (
	"fmt"
	"strings"

	"github.com/PeterStoica/chatwire/internal/node"
)

type Contact struct {
	Number     string
	JID        node.JID
	OnWhatsApp bool
	LID        node.JID
	About      string
}

func ContactsRequest(sid string, numbers []string) node.Node {
	list := make([]node.Node, len(numbers))
	for i, n := range numbers {
		list[i] = node.Node{Tag: "user", Children: []node.Node{{Tag: "contact", Bytes: []byte("+" + strings.TrimPrefix(n, "+"))}}}
	}
	return request(sid, ContextInteractive, []node.Node{{Tag: "contact"}, {Tag: "lid"}, {Tag: "status"}}, list)
}

func ParseContacts(reply node.Node) ([]Contact, error) {
	usync, ok := reply.Child("usync")
	if kind, _ := reply.Attr("type").Text(); reply.Tag != "iq" || kind != "result" || !ok {
		return nil, fmt.Errorf("%w: %s", ErrReply, reply)
	}
	list, _ := usync.Child("list")
	out := make([]Contact, 0, len(list.Children))
	for _, entry := range list.Children {
		contact, ok := entry.Child("contact")
		if entry.Tag != "user" || !ok {
			continue
		}
		kind, _ := contact.Attr("type").Text()
		c := Contact{Number: strings.TrimPrefix(string(contact.Bytes), "+"), OnWhatsApp: kind == "in"}
		c.JID, _ = entry.Attr("jid").JID()
		if lid, ok := entry.Child("lid"); ok {
			c.LID, _ = lid.Attr("val").JID()
		}
		if status, ok := entry.Child("status"); ok {
			if _, failed := status.Child("error"); !failed {
				c.About = strings.TrimSpace(string(status.Bytes))
			}
		}
		out = append(out, c)
	}
	return out, nil
}
