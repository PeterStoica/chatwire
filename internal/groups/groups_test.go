package groups_test

import (
	"errors"
	"testing"
	"time"

	"github.com/PeterStoica/chatwire/internal/groups"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/testkit/fakegroups"
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

func TestLookingUpOneGroup(t *testing.T) {
	family := groups.Group{JID: node.JID{User: "120363000000000031", Server: node.ServerGroup}, Subject: "Family", Created: time.Unix(1700000000, 0)}
	work := groups.Group{JID: node.JID{User: "120363000000000032", Server: node.ServerGroup}, Subject: "Work", Created: time.Unix(1700000001, 0)}
	request := groups.InfoRequest(family.JID)
	query, ok := request.Child("query")
	if !ok || len(query.Children) != 1 || query.Children[0].Tag != "group" || query.Children[0].Attr("jid").String() != family.JID.String() {
		t.Fatalf("request = %s", request)
	}
	server := fakegroups.New(family, work)
	found, err := groups.ParseInfo(server.Handle(request))
	if err != nil || len(found) != 1 || found[0].JID != family.JID || found[0].Subject != "Family" {
		t.Fatalf("ParseInfo = %+v, %v", found, err)
	}
	if lists, lookups := server.Queries(); lists != 0 || lookups != 1 {
		t.Fatalf("the server saw %d list and %d lookup queries", lists, lookups)
	}
	reply := node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "type", Value: node.Text("result")}}, Children: []node.Node{{Tag: "groups", Children: []node.Node{
		{Tag: "group", Attrs: []node.Attr{{Key: "id", Value: node.Text("120363000000000099")}}, Children: []node.Node{{Tag: "error", Attrs: []node.Attr{{Key: "code", Value: node.Text("403")}}}}},
	}}}}
	if found, err := groups.ParseInfo(reply); err != nil || len(found) != 0 {
		t.Fatalf("a group we are not in: %+v, %v", found, err)
	}
	if _, err := groups.ParseParticipating(reply); !errors.Is(err, groups.ErrReply) {
		t.Fatalf("the full list still rejects it: %v", err)
	}
}
