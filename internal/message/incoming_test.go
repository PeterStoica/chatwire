package message_test

import (
	"errors"
	"testing"
	"time"

	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/node"
)

var (
	me      = node.JID{User: "40711111111", Server: node.ServerUser}
	bob     = node.JID{User: "40722222222", Server: node.ServerUser}
	bobDev  = node.JID{User: bob.User, Device: 3, Server: bob.Server}
	group   = node.JID{User: "120363000000000000", Server: node.ServerGroup}
	ownDev  = node.JID{User: me.User, Device: 2, Server: me.Server}
	hostDev = node.JID{User: bob.User, Device: 99, Server: node.ServerHosted}
)

func isMe(j node.JID) bool { return j.User == me.User && j.Server == me.Server }

func incoming(from node.JID, extra []node.Attr, children ...node.Node) node.Node {
	attrs := make([]node.Attr, 0, 5+len(extra))
	attrs = append(attrs, node.Attr{Key: "from", Value: node.Address(from)}, node.Attr{Key: "type", Value: node.Text("text")},
		node.Attr{Key: "id", Value: node.Text("3EB0AA")}, node.Attr{Key: "t", Value: node.Text("1790000000")}, node.Attr{Key: "notify", Value: node.Text("Bob")})
	return node.Node{Tag: "message", Attrs: append(attrs, extra...), Children: children}
}

func enc(kind string, attrs ...node.Attr) node.Node {
	return node.Node{Tag: "enc", Attrs: append([]node.Attr{{Key: "v", Value: node.Text("2")}, {Key: "type", Value: node.Text(kind)}}, attrs...), Bytes: []byte{9, 9}}
}

func TestIncomingAddressing(t *testing.T) {
	for _, tt := range []struct {
		name         string
		in           node.Node
		chat, author node.JID
	}{
		{"primary phone", incoming(bob, nil, enc("pkmsg")), bob, bob},
		{"companion device", incoming(bobDev, nil, enc("msg")), bob, bobDev},
		{"hosted device", incoming(hostDev, nil, enc("msg")), node.JID{User: bob.User, Server: node.ServerHosted}, hostDev},
		{"our other device", incoming(ownDev, []node.Attr{{Key: "recipient", Value: node.Address(bob)}}, enc("msg")), bob, ownDev},
		{"group", incoming(group, []node.Attr{{Key: "participant", Value: node.Address(bobDev)}}, enc("skmsg")), group, bobDev},
		{"our device in a group", incoming(group, []node.Attr{{Key: "participant", Value: node.Address(ownDev)}, {Key: "recipient", Value: node.Address(bob)}}, enc("skmsg")), group, ownDev},
	} {
		t.Run(tt.name, func(t *testing.T) {
			in, err := message.ParseIncoming(isMe, tt.in)
			if err != nil || in.Chat != tt.chat || in.Author != tt.author {
				t.Fatalf("ParseIncoming() = chat %v author %v, %v; want %v, %v", in.Chat, in.Author, err, tt.chat, tt.author)
			}
		})
	}
}

func TestIncomingFields(t *testing.T) {
	in, err := message.ParseIncoming(isMe, incoming(bob, nil,
		enc("msg", node.Attr{Key: "count", Value: node.Text("2")}, node.Attr{Key: "mediatype", Value: node.Text("image")}),
		enc("pkmsg"),
		node.Node{Tag: "device-identity", Bytes: []byte{7}},
		node.Node{Tag: "meta"},
	))
	if err != nil {
		t.Fatal(err)
	}
	if in.ID != "3EB0AA" || !in.Timestamp.Equal(time.Unix(1790000000, 0)) || in.PushName != "Bob" || in.Type != "text" || len(in.DeviceIdentity) != 1 {
		t.Fatalf("incoming = %+v", in)
	}
	if len(in.Encs) != 2 || in.Encs[0].Retry != 2 || in.Encs[0].MediaType != "image" || in.Encs[0].Type != "msg" || in.Encs[1].Type != "pkmsg" || len(in.Encs[1].Ciphertext) != 2 {
		t.Fatalf("encs = %+v", in.Encs)
	}
}

func TestIncomingRefusals(t *testing.T) {
	without := func(n node.Node, key string) node.Node {
		kept := []node.Attr{}
		for _, a := range n.Attrs {
			if a.Key != key {
				kept = append(kept, a)
			}
		}
		n.Attrs = kept
		return n
	}
	for _, tt := range []struct {
		name string
		in   node.Node
	}{
		{"not a message", node.Node{Tag: "receipt"}},
		{"plaintext", incoming(bob, nil, node.Node{Tag: "plaintext"})},
		{"no id", without(incoming(bob, nil, enc("msg")), "id")},
		{"no timestamp", without(incoming(bob, nil, enc("msg")), "t")},
		{"timestamp not a number", incoming(bob, nil, enc("msg")).With("t", node.Text("soon"))},
		{"no from", without(incoming(bob, nil, enc("msg")), "from")},
		{"from a server", incoming(node.JID{Server: "newsletter"}, nil, enc("msg"))},
		{"recipient from someone else", incoming(bobDev, []node.Attr{{Key: "recipient", Value: node.Address(me)}}, enc("msg"))},
		{"group without participant", incoming(group, nil, enc("skmsg"))},
		{"group from a hosted device", incoming(group, []node.Attr{{Key: "participant", Value: node.Address(hostDev)}}, enc("skmsg"))},
		{"group recipient from someone else", incoming(group, []node.Attr{{Key: "participant", Value: node.Address(bobDev)}, {Key: "recipient", Value: node.Address(me)}}, enc("skmsg"))},
		{"unknown enc type", incoming(bob, nil, enc("frmsg"))},
		{"retry count not a number", incoming(bob, nil, enc("msg", node.Attr{Key: "count", Value: node.Text("x")}))},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := message.ParseIncoming(isMe, tt.in); !errors.Is(err, message.ErrIncoming) {
				t.Fatalf("ParseIncoming() = %v, want %v", err, message.ErrIncoming)
			}
		})
	}
	hostedLID := node.JID{User: "333", Device: 1, Server: node.ServerHostedLID}
	if _, err := message.ParseIncoming(isMe, incoming(group, []node.Attr{{Key: "participant", Value: node.Address(hostedLID)}}, enc("skmsg"))); !errors.Is(err, message.ErrIncoming) {
		t.Fatalf("hosted lid participant = %v", err)
	}
	for _, kind := range []string{"msg", "pkmsg", "skmsg", "msmsg"} {
		if _, err := message.ParseIncoming(isMe, incoming(bob, nil, enc(kind))); err != nil {
			t.Fatalf("enc type %s: %v", kind, err)
		}
	}
	if in, err := message.ParseIncoming(isMe, incoming(node.JID{User: "4", Server: node.ServerBroadcast}, []node.Attr{{Key: "participant", Value: node.Address(bobDev)}}, enc("skmsg"))); err != nil || in.Author != bobDev {
		t.Fatalf("broadcast = %+v, %v", in, err)
	}
}

func TestIncomingEdits(t *testing.T) {
	t.Parallel()
	edit := func(v string) []node.Attr { return []node.Attr{{Key: "edit", Value: node.Text(v)}} }
	for _, tt := range []struct {
		name  string
		attrs []node.Attr
		want  message.Edit
	}{
		{name: "no edit", want: message.EditNone},
		{name: "an edit", attrs: edit("1"), want: message.EditMessage},
		{name: "a pin", attrs: edit("2"), want: message.EditPin},
		{name: "a channel edit", attrs: edit("3"), want: message.EditNewsletter},
		{name: "deleted by its sender", attrs: edit("7"), want: message.EditSenderRevoke},
		{name: "deleted by an admin", attrs: edit("8"), want: message.EditAdminRevoke},
		{name: "an unknown kind is kept", attrs: edit("42"), want: message.Edit(42)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			in, err := message.ParseIncoming(isMe, incoming(bob, tt.attrs, enc("msg")))
			if err != nil || in.Edit != tt.want {
				t.Fatalf("Edit = %d, %v; want %d", in.Edit, err, tt.want)
			}
		})
	}
	if _, err := message.ParseIncoming(isMe, incoming(bob, edit("seven"), enc("msg"))); !errors.Is(err, message.ErrIncoming) {
		t.Fatalf("an edit that is not a number: %v", err)
	}
}

func TestIncomingNamesBothIDsOfAPerson(t *testing.T) {
	t.Parallel()
	bobLID := node.JID{User: "99001", Server: node.ServerLID}
	bobLIDDev := node.JID{User: "99001", Device: 3, Server: node.ServerLID}
	myLID := node.JID{User: "99000", Device: 2, Server: node.ServerLID}
	carolLID := node.JID{User: "99002", Device: 1, Server: node.ServerLID}
	carol := node.JID{User: "40733333333", Server: node.ServerUser}
	addr := func(key string, j node.JID) node.Attr { return node.Attr{Key: key, Value: node.Address(j)} }
	for _, tt := range []struct {
		name string
		from node.JID
		more []node.Attr
		want map[node.JID]node.JID
	}{
		{name: "number with its private id", from: bobDev, more: []node.Attr{addr("sender_lid", bobLIDDev)}, want: map[node.JID]node.JID{bobLID: bob}},
		{name: "private id with its number", from: bobLIDDev, more: []node.Attr{addr("sender_pn", bobDev)}, want: map[node.JID]node.JID{bobLID: bob}},
		{name: "own device naming the recipient", from: myLID, more: []node.Attr{addr("sender_pn", ownDev), addr("recipient", bobLID), addr("peer_recipient_pn", bob)},
			want: map[node.JID]node.JID{bobLID: bob, {User: "99000", Server: node.ServerLID}: me}},
		{name: "group participant", from: group, more: []node.Attr{addr("participant", carolLID), addr("participant_pn", carol)}, want: map[node.JID]node.JID{{User: "99002", Server: node.ServerLID}: carol}},
		{name: "nothing to pair", from: bobDev, want: nil},
		{name: "two numbers are no pair", from: bobDev, more: []node.Attr{addr("sender_pn", bobDev)}, want: nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := message.Pairs(incoming(tt.from, tt.more))
			if len(got) != len(tt.want) {
				t.Fatalf("Pairs = %v, want %v", got, tt.want)
			}
			for lid, pn := range tt.want {
				if got[lid] != pn {
					t.Fatalf("Pairs = %v, want %v", got, tt.want)
				}
			}
		})
	}
	in, err := message.ParseIncoming(isMe, incoming(bobLIDDev, []node.Attr{addr("sender_pn", bobDev)}, enc("msg")))
	if err != nil || in.Pairs[bobLID] != bob {
		t.Fatalf("ParseIncoming pairs = %v, %v", in.Pairs, err)
	}
}
