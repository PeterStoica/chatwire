package message

import (
	"slices"

	"github.com/PeterStoica/chatwire/internal/node"
)

const (
	categoryPeer = "peer"
	statusUser   = "status"
	attrType     = "type"
	tagIdentity  = "device-identity"
	tagMessage   = "message"
	tagReceipt   = "receipt"

	attrParticipant = "participant"
)

func DeliveryReceipt(mine func(node.JID) bool, in Incoming) node.Node {
	attrs := []node.Attr{{Key: "id", Value: node.Text(in.ID)}, {Key: "to", Value: node.Address(in.From)}}
	group := in.From.Server == node.ServerGroup || in.From.Server == node.ServerBroadcast
	if group {
		attrs = append(attrs, node.Attr{Key: attrParticipant, Value: node.Device(in.Author)})
	}
	peer := in.Category == categoryPeer
	fromUs := mine(in.Author)
	if !peer && fromUs && !group {
		attrs = append(attrs, node.Attr{Key: "recipient", Value: node.Address(in.Chat)})
	}
	switch {
	case peer:
		attrs = append(attrs, node.Attr{Key: attrType, Value: node.Text("peer_msg")})
	case fromUs:
		attrs = append(attrs, node.Attr{Key: attrType, Value: node.Text("sender")})
	}
	if in.From.User == statusUser && in.From.Server == node.ServerBroadcast {
		attrs = append(attrs, node.Attr{Key: "class", Value: node.Text("status")})
	}
	return node.Node{Tag: tagReceipt, Attrs: attrs}
}

func HistorySyncReceipt(in Incoming) node.Node {
	return node.Node{Tag: tagReceipt, Attrs: []node.Attr{
		{Key: "to", Value: node.Address(in.Chat)}, {Key: attrType, Value: node.Text("hist_sync")}, {Key: "id", Value: node.Text(in.ID)},
	}}
}

const maxReceiptIDs = 256

func ReadReceipts(chat, sender node.JID, ids []string) []node.Node {
	var out []node.Node
	for batch := range slices.Chunk(ids, maxReceiptIDs) {
		out = append(out, readReceipt(chat, sender, batch))
	}
	return out
}

func readReceipt(chat, sender node.JID, ids []string) node.Node {
	attrs := []node.Attr{{Key: "to", Value: node.Address(chat)}, {Key: attrType, Value: node.Text("read")}, {Key: "id", Value: node.Text(ids[0])}}
	if chat.Server == node.ServerGroup || chat.Server == node.ServerBroadcast {
		attrs = append(attrs, node.Attr{Key: attrParticipant, Value: node.Device(sender)})
	}
	receipt := node.Node{Tag: tagReceipt, Attrs: attrs}
	if rest := ids[1:]; len(rest) > 0 {
		items := make([]node.Node, 0, len(rest))
		for _, id := range rest {
			items = append(items, node.Node{Tag: "item", Attrs: []node.Attr{{Key: "id", Value: node.Text(id)}}})
		}
		receipt.Children = []node.Node{{Tag: "list", Children: items}}
	}
	return receipt
}
