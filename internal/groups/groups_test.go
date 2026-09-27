package groups_test

import (
	"errors"
	"slices"
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
		AddressingMode: "lid", Description: "plans", Disappearing: 86400,
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
	if g.JID != family.JID || g.Subject != "Family" || !g.Created.Equal(family.Created) || g.AddressingMode != "lid" || g.Description != "plans" || g.Disappearing != 86400 || len(g.Participants) != 2 {
		t.Fatalf("family = %+v", g)
	}
	if !g.Participants[0].Admin || g.Participants[1].Admin || g.Participants[1].LID != family.Participants[1].LID || g.Participants[1].JID != family.Participants[1].JID {
		t.Fatalf("participants = %+v", g.Participants)
	}
	if got[1].Description != "" || len(got[1].Participants) != 0 || got[1].AddressingMode != "" || got[1].Disappearing != 0 {
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
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := groups.ParseParticipating(tt.reply); !errors.Is(err, groups.ErrReply) {
				t.Fatalf("ParseParticipating() = %v, want %v", err, groups.ErrReply)
			}
		})
	}
	for _, tt := range []struct {
		name  string
		reply node.Node
		check func([]groups.Group) bool
	}{
		{"group without id", group([]node.Attr{creation}), func(g []groups.Group) bool { return len(g) == 0 }},
		{"creation not a number", group([]node.Attr{id, {Key: "creation", Value: node.Text("soon")}}), func(g []groups.Group) bool { return len(g) == 1 && g[0].Created.IsZero() }},
		{"community parent without creation", group([]node.Attr{id}), func(g []groups.Group) bool { return len(g) == 1 }},
		{"participant without jid", group([]node.Attr{id, creation}, node.Node{Tag: "participant"}), func(g []groups.Group) bool { return len(g) == 1 && len(g[0].Participants) == 0 }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if found, err := groups.ParseParticipating(tt.reply); err != nil || !tt.check(found) {
				t.Fatalf("ParseParticipating() = %+v, %v", found, err)
			}
		})
	}
	lidMember := node.Node{Tag: "participant", Attrs: []node.Attr{
		{Key: "jid", Value: node.Address(node.JID{User: "99001", Server: node.ServerLID})},
		{Key: "phone_number", Value: node.Address(node.JID{User: "40722222222", Server: node.ServerUser})},
	}}
	found, err := groups.ParseParticipating(group([]node.Attr{id, creation, {Key: "addressing_mode", Value: node.Text("lid")}}, lidMember))
	if err != nil || len(found) != 1 || len(found[0].Participants) != 1 {
		t.Fatalf("a lid group: %+v, %v", found, err)
	}
	if lid, phone, ok := found[0].Participants[0].Pair(); !ok || lid.User != "99001" || phone.User != "40722222222" {
		t.Fatalf("the member's pair: %v %v %v", lid, phone, ok)
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
	if found, err := groups.ParseParticipating(reply); err != nil || len(found) != 0 {
		t.Fatalf("the full list skips it too: %+v, %v", found, err)
	}
}

func TestChangingAGroup(t *testing.T) {
	me := node.JID{User: "40711111111", Server: node.ServerUser}
	bob := node.JID{User: "40722222222", Server: node.ServerUser}
	carol := node.JID{User: "40733333333", Server: node.ServerUser}
	shy := node.JID{User: "40744444444", Server: node.ServerUser}
	family := groups.Group{JID: node.JID{User: "120363000000000031", Server: node.ServerGroup}, Subject: "Family", Created: time.Unix(1700000000, 0),
		Description: "old rules", DescriptionID: "D7", Participants: []groups.Participant{{JID: me, Admin: true}, {JID: bob}}}
	server := fakegroups.New(family)
	server.Actor, server.InviteOnly = me, map[node.JID]bool{shy: true}

	if reply := server.Handle(groups.SetSubject(family.JID, "Family 2026")); groups.RefusedBy(reply) != nil {
		t.Fatalf("renaming: %s", reply)
	}
	request := groups.SetDescription(family.JID, "D8", "D7", "new rules")
	description, _ := request.Child("description")
	if body, _ := description.Child("body"); description.Attr("prev").String() != "D7" || string(body.Bytes) != "new rules" {
		t.Fatalf("description request = %s", request)
	}
	if reply := server.Handle(request); groups.RefusedBy(reply) != nil {
		t.Fatalf("describing: %s", reply)
	}
	var refused groups.Refused
	if err := groups.RefusedBy(server.Handle(groups.SetDescription(family.JID, "D9", "D7", "stale"))); !errors.As(err, &refused) || refused.Code != 409 {
		t.Fatalf("a description change from a stale copy: %v", err)
	}
	add := groups.ChangeMembers(family.JID, groups.Add, []groups.Member{{JID: carol}, {JID: bob}, {JID: shy}})
	got, err := groups.ParseChange(server.Handle(add), groups.Add)
	want := []groups.Outcome{{JID: carol}, {JID: bob, Code: 409}, {JID: shy, Code: 403}}
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("adding: %+v, %v", got, err)
	}
	if got, err := groups.ParseChange(server.Handle(groups.ChangeMembers(family.JID, groups.Promote, []groups.Member{{JID: carol}})), groups.Promote); err != nil || len(got) != 1 || got[0].Code != 0 {
		t.Fatalf("promoting: %+v, %v", got, err)
	}
	if got, err := groups.ParseChange(server.Handle(groups.ChangeMembers(family.JID, groups.Remove, []groups.Member{{JID: bob}, {JID: shy}})), groups.Remove); err != nil || !slices.Equal(got, []groups.Outcome{{JID: bob}, {JID: shy, Code: 404}}) {
		t.Fatalf("removing: %+v, %v", got, err)
	}
	after, _ := server.Group(family.JID)
	if after.Subject != "Family 2026" || after.Description != "new rules" || after.DescriptionID != "D8" || len(after.Participants) != 2 || !after.Participants[1].Admin {
		t.Fatalf("the group after the changes: %+v", after)
	}
	server.Actor = bob
	if err := groups.RefusedBy(server.Handle(groups.SetSubject(family.JID, "hijacked"))); !errors.As(err, &refused) || refused.Code != 401 {
		t.Fatalf("a change by someone who is not an admin: %v", err)
	}
	server.Actor = me
	server.Handle(groups.Leave(family.JID))
	if after, _ := server.Group(family.JID); len(after.Participants) != 1 {
		t.Fatalf("after leaving: %+v", after.Participants)
	}
}
