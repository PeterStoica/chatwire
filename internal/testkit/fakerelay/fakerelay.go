package fakerelay

import (
	"strconv"
	"time"

	"github.com/PeterStoica/chatwire/internal/node"
)

const (
	attrFrom   = "from"
	attrType   = "type"
	typePreKey = "pkmsg"
	tagMessage = "message"
)

func Deliver(sender node.JID, pushName string, at time.Time, out node.Node) map[node.JID]node.Node {
	return DeliverFrom(sender, node.JID{}, pushName, at, out)
}

func DeliverFrom(sender, senderLID node.JID, pushName string, at time.Time, out node.Node) map[node.JID]node.Node {
	to, _ := out.Attr("to").JID()
	identity, hasIdentity := out.Child("device-identity")
	targets := map[node.JID]node.Node{}
	if participants, ok := out.Child("participants"); ok {
		for _, target := range participants.Children {
			device, _ := target.Attr("jid").JID()
			enc, _ := target.Child("enc")
			targets[device] = enc
		}
	} else if enc, ok := out.Child("enc"); ok {
		targets[to.WithoutDevice()] = enc
	}
	deliveries := make(map[node.JID]node.Node, len(targets))
	for device, enc := range targets {
		attrs := []node.Attr{
			{Key: attrFrom, Value: node.Address(sender)}, {Key: attrType, Value: out.Attr(attrType)}, {Key: "id", Value: out.Attr("id")},
			{Key: "t", Value: node.Text(strconv.FormatInt(at.Unix(), 10))}, {Key: "notify", Value: node.Text(pushName)},
		}
		if device.User == sender.User && device.Server == sender.Server || senderLID.User != "" && device.User == senderLID.User && device.Server == senderLID.Server {
			attrs = append(attrs, node.Attr{Key: "recipient", Value: node.Address(to)})
		}
		children := []node.Node{enc}
		if kind, _ := enc.Attr(attrType).Text(); kind == typePreKey && hasIdentity {
			children = append(children, identity)
		}
		deliveries[device] = passThrough(out, node.Node{Tag: tagMessage, Attrs: attrs, Children: children})
	}
	return deliveries
}

func DeliverGroup(sender node.JID, pushName string, at time.Time, out node.Node, members []node.JID) map[node.JID]node.Node {
	group, _ := out.Attr("to").JID()
	identity, hasIdentity := out.Child("device-identity")
	pairwise := map[node.JID]node.Node{}
	if participants, ok := out.Child("participants"); ok {
		for _, target := range participants.Children {
			device, _ := target.Attr("jid").JID()
			pairwise[device], _ = target.Child("enc")
		}
	}
	var skmsg node.Node
	for _, child := range out.Children {
		if kind, _ := child.Attr(attrType).Text(); child.Tag == "enc" && kind == "skmsg" {
			skmsg = child
		}
	}
	deliveries := make(map[node.JID]node.Node, len(members))
	for _, device := range members {
		if device == sender {
			continue
		}
		var children []node.Node
		if enc, ok := pairwise[device]; ok {
			children = append(children, enc)
			if kind, _ := enc.Attr(attrType).Text(); kind == typePreKey && hasIdentity {
				children = append(children, identity)
			}
		}
		children = append(children, skmsg)
		deliveries[device] = passThrough(out, node.Node{Tag: tagMessage, Attrs: []node.Attr{
			{Key: attrFrom, Value: node.Address(group)}, {Key: "participant", Value: node.Address(sender)},
			{Key: attrType, Value: out.Attr(attrType)}, {Key: "id", Value: out.Attr("id")},
			{Key: "t", Value: node.Text(strconv.FormatInt(at.Unix(), 10))}, {Key: "notify", Value: node.Text(pushName)},
		}, Children: children})
	}
	return deliveries
}

func DeliverPeer(sender node.JID, at time.Time, out node.Node) node.Node {
	var enc node.Node
	for _, child := range out.Children {
		switch child.Tag {
		case "enc":
			enc = child
		case "participants":
			for _, to := range child.Children {
				enc, _ = to.Child("enc")
			}
		}
	}
	children := []node.Node{enc}
	if kind, _ := enc.Attr(attrType).Text(); kind == typePreKey {
		if identity, ok := out.Child("device-identity"); ok {
			children = append(children, identity)
		}
	}
	return node.Node{Tag: tagMessage, Attrs: []node.Attr{
		{Key: attrFrom, Value: node.Address(sender)}, {Key: attrType, Value: out.Attr(attrType)}, {Key: "id", Value: out.Attr("id")},
		{Key: "t", Value: node.Text(strconv.FormatInt(at.Unix(), 10))}, {Key: "category", Value: node.Text("peer")},
	}, Children: children}
}

func DeliverReceipt(sender node.JID, out node.Node) node.Node {
	to, _ := out.Attr("to").JID()
	group := to.Server == node.ServerGroup || to.Server == node.ServerBroadcast
	attrs := make([]node.Attr, 0, len(out.Attrs))
	for _, a := range out.Attrs {
		switch {
		case a.Key == "to" && group:
			attrs = append(attrs, node.Attr{Key: attrFrom, Value: a.Value})
		case a.Key == "to":
			attrs = append(attrs, node.Attr{Key: attrFrom, Value: node.Device(sender)})
		case a.Key == "participant":
			attrs = append(attrs, node.Attr{Key: "participant", Value: node.Device(sender)})
		default:
			attrs = append(attrs, a)
		}
	}
	out.Attrs = attrs
	return out
}

func passThrough(out, in node.Node) node.Node {
	if edit, ok := out.Attr("edit").Text(); ok {
		in.Attrs = append(in.Attrs, node.Attr{Key: "edit", Value: node.Text(edit)})
	}
	if meta, ok := out.Child("meta"); ok {
		in.Children = append(in.Children, meta)
	}
	return in
}
