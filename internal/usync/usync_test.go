package usync_test

import (
	"errors"
	"testing"

	"github.com/PeterStoica/chatwire/internal/fakeusync"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/usync"
)

var (
	alice = node.JID{User: "40711111111", Server: node.ServerUser}
	carol = node.JID{User: "123456789012345", Server: node.ServerLID}
)

func TestRequestAsksForUsersNotDevices(t *testing.T) {
	request := usync.DevicesRequest("7.8-9", usync.ContextMessage, []node.JID{{User: alice.User, Device: 4, Server: alice.Server}, carol})
	query, _ := request.Child("usync")
	if sid, _ := query.Attr("sid").Text(); sid != "7.8-9" {
		t.Fatalf("sid = %q", sid)
	}
	if context, _ := query.Attr("context").Text(); context != "message" {
		t.Fatalf("context = %q", context)
	}
	list, _ := query.Child("list")
	for i, want := range []node.JID{alice, carol} {
		if got, _ := list.Children[i].Attr("jid").JID(); got != want {
			t.Fatalf("user %d = %v, want %v", i, got, want)
		}
	}
}

func TestDevicesFromTheServer(t *testing.T) {
	server := fakeusync.New()
	server.Set(alice, fakeusync.Device{ID: 0}, fakeusync.Device{ID: 5, KeyIndex: 9})
	server.Set(carol, fakeusync.Device{ID: 0}, fakeusync.Device{ID: 7, Hosted: true})
	stranger := node.JID{User: "40799999999", Server: node.ServerUser}
	users, err := usync.ParseDevices(server.Handle(usync.DevicesRequest("1.1-1", usync.ContextMessage, []node.JID{alice, carol, stranger})))
	if err != nil {
		t.Fatal(err)
	}
	want := map[node.JID][]usync.Device{
		alice: {{JID: alice}, {JID: node.JID{User: alice.User, Device: 5, Server: alice.Server}, KeyIndex: 9}},
		carol: {{JID: carol}, {JID: node.JID{User: carol.User, Device: 7, Server: node.ServerHostedLID}}},
	}
	for _, u := range users[:2] {
		if len(u.Devices) != len(want[u.JID]) || u.Err != nil || len(u.KeyIndexList) == 0 {
			t.Fatalf("%v: %+v", u.JID, u)
		}
		for i, d := range u.Devices {
			if d != want[u.JID][i] {
				t.Fatalf("%v device %d = %+v, want %+v", u.JID, i, d, want[u.JID][i])
			}
		}
	}
	failure, ok := errors.AsType[usync.ServerError](users[2].Err)
	if users[2].JID != stranger || !errors.Is(users[2].Err, usync.ErrDevices) || !ok || failure.Code != 404 {
		t.Fatalf("stranger = %+v", users[2])
	}
}

func TestHostedPhoneNumberDevices(t *testing.T) {
	server := fakeusync.New()
	server.Set(alice, fakeusync.Device{ID: 99, Hosted: true})
	users, err := usync.ParseDevices(server.Handle(usync.DevicesRequest("1.1-1", usync.ContextMessage, []node.JID{alice})))
	if err != nil || users[0].Devices[0].JID != (node.JID{User: alice.User, Device: 99, Server: node.ServerHosted}) {
		t.Fatalf("hosted device = %+v, %v", users, err)
	}
}

func TestMalformedDeviceLists(t *testing.T) {
	server := fakeusync.New()
	server.Set(alice, fakeusync.Device{ID: 1, KeyIndex: 2})
	good := server.Handle(usync.DevicesRequest("1.1-1", usync.ContextMessage, []node.JID{alice}))
	withDevice := func(attrs ...node.Attr) node.Node {
		reply := good
		query, _ := reply.Child("usync")
		list, _ := query.Child("list")
		user := list.Children[0]
		user.Children = []node.Node{{Tag: "devices", Children: []node.Node{{Tag: "device-list", Children: []node.Node{{Tag: "device", Attrs: attrs}}}}}}
		query.Children = []node.Node{query.Children[0], {Tag: "list", Children: []node.Node{user}}}
		reply.Children = []node.Node{query}
		return reply
	}
	for _, tt := range []struct {
		name  string
		reply node.Node
	}{
		{"not a result", node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "type", Value: node.Text("error")}}}},
		{"no usync", node.Node{Tag: "iq", Attrs: []node.Attr{{Key: "type", Value: node.Text("result")}}}},
		{"device id not a number", withDevice(node.Attr{Key: "id", Value: node.Text("one")})},
		{"device id over 255", withDevice(node.Attr{Key: "id", Value: node.Text("256")})},
		{"key index not a number", withDevice(node.Attr{Key: "id", Value: node.Text("1")}, node.Attr{Key: "key-index", Value: node.Text("-1")})},
		{"key index over 32 bits", withDevice(node.Attr{Key: "id", Value: node.Text("1")}, node.Attr{Key: "key-index", Value: node.Text("4294967296")})},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := usync.ParseDevices(tt.reply); !errors.Is(err, usync.ErrReply) {
				t.Fatalf("ParseDevices() = %v, want %v", err, usync.ErrReply)
			}
		})
	}
	protocolFailure := good
	query, _ := protocolFailure.Child("usync")
	query.Children = []node.Node{{Tag: "result", Children: []node.Node{{Tag: "devices", Children: []node.Node{
		{Tag: "error", Attrs: []node.Attr{{Key: "code", Value: node.Text("429")}, {Key: "text", Value: node.Text("rate-overlimit")}}},
	}}}}, query.Children[1]}
	protocolFailure.Children = []node.Node{query}
	_, err := usync.ParseDevices(protocolFailure)
	if failure, ok := errors.AsType[usync.ServerError](err); !errors.Is(err, usync.ErrDevices) || !ok || failure.Code != 429 || failure.Text != "rate-overlimit" {
		t.Fatalf("protocol failure = %v", err)
	}
	largest := withDevice(node.Attr{Key: "id", Value: node.Text("255")}, node.Attr{Key: "key-index", Value: node.Text("4294967295")})
	if users, err := usync.ParseDevices(largest); err != nil || users[0].Devices[0].JID.Device != 255 || users[0].Devices[0].KeyIndex != 4294967295 {
		t.Fatalf("largest device id and key index = %+v, %v", users, err)
	}
}
