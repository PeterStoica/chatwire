package fakeusync

import (
	"strconv"
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
	mu    sync.Mutex
	users map[node.JID][]Device
}

func New() *Server {
	return &Server{users: map[node.JID][]Device{}}
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
