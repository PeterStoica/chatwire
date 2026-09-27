package fakegroups

import (
	"cmp"
	"strconv"
	"sync"

	"github.com/PeterStoica/chatwire/internal/groups"
	"github.com/PeterStoica/chatwire/internal/node"
)

type Server struct {
	Actor      node.JID
	InviteOnly map[node.JID]bool
	mu         sync.Mutex
	groups     []groups.Group
	lists      int
	lookups    int
}

func (s *Server) Group(jid node.JID) (groups.Group, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, g := range s.groups {
		if g.JID == jid {
			return g, true
		}
	}
	return groups.Group{}, false
}

func (s *Server) Queries() (lists, lookups int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lists, s.lookups
}

func New(g ...groups.Group) *Server {
	return &Server{groups: g}
}

func (s *Server) Handle(request node.Node) node.Node {
	s.mu.Lock()
	defer s.mu.Unlock()
	if kind, _ := request.Attr("type").Text(); kind == "set" {
		return s.change(request)
	}
	chosen := s.groups
	if query, ok := request.Child("query"); ok {
		s.lookups++
		chosen = nil
		for _, wanted := range query.Children {
			jid, _ := wanted.Attr("jid").JID()
			for _, g := range s.groups {
				if g.JID == jid {
					chosen = append(chosen, g)
				}
			}
		}
	} else {
		s.lists++
	}
	list := make([]node.Node, len(chosen))
	for i, g := range chosen {
		attrs := []node.Attr{
			{Key: "id", Value: node.Text(g.JID.User)}, {Key: "subject", Value: node.Text(g.Subject)},
			{Key: "creation", Value: node.Text(strconv.FormatInt(g.Created.Unix(), 10))},
		}
		if g.AddressingMode != "" {
			attrs = append(attrs, node.Attr{Key: "addressing_mode", Value: node.Text(g.AddressingMode)})
		}
		children := make([]node.Node, 0, len(g.Participants)+1)
		for _, p := range g.Participants {
			pattrs := []node.Attr{{Key: "jid", Value: node.Address(p.JID)}}
			if p.Admin {
				pattrs = append(pattrs, node.Attr{Key: "type", Value: node.Text("admin")})
			}
			if p.LID.Server != "" {
				pattrs = append(pattrs, node.Attr{Key: "lid", Value: node.Address(p.LID)})
			}
			if p.Phone.Server != "" {
				pattrs = append(pattrs, node.Attr{Key: "phone_number", Value: node.Address(p.Phone)})
			}
			children = append(children, node.Node{Tag: "participant", Attrs: pattrs})
		}
		if g.Disappearing > 0 {
			children = append(children, node.Node{Tag: "ephemeral", Attrs: []node.Attr{{Key: "expiration", Value: node.Text(strconv.FormatUint(uint64(g.Disappearing), 10))}}})
		}
		if g.Description != "" {
			children = append(children, node.Node{Tag: "description", Attrs: []node.Attr{{Key: "id", Value: node.Text(cmp.Or(g.DescriptionID, "D1"))}}, Children: []node.Node{{Tag: "body", Bytes: []byte(g.Description)}}})
		}
		list[i] = node.Node{Tag: "group", Attrs: attrs, Children: children}
	}
	return node.Node{
		Tag: "iq",
		Attrs: []node.Attr{
			{Key: "from", Value: node.Address(node.JID{Server: node.ServerGroup})},
			{Key: "type", Value: node.Text("result")}, {Key: "id", Value: request.Attr("id")},
		},
		Children: []node.Node{{Tag: "groups", Children: list}},
	}
}

func (s *Server) Add(g groups.Group) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.groups = append(s.groups, g)
}

func reply(request node.Node, children ...node.Node) node.Node {
	return node.Node{Tag: "iq", Attrs: []node.Attr{
		{Key: "from", Value: request.Attr("to")}, {Key: "type", Value: node.Text("result")}, {Key: "id", Value: request.Attr("id")},
	}, Children: children}
}

func refuse(request node.Node, code int, text string) node.Node {
	return node.Node{Tag: "iq", Attrs: []node.Attr{
		{Key: "from", Value: request.Attr("to")}, {Key: "type", Value: node.Text("error")}, {Key: "id", Value: request.Attr("id")},
	}, Children: []node.Node{{Tag: "error", Attrs: []node.Attr{{Key: "code", Value: node.Text(strconv.Itoa(code))}, {Key: "text", Value: node.Text(text)}}}}}
}

func (s *Server) change(request node.Node) node.Node {
	to, _ := request.Attr("to").JID()
	if leave, ok := request.Child("leave"); ok {
		for _, entry := range leave.Children {
			id, _ := entry.Attr("id").JID()
			if i := s.index(id); i >= 0 {
				s.groups[i].Participants = without(s.groups[i].Participants, s.Actor)
			}
		}
		return reply(request, leave)
	}
	i := s.index(to)
	if i < 0 {
		return refuse(request, 404, "item-not-found")
	}
	g := &s.groups[i]
	if s.Actor.Server != "" && !admin(*g, s.Actor) {
		return refuse(request, 401, "not-authorized")
	}
	if len(request.Children) == 0 {
		return refuse(request, 400, "bad-request")
	}
	op := request.Children[0]
	switch op.Tag {
	case "subject":
		g.Subject = string(op.Bytes)
		return reply(request)
	case "description":
		prev, _ := op.Attr("prev").Text()
		if prev != g.DescriptionID {
			return refuse(request, 409, "conflict")
		}
		g.DescriptionID, _ = op.Attr("id").Text()
		g.Description = ""
		if body, ok := op.Child("body"); ok {
			g.Description = string(body.Bytes)
		}
		return reply(request)
	case "add", "remove", "promote", "demote":
		results := make([]node.Node, 0, len(op.Children))
		for _, p := range op.Children {
			jid, _ := p.Attr("jid").JID()
			code := s.member(g, op.Tag, jid)
			attrs := []node.Attr{{Key: "jid", Value: node.Address(jid)}}
			if code != 0 {
				attrs = append(attrs, node.Attr{Key: "error", Value: node.Text(strconv.Itoa(code))})
			}
			results = append(results, node.Node{Tag: "participant", Attrs: attrs})
		}
		return reply(request, node.Node{Tag: op.Tag, Children: results})
	}
	return refuse(request, 400, "bad-request")
}

func (s *Server) member(g *groups.Group, change string, jid node.JID) int {
	in := slicesIndex(g.Participants, jid)
	switch {
	case change == "add" && in >= 0:
		return 409
	case change == "add" && s.InviteOnly[jid]:
		return 403
	case change == "add":
		g.Participants = append(g.Participants, groups.Participant{JID: jid})
	case in < 0:
		return 404
	case change == "remove":
		g.Participants = without(g.Participants, jid)
	default:
		g.Participants[in].Admin = change == "promote"
	}
	return 0
}

func (s *Server) index(jid node.JID) int {
	for i, g := range s.groups {
		if g.JID == jid {
			return i
		}
	}
	return -1
}

func admin(g groups.Group, who node.JID) bool {
	i := slicesIndex(g.Participants, who)
	return i >= 0 && g.Participants[i].Admin
}

func slicesIndex(members []groups.Participant, jid node.JID) int {
	for i, p := range members {
		if p.JID.WithoutDevice() == jid.WithoutDevice() {
			return i
		}
	}
	return -1
}

func without(members []groups.Participant, jid node.JID) []groups.Participant {
	out := members[:0:0]
	for _, p := range members {
		if p.JID.WithoutDevice() != jid.WithoutDevice() {
			out = append(out, p)
		}
	}
	return out
}
