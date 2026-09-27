package message_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/node"
)

func TestParsingIncomingReceipts(t *testing.T) {
	t.Parallel()
	attr := func(k, v string) node.Attr { return node.Attr{Key: k, Value: node.Text(v)} }
	receipt := func(extra ...node.Attr) node.Node {
		return node.Node{Tag: "receipt", Attrs: append([]node.Attr{{Key: "from", Value: node.Address(bob)}, attr("id", "3EB0R1"), attr("t", "1790000000")}, extra...)}
	}
	for _, tt := range []struct {
		kind string
		ack  message.Ack
		self bool
	}{
		{"", message.AckDelivered, false}, {"delivery", message.AckDelivered, false}, {"sender", message.AckDelivered, false},
		{"read", message.AckRead, false}, {"read-self", message.AckRead, true}, {"played", message.AckPlayed, false}, {"played-self", message.AckPlayed, true},
		{"inactive", message.AckInactive, false}, {"server-error", message.AckGone, false}, {"peer_msg", message.AckPeer, false}, {"hologram", message.AckDelivered, false},
	} {
		n := receipt()
		if tt.kind != "" {
			n = receipt(attr("type", tt.kind))
		}
		got, err := message.ParseReceipt(n)
		if err != nil || got.Ack != tt.ack || got.Self != tt.self || got.From != bob || !got.Time.Equal(time.Unix(1790000000, 0)) || !slices.Equal(got.IDs, []string{"3EB0R1"}) {
			t.Errorf("type %q = %+v, %v", tt.kind, got, err)
		}
	}
	listed := receipt(attr("type", "read"), node.Attr{Key: "participant", Value: node.Address(bobDev)})
	listed.Children = []node.Node{{Tag: "list", Children: []node.Node{{Tag: "item", Attrs: []node.Attr{attr("id", "3EB0R2")}}, {Tag: "other"}, {Tag: "item", Attrs: []node.Attr{attr("id", "3EB0R3")}}}}}
	if got, err := message.ParseReceipt(listed); err != nil || !slices.Equal(got.IDs, []string{"3EB0R2", "3EB0R3", "3EB0R1"}) || got.Participant != bobDev {
		t.Fatalf("a listed receipt = %+v, %v", got, err)
	}
	view := receipt(attr("type", "view"))
	view.Children = []node.Node{{Tag: "list", Children: []node.Node{{Tag: "item", Attrs: []node.Attr{attr("server_id", "77"), attr("id", "ignored")}}}}}
	if got, err := message.ParseReceipt(view); err != nil || !slices.Equal(got.IDs, []string{"77"}) {
		t.Fatalf("a view receipt = %+v, %v", got, err)
	}
	untimed := node.Node{Tag: "receipt", Attrs: []node.Attr{{Key: "from", Value: node.Address(bob)}, attr("id", "3EB0R1"), {Key: "recipient", Value: node.Address(group)}}}
	if got, err := message.ParseReceipt(untimed); err != nil || !got.Time.IsZero() || got.Recipient != group {
		t.Fatalf("a receipt without a time = %+v, %v", got, err)
	}
	for name, n := range map[string]node.Node{
		"not a receipt": {Tag: "message", Attrs: []node.Attr{{Key: "from", Value: node.Address(bob)}, attr("id", "x")}},
		"no id":         {Tag: "receipt", Attrs: []node.Attr{{Key: "from", Value: node.Address(bob)}}},
		"no from":       {Tag: "receipt", Attrs: []node.Attr{attr("id", "x")}},
		"a bad time":    {Tag: "receipt", Attrs: []node.Attr{{Key: "from", Value: node.Address(bob)}, attr("id", "x"), attr("t", "soon")}},
		"aggregated":    {Tag: "receipt", Attrs: []node.Attr{{Key: "from", Value: node.Address(group)}, attr("id", "x")}, Children: []node.Node{{Tag: "participants"}}},
	} {
		if _, err := message.ParseReceipt(n); !errors.Is(err, message.ErrIncoming) {
			t.Errorf("%s: %v", name, err)
		}
	}
}
