package fakekeys

import (
	"bytes"
	"crypto/sha1"
	"sync"

	"github.com/PeterStoica/chatwire/internal/node"
)

const attrType = "type"

type Server struct {
	mu         sync.Mutex
	devices    map[node.JID]*bundle
	identities map[node.JID][]byte
}

type bundle struct {
	registration, identity []byte
	skey                   node.Node
	keys                   []node.Node
}

func New() *Server {
	return &Server{devices: map[node.JID]*bundle{}, identities: map[node.JID][]byte{}}
}

func (s *Server) Handle(from node.JID, request node.Node) node.Node {
	s.mu.Lock()
	defer s.mu.Unlock()
	kind, _ := request.Attr(attrType).Text()
	_, digest := request.Child("digest")
	_, key := request.Child("key")
	switch {
	case kind == "set":
		return s.upload(from, request)
	case kind == "get" && digest:
		return s.digest(from, request)
	case kind == "get" && key:
		return s.bundles(request)
	default:
		return failure(request, "400", "bad-request")
	}
}

func (s *Server) Keys(device node.JID) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b, ok := s.devices[device]; ok {
		return len(b.keys)
	}
	return 0
}

func (s *Server) upload(from node.JID, request node.Node) node.Node {
	registration, _ := request.Child("registration")
	keyType, _ := request.Child("type")
	identity, _ := request.Child("identity")
	list, _ := request.Child("list")
	skey, _ := request.Child("skey")
	valid := len(registration.Bytes) == 4 && bytes.Equal(keyType.Bytes, []byte{5}) && len(identity.Bytes) == 32 &&
		sized(skey, "id", 3) && sized(skey, "value", 32) && sized(skey, "signature", 64) && len(list.Children) > 0
	for _, key := range list.Children {
		valid = valid && key.Tag == "key" && sized(key, "id", 3) && sized(key, "value", 32)
	}
	if !valid {
		return failure(request, "406", "not-acceptable")
	}
	b, ok := s.devices[from]
	if !ok {
		b = &bundle{}
		s.devices[from] = b
	}
	b.registration, b.identity, b.skey = registration.Bytes, identity.Bytes, skey
	b.keys = append(b.keys, list.Children...)
	return result(request)
}

func (s *Server) digest(from node.JID, request node.Node) node.Node {
	b, ok := s.devices[from]
	if !ok {
		return failure(request, "404", "item-not-found")
	}
	value, _ := b.skey.Child("value")
	signature, _ := b.skey.Child("signature")
	material := bytes.Join([][]byte{b.identity, value.Bytes, signature.Bytes}, nil)
	ids := make([]node.Node, len(b.keys))
	for i, key := range b.keys {
		id, _ := key.Child("id")
		keyValue, _ := key.Child("value")
		ids[i] = node.Node{Tag: "id", Bytes: id.Bytes}
		material = append(material, keyValue.Bytes...)
	}
	hash := sha1.Sum(material)
	reply := result(request)
	reply.Children = []node.Node{{Tag: "digest", Children: []node.Node{
		{Tag: "registration", Bytes: b.registration},
		{Tag: attrType, Bytes: []byte{5}},
		{Tag: "identity", Bytes: b.identity},
		b.skey,
		{Tag: "list", Children: ids},
		{Tag: "hash", Bytes: hash[:]},
	}}}
	return reply
}

func (s *Server) Register(device node.JID, deviceIdentity []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.identities[device] = deviceIdentity
}

func (s *Server) bundles(request node.Node) node.Node {
	key, _ := request.Child("key")
	users := make([]node.Node, 0, len(key.Children))
	for _, user := range key.Children {
		jid := user.Attr("jid")
		device, _ := jid.JID()
		b, ok := s.devices[device]
		if !ok {
			users = append(users, node.Node{Tag: "user", Attrs: []node.Attr{{Key: "jid", Value: jid}}, Children: []node.Node{
				{Tag: "error", Attrs: []node.Attr{{Key: "code", Value: node.Text("500")}, {Key: "text", Value: node.Text("internal-server-error")}}},
			}})
			continue
		}
		children := []node.Node{{Tag: "registration", Bytes: b.registration}, {Tag: attrType, Bytes: []byte{5}}, {Tag: "identity", Bytes: b.identity}}
		if len(b.keys) > 0 {
			children, b.keys = append(children, b.keys[0]), b.keys[1:]
		}
		children = append(children, b.skey)
		if identity, ok := s.identities[device]; ok {
			children = append(children, node.Node{Tag: "device-identity", Bytes: identity})
		}
		users = append(users, node.Node{Tag: "user", Attrs: []node.Attr{{Key: "jid", Value: jid}}, Children: children})
	}
	reply := result(request)
	reply.Children = []node.Node{{Tag: "list", Children: users}}
	return reply
}

func sized(n node.Node, tag string, size int) bool {
	child, ok := n.Child(tag)
	return ok && len(child.Bytes) == size
}

func result(request node.Node) node.Node {
	return node.Node{Tag: "iq", Attrs: []node.Attr{
		{Key: "from", Value: node.Address(node.JID{Server: node.ServerUser})},
		{Key: attrType, Value: node.Text("result")},
		{Key: "id", Value: request.Attr("id")},
	}}
}

func failure(request node.Node, code, text string) node.Node {
	reply := result(request)
	reply.Attrs[1].Value = node.Text("error")
	reply.Children = []node.Node{{Tag: "error", Attrs: []node.Attr{{Key: "code", Value: node.Text(code)}, {Key: "text", Value: node.Text(text)}}}}
	return reply
}
