package groups_test

import (
	"errors"
	"testing"
	"time"

	"github.com/PeterStoica/chatwire/internal/fakegroups"
	"github.com/PeterStoica/chatwire/internal/groups"
	"github.com/PeterStoica/chatwire/internal/node"
)

func TestParticipatingRequestLayout(t *testing.T) {
	request := groups.ParticipatingRequest()
	participating, ok := request.Child("participating")
	if request.Tag != "iq" || !ok || len(participating.Children) != 2 || participating.Children[0].Tag != "participants" || participating.Children[1].Tag != "description" {
		t.Fatalf("request = %s", request)
	}
	for i, want := range []string{"to=g.us", "xmlns=w:g2", "id=", "type=get"} {
		if got := request.Attrs[i].Key + "=" + request.Attrs[i].Value.String(); got != want {
			t.Fatalf("attribute %d = %s, want %s", i, got, want)
		}
	}
}

func TestGroupsRoundTripThroughTheServer(t *testing.T) {
	family := groups.Group{
		JID: node.JID{User: "120363000000000001", Server: node.ServerGroup}, Subject: "Family", Created: time.Unix(1700000000, 0),
		AddressingMode: "lid", Description: "plans",
		Participants: []groups.Participant{
			{JID: node.JID{User: "1", Server: node.ServerLID}, Admin: true},
			{JID: node.JID{User: "40722222222", Server: node.ServerUser}, LID: node.JID{User: "2", Server: node.ServerLID}},
		},
	}
	quiet := groups.Group{JID: node.JID{User: "120363000000000002", Server: node.ServerGroup}, Subject: "Quiet", Created: time.Unix(1, 0)}
	got, err := groups.ParseParticipating(fakegroups.New(family, quiet).Handle(groups.ParticipatingRequest()))
	if err != nil || len(got) != 2 {
		t.Fatalf("ParseParticipating() = %+v, %v", got, err)
	}
	g := got[0]
	if g.JID != family.JID || g.Subject != "Family" || !g.Created.Equal(family.Created) || g.AddressingMode != "lid" || g.Description != "plans" || len(g.Participants) != 2 {
		t.Fatalf("family = %+v", g)
	}
	if !g.Participants[0].Admin || g.Participants[1].Admin || g.Participants[1].LID != family.Participants[1].LID || g.Participants[1].JID != family.Participants[1].JID {
		t.Fatalf("participants = %+v", g.Participants)
	}
	if got[1].Description != "" || len(got[1].Participants) != 0 || got[1].AddressingMode != "" {
		t.Fatalf("quiet = %+v", got[1])
	}
}

func TestParticipantRolesAndIgnoredChildren(t *testing.T) {
	reply := node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "type", Value: node.Text("result")}}, Children: []node.Node{{Tag: "groups", Children: []node.Node{
		{Tag: "note"},
		{Tag: "group", Attrs: []node.Attr{{Key: "id", Value: node.Text("9")}, {Key: "creation", Value: node.Text("5")}}, Children: []node.Node{
			{Tag: "participant", Attrs: []node.Attr{{Key: "jid", Value: node.Address(node.JID{User: "1", Server: node.ServerUser})}, {Key: "type", Value: node.Text("superadmin")}}},
			{Tag: "participant", Attrs: []node.Attr{{Key: "jid", Value: node.Address(node.JID{User: "2", Server: node.ServerUser})}, {Key: "type", Value: node.Text("member")}}},
			{Tag: "description"},
			{Tag: "announcement"},
		}},
	}}}}
	got, err := groups.ParseParticipating(reply)
	if err != nil || len(got) != 1 || !got[0].Participants[0].Admin || got[0].Participants[1].Admin || got[0].Description != "" {
		t.Fatalf("ParseParticipating() = %+v, %v", got, err)
	}
}

func TestMalformedGroupLists(t *testing.T) {
	result := []node.Attr{{Key: "type", Value: node.Text("result")}}
	group := func(attrs []node.Attr, children ...node.Node) node.Node {
		return node.Node{Tag: "iq", Attrs: result, Children: []node.Node{{Tag: "groups", Children: []node.Node{{Tag: "group", Attrs: attrs, Children: children}}}}}
	}
	id, creation := node.Attr{Key: "id", Value: node.Text("9")}, node.Attr{Key: "creation", Value: node.Text("5")}
	for _, tt := range []struct {
		name  string
		reply node.Node
	}{
		{"error", node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "type", Value: node.Text("error")}}, Children: []node.Node{{Tag: "groups"}}}},
		{"not an iq", node.Node{Tag: "message", Attrs: result, Children: []node.Node{{Tag: "groups"}}}},
		{"no groups", node.Node{Tag: "iq", Attrs: result}},
		{"group without id", group([]node.Attr{creation})},
		{"creation not a number", group([]node.Attr{id, {Key: "creation", Value: node.Text("soon")}})},
		{"participant without jid", group([]node.Attr{id, creation}, node.Node{Tag: "participant"})},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := groups.ParseParticipating(tt.reply); !errors.Is(err, groups.ErrReply) {
				t.Fatalf("ParseParticipating() = %v, want %v", err, groups.ErrReply)
			}
		})
	}
}
