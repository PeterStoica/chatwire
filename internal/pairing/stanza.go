package pairing

import "github.com/PeterStoica/chatwire/internal/node"

const (
	attrType = "type"
	attrID   = "id"
	attrTo   = "to"
)

func server() node.Value {
	return node.Address(node.JID{Server: node.ServerUser})
}

func reply(kind string, id node.Value, children ...node.Node) node.Node {
	return node.Node{Tag: "iq", Attrs: []node.Attr{{Key: attrID, Value: id}, {Key: attrTo, Value: server()}, {Key: attrType, Value: node.Text(kind)}}, Children: children}
}

func iqSet(child node.Node) node.Node {
	return node.Node{
		Tag: "iq",
		Attrs: []node.Attr{
			{Key: "xmlns", Value: node.Text("md")}, {Key: attrTo, Value: server()},
			{Key: attrID, Value: node.Value{}}, {Key: attrType, Value: node.Text("set")},
		},
		Children: []node.Node{child},
	}
}
