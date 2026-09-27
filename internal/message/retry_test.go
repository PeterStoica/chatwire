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
	"github.com/PeterStoica/chatwire/internal/signal"
	"github.com/PeterStoica/chatwire/internal/wire"
	"time"
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

func asIncoming(receipt node.Node) node.Node {
	out := receipt
	out.Attrs = append([]node.Attr(nil), receipt.Attrs...)
	for i, a := range out.Attrs {
		if a.Key == "to" {
			out.Attrs[i].Key = "from"
		}
	}
	return out
}

func TestRetryRequestsFromOthersParseAndResend(t *testing.T) {
	identity, err := device.New(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	fresh, _, err := prekeys.Generate(rand.Reader, 7, 1)
	if err != nil {
		t.Fatal(err)
	}
	retry := message.Retry{Count: 2, Registration: identity.Registration(device.Props()), PreKey: fresh[0], DeviceIdentity: []byte("adv")}
	for _, tt := range []struct {
		name       string
		in         node.Node
		device     node.JID
		fanout     bool
		wantEchoed string
	}{
		{"a contact", incoming(bobDev, nil, enc("msg")), bobDev, true, "to=40722222222:3@s.whatsapp.net"},
		{"a group member", incoming(group, []node.Attr{{Key: "participant", Value: node.Address(bobDev)}}, enc("skmsg")), bobDev, false,
			"to=120363000000000000@g.us participant=40722222222:3@s.whatsapp.net"},
		{"our other device", incoming(ownDev, []node.Attr{{Key: "recipient", Value: node.Address(bob)}}, enc("msg")), ownDev, true,
			"to=40711111111:2@s.whatsapp.net recipient=40722222222@s.whatsapp.net"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			in, err := message.ParseIncoming(isMe, tt.in)
			if err != nil {
				t.Fatal(err)
			}
			theirs, err := message.RetryReceipt(isMe, in, retry)
			if err != nil {
				t.Fatal(err)
			}
			req, err := message.ParseRetryRequest(asIncoming(theirs))
			if err != nil || req.ID != "3EB0AA" || req.Count != 2 || req.Device() != tt.device || req.Keys == nil {
				t.Fatalf("ParseRetryRequest = %+v, %v", req, err)
			}
			bundle, err := prekeys.RetryBundle(req.Device(), req.Registration, *req.Keys)
			if err != nil || bundle.Keys.RegistrationID != retry.Registration.RegistrationID || bundle.Keys.PreKey == nil || bundle.Keys.PreKey.ID != 7 ||
				bundle.Keys.Identity != retry.Registration.Identity || string(bundle.DeviceIdentity) != "adv" {
				t.Fatalf("RetryBundle = %+v, %v", bundle, err)
			}
			photo := &wire.Message{ImageMessage: &wire.Message_ImageMessage{Caption: new("sea")}}
			out := message.Resend(req, photo, message.Part{Device: req.Device(), Ciphertext: signal.Ciphertext{Type: signal.TypePreKeyMessage, Bytes: []byte{9}}}, []byte("mine"), time.Unix(1790000100, 0))
			fanout := " device_fanout=false"
			if !tt.fanout {
				fanout = ""
			}
			if want := "message id=3EB0AA type=media t=1790000100 " + tt.wantEchoed + fanout; render(out) != want {
				t.Fatalf("resend\n got %s\nwant %s", render(out), want)
			}
			encNode, _ := out.Child("enc")
			if render(encNode) != "enc v=2 type=pkmsg mediatype=image count=2" {
				t.Fatalf("enc = %s", render(encNode))
			}
			if identity, ok := out.Child("device-identity"); !ok || string(identity.Bytes) != "mine" {
				t.Fatalf("a prekey resend carries no device identity: %s", out)
			}
		})
	}
	for _, bad := range []node.Node{
		{Tag: "receipt", Attrs: []node.Attr{{Key: "id", Value: node.Text("1")}, {Key: "from", Value: node.Address(bob)}, {Key: "type", Value: node.Text("read")}}},
		{Tag: "receipt", Attrs: []node.Attr{{Key: "id", Value: node.Text("1")}, {Key: "from", Value: node.Address(bob)}, {Key: "type", Value: node.Text("retry")}}},
		{Tag: "receipt", Attrs: []node.Attr{{Key: "id", Value: node.Text("1")}, {Key: "from", Value: node.Address(group)}, {Key: "type", Value: node.Text("retry")}},
			Children: []node.Node{{Tag: "retry", Attrs: []node.Attr{{Key: "count", Value: node.Text("1")}}}, {Tag: "registration"}}},
	} {
		if _, err := message.ParseRetryRequest(bad); !errors.Is(err, message.ErrIncoming) {
			t.Fatalf("ParseRetryRequest(%s) = %v", bad, err)
		}
	}
}
