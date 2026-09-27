package fakegroups

import (
	"strconv"
	"sync"

	"github.com/PeterStoica/chatwire/internal/groups"
	"github.com/PeterStoica/chatwire/internal/node"
)

type Server struct {
	mu      sync.Mutex
	groups  []groups.Group
	lists   int
	lookups int
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
		if g.Description != "" {
			children = append(children, node.Node{Tag: "description", Attrs: []node.Attr{{Key: "id", Value: node.Text("D1")}}, Children: []node.Node{{Tag: "body", Bytes: []byte(g.Description)}}})
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
