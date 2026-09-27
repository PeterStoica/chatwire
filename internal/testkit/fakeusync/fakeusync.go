package fakeusync

import (
	"strconv"
	"strings"
	"sync"

	"github.com/PeterStoica/chatwire/internal/node"
)

const tagDevices = "devices"

type Device struct {
	ID       uint8
	KeyIndex uint32
	Hosted   bool
}

type Server struct {
	queries int
	mu      sync.Mutex
	users   map[node.JID][]Device
	lids    map[node.JID]node.JID
	abouts  map[node.JID]string
}

func (s *Server) Profile(user, lid node.JID, about string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lids[user], s.abouts[user] = lid, about
}

func New() *Server {
	return &Server{users: map[node.JID][]Device{}, lids: map[node.JID]node.JID{}, abouts: map[node.JID]string{}}
}

func (s *Server) Set(user node.JID, devices ...Device) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[user] = devices
}

func (s *Server) Handle(request node.Node) node.Node {
	s.mu.Lock()
	defer s.mu.Unlock()
	query, _ := request.Child("usync")
	requested, _ := query.Child("list")
	users := make([]node.Node, 0, len(requested.Children))
	s.queries++
	if protocols, _ := query.Child("query"); len(protocols.Children) > 0 && protocols.Children[0].Tag == "contact" {
		return s.contacts(request, query, requested)
	}
	for _, entry := range requested.Children {
		jid, _ := entry.Attr("jid").JID()
		users = append(users, node.Node{Tag: "user", Attrs: []node.Attr{{Key: "jid", Value: node.Address(jid)}}, Children: []node.Node{s.devices(jid)}})
	}
	return node.Node{
		Tag: "iq",
		Attrs: []node.Attr{
			{Key: "from", Value: node.Address(node.JID{Server: node.ServerUser})},
			{Key: "type", Value: node.Text("result")}, {Key: "id", Value: request.Attr("id")},
		},
		Children: []node.Node{{Tag: "usync", Attrs: query.Attrs, Children: []node.Node{
			{Tag: "result", Children: []node.Node{{Tag: tagDevices}}},
			{Tag: "list", Children: users},
		}}},
	}
}

func (s *Server) devices(user node.JID) node.Node {
	devices, ok := s.users[user]
	if !ok {
		return node.Node{Tag: tagDevices, Children: []node.Node{{Tag: "error", Attrs: []node.Attr{{Key: "code", Value: node.Text("404")}, {Key: "text", Value: node.Text("not-found")}}}}}
	}
	list := make([]node.Node, len(devices))
	for i, d := range devices {
		attrs := []node.Attr{{Key: "id", Value: node.Text(strconv.Itoa(int(d.ID)))}}
		if d.KeyIndex > 0 {
			attrs = append(attrs, node.Attr{Key: "key-index", Value: node.Text(strconv.FormatUint(uint64(d.KeyIndex), 10))})
		}
		if d.Hosted {
			attrs = append(attrs, node.Attr{Key: "is_hosted", Value: node.Text("true")})
		}
		list[i] = node.Node{Tag: "device", Attrs: attrs}
	}
	return node.Node{Tag: tagDevices, Children: []node.Node{
		{Tag: "device-list", Children: list},
		{Tag: "key-index-list", Attrs: []node.Attr{{Key: "ts", Value: node.Text("1800000000")}}, Bytes: []byte{0x0a, 0x01}},
	}}
}

func (s *Server) contacts(request, query, requested node.Node) node.Node {
	users := make([]node.Node, 0, len(requested.Children))
	for _, entry := range requested.Children {
		asked, _ := entry.Child("contact")
		user := node.JID{User: strings.TrimPrefix(string(asked.Bytes), "+"), Server: node.ServerUser}
		_, registered := s.users[user]
		kind := "out"
		if registered {
			kind = "in"
		}
		children := []node.Node{{Tag: "contact", Attrs: []node.Attr{{Key: "type", Value: node.Text(kind)}}, Bytes: asked.Bytes}}
		if lid, ok := s.lids[user]; ok && registered {
			children = append(children, node.Node{Tag: "lid", Attrs: []node.Attr{{Key: "val", Value: node.Address(lid)}}})
		}
		if about, ok := s.abouts[user]; ok && registered {
			children = append(children, node.Node{Tag: "status", Bytes: []byte(about)})
		}
		users = append(users, node.Node{Tag: "user", Attrs: []node.Attr{{Key: "jid", Value: node.Address(user)}}, Children: children})
	}
	return node.Node{
		Tag: "iq",
		Attrs: []node.Attr{
			{Key: "from", Value: node.Address(node.JID{Server: node.ServerUser})},
			{Key: "type", Value: node.Text("result")}, {Key: "id", Value: request.Attr("id")},
		},
		Children: []node.Node{{Tag: "usync", Attrs: query.Attrs, Children: []node.Node{{Tag: "list", Children: users}}}},
	}
}

func (s *Server) Queries() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.queries
}

func (s *Server) DevicesOf(users ...node.JID) []node.JID {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []node.JID
	for _, u := range users {
		for _, d := range s.users[u.WithoutDevice()] {
			out = append(out, node.JID{User: u.User, Device: d.ID, Server: u.Server})
		}
	}
	return out
}
