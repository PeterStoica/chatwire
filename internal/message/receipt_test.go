package message_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/node"
)

func render(n node.Node) string {
	parts := make([]string, 0, len(n.Attrs))
	for _, a := range n.Attrs {
		parts = append(parts, a.Key+"="+a.Value.String())
	}
	return n.Tag + " " + strings.Join(parts, " ")
}

func TestDeliveryReceipts(t *testing.T) {
	status := node.JID{User: "status", Server: node.ServerBroadcast}
	for _, tt := range []struct {
		name string
		in   node.Node
		want string
	}{
		{"phone", incoming(bob, nil, enc("msg")), "receipt id=3EB0AA to=40722222222@s.whatsapp.net"},
		{"companion", incoming(bobDev, nil, enc("msg")), "receipt id=3EB0AA to=40722222222:3@s.whatsapp.net"},
		{"our other device", incoming(ownDev, []node.Attr{{Key: "recipient", Value: node.Address(bob)}}, enc("msg")),
			"receipt id=3EB0AA to=40711111111:2@s.whatsapp.net recipient=40722222222@s.whatsapp.net type=sender"},
		{"group", incoming(group, []node.Attr{{Key: "participant", Value: node.Address(bobDev)}}, enc("skmsg")),
			"receipt id=3EB0AA to=120363000000000000@g.us participant=40722222222:3@s.whatsapp.net"},
		{"our device in a group", incoming(group, []node.Attr{{Key: "participant", Value: node.Address(ownDev)}}, enc("skmsg")),
			"receipt id=3EB0AA to=120363000000000000@g.us participant=40711111111:2@s.whatsapp.net type=sender"},
		{"peer", incoming(ownDev, []node.Attr{{Key: "category", Value: node.Text("peer")}}, enc("msg")),
			"receipt id=3EB0AA to=40711111111:2@s.whatsapp.net type=peer_msg"},
		{"status", incoming(status, []node.Attr{{Key: "participant", Value: node.Address(bobDev)}}, enc("skmsg")),
			"receipt id=3EB0AA to=status@broadcast participant=40722222222:3@s.whatsapp.net class=status"},
		{"other broadcast", incoming(node.JID{User: "123", Server: node.ServerBroadcast}, []node.Attr{{Key: "participant", Value: node.Address(bobDev)}}, enc("skmsg")),
			"receipt id=3EB0AA to=123@broadcast participant=40722222222:3@s.whatsapp.net"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			in, err := message.ParseIncoming(isMe, tt.in)
			if err != nil {
				t.Fatal(err)
			}
			if got := render(message.DeliveryReceipt(isMe, in)); got != tt.want {
				t.Fatalf("receipt\n got %s\nwant %s", got, tt.want)
			}
		})
	}
}

func TestReadReceiptsAreSplitLikeWhatsAppWeb(t *testing.T) {
	t.Parallel()
	ids := make([]string, 0, 600)
	for i := range 600 {
		ids = append(ids, fmt.Sprintf("3EB0%04d", i))
	}
	receipts := message.ReadReceipts(bob, bob, ids)
	if len(receipts) != 3 {
		t.Fatalf("%d receipts for 600 ids", len(receipts))
	}
	for i, want := range []int{256, 256, 88} {
		r := receipts[i]
		first, _ := r.Attr("id").Text()
		items := 0
		if list, ok := r.Child("list"); ok {
			items = len(list.Children)
		}
		if first != ids[i*256] || items+1 != want {
			t.Fatalf("receipt %d starts at %s with %d ids", i, first, items+1)
		}
		if _, ok := r.Attr("participant").JID(); ok {
			t.Fatal("a one-to-one read receipt names a participant")
		}
	}
	single := message.ReadReceipts(group, bobDev, []string{"3EB0G1"})
	participant, ok := single[0].Attr("participant").JID()
	if _, hasList := single[0].Child("list"); len(single) != 1 || hasList || !ok || participant != bobDev {
		t.Fatalf("group receipt = %v", single)
	}
	if none := message.ReadReceipts(bob, bob, nil); len(none) != 0 {
		t.Fatalf("no ids gave %d receipts", len(none))
	}
}
