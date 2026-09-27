package message_test

import (
	"bytes"
	"crypto/rand"
	"errors"
	"testing"

	"github.com/PeterStoica/chatwire/internal/device"
	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/prekeys"
)

func TestRetryReceipts(t *testing.T) {
	identity, err := device.New(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	registration := identity.Registration(device.Props())
	fresh, _, err := prekeys.Generate(rand.Reader, 0x010203, 1)
	if err != nil {
		t.Fatal(err)
	}
	reason := message.RetryNoSession
	retry := message.Retry{Count: 1, Registration: registration, PreKey: fresh[0], DeviceIdentity: []byte("adv")}
	for _, tt := range []struct {
		name string
		in   node.Node
		want string
	}{
		{"phone", incoming(bob, nil, enc("msg")), "receipt id=3EB0AA to=40722222222@s.whatsapp.net type=retry"},
		{"group", incoming(group, []node.Attr{{Key: "participant", Value: node.Address(bobDev)}}, enc("skmsg")),
			"receipt id=3EB0AA to=120363000000000000@g.us participant=40722222222:3@s.whatsapp.net type=retry"},
		{"our other device", incoming(ownDev, []node.Attr{{Key: "recipient", Value: node.Address(bob)}}, enc("msg")),
			"receipt id=3EB0AA to=40711111111:2@s.whatsapp.net recipient=40722222222@s.whatsapp.net type=retry"},
		{"peer", incoming(ownDev, []node.Attr{{Key: "category", Value: node.Text("peer")}}, enc("msg")),
			"receipt id=3EB0AA to=40711111111:2@s.whatsapp.net type=retry category=peer"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			in, err := message.ParseIncoming(isMe, tt.in)
			if err != nil {
				t.Fatal(err)
			}
			receipt, err := message.RetryReceipt(isMe, in, retry)
			if err != nil || render(receipt) != tt.want {
				t.Fatalf("retry receipt\n got %s (%v)\nwant %s", render(receipt), err, tt.want)
			}
			to := receipt.Attrs[1].Value
			if _, device := to.JID(); !device || (tt.name == "group") == (to != node.Address(in.From)) {
				t.Fatalf("to is addressed as %#v", to)
			}
		})
	}
	in, err := message.ParseIncoming(isMe, incoming(bob, nil, enc("msg")))
	if err != nil {
		t.Fatal(err)
	}
	first, err := message.RetryReceipt(isMe, in, retry)
	if err != nil || len(first.Children) != 2 || first.Children[0].Tag != "retry" || render(first.Children[0]) != "retry v=1 count=1 id=3EB0AA t=1790000000" {
		t.Fatalf("first retry = %s, %v", first, err)
	}
	if registrationID, _ := first.Child("registration"); !bytes.Equal(registrationID.Bytes, []byte{0, 0, byte(registration.RegistrationID >> 8), byte(registration.RegistrationID)}) {
		t.Fatalf("registration = %x", registrationID.Bytes)
	}
	retry.Count, retry.Reason = 2, &reason
	second, err := message.RetryReceipt(isMe, in, retry)
	if err != nil || render(second.Children[0]) != "retry v=1 count=2 id=3EB0AA t=1790000000 error=1" {
		t.Fatalf("second retry = %s, %v", second, err)
	}
	keys, ok := second.Child("keys")
	oneTime, _ := keys.Child("key")
	id, _ := oneTime.Child("id")
	if !ok || !bytes.Equal(id.Bytes, []byte{1, 2, 3}) || len(keys.Children) != 5 {
		t.Fatalf("keys = %s", keys)
	}
	for i, tag := range []string{"type", "identity", "key", "skey", "device-identity"} {
		if keys.Children[i].Tag != tag {
			t.Fatalf("keys child %d = %s, want %s", i, keys.Children[i].Tag, tag)
		}
	}
	skey, _ := keys.Child("skey")
	skeyID, _ := skey.Child("id")
	if !bytes.Equal(skeyID.Bytes, []byte{0, 0, byte(registration.SignedPreKey.ID)}) {
		t.Fatalf("signed prekey id = %x", skeyID.Bytes)
	}
	own, err := message.ParseIncoming(isMe, incoming(ownDev, nil, enc("msg")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := message.RetryReceipt(isMe, own, retry); !errors.Is(err, message.ErrRetry) {
		t.Fatalf("a retry to our own device without a recipient: %v", err)
	}
}
