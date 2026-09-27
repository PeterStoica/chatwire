package usync_test

import (
	"testing"

	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/testkit/fakeusync"
	"github.com/PeterStoica/chatwire/internal/usync"
)

func TestLookingUpNumbers(t *testing.T) {
	bob := node.JID{User: "40722222222", Server: node.ServerUser}
	bobLID := node.JID{User: "99001", Server: node.ServerLID}
	server := fakeusync.New()
	server.Set(bob, fakeusync.Device{ID: 0})
	server.Profile(bob, bobLID, "Busy building")
	request := usync.ContactsRequest("sid-1", []string{"40722222222", "+40799999999"})
	body, _ := request.Child("usync")
	list, _ := body.Child("list")
	if first, _ := list.Children[0].Child("contact"); string(first.Bytes) != "+40722222222" {
		t.Fatalf("request = %s", request)
	}
	got, err := usync.ParseContacts(server.Handle(request))
	if err != nil || len(got) != 2 {
		t.Fatalf("ParseContacts = %+v, %v", got, err)
	}
	if b := got[0]; !b.OnWhatsApp || b.Number != "40722222222" || b.JID != bob || b.LID != bobLID || b.About != "Busy building" {
		t.Fatalf("bob = %+v", b)
	}
	if n := got[1]; n.OnWhatsApp || n.Number != "40799999999" || n.LID.Server != "" || n.About != "" {
		t.Fatalf("a number not on WhatsApp = %+v", n)
	}
}
