package mediaretry_test

import (
	"bytes"
	"crypto/rand"
	"errors"
	"testing"

	"github.com/PeterStoica/chatwire/internal/mediaretry"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/wire"
)

var (
	bob   = node.JID{User: "40722222222", Server: node.ServerUser}
	group = node.JID{User: "120363000000000061", Server: node.ServerGroup}
	lid   = node.JID{User: "111111111111111", Device: 4, Server: node.ServerLID}
)

func TestSealedNotificationsOpenOnlyForTheirMessage(t *testing.T) {
	t.Parallel()
	mediaKey := bytes.Repeat([]byte{7}, 32)
	id := "3EB0AA"
	sealed, iv, err := mediaretry.Seal(rand.Reader, mediaKey, id, &wire.MediaRetryNotification{StanzaId: new(id), DirectPath: new("/v/new"), Result: wire.MediaRetryNotification_SUCCESS.Enum()})
	if err != nil {
		t.Fatal(err)
	}
	got, err := mediaretry.Decrypt(mediaKey, mediaretry.Notification{MessageID: id, Ciphertext: sealed, IV: iv})
	if err != nil || got.GetDirectPath() != "/v/new" || got.GetResult() != wire.MediaRetryNotification_SUCCESS || got.GetStanzaId() != id {
		t.Fatalf("Decrypt() = %v, %v", got, err)
	}
	flipped := bytes.Clone(sealed)
	flipped[0] ^= 1
	for name, n := range map[string]mediaretry.Notification{
		"another message id": {MessageID: "3EB0BB", Ciphertext: sealed, IV: iv},
		"a short iv":         {MessageID: id, Ciphertext: sealed, IV: iv[:11]},
		"a flipped byte":     {MessageID: id, Ciphertext: flipped, IV: iv},
		"nothing":            {MessageID: id},
	} {
		if _, err := mediaretry.Decrypt(mediaKey, n); !errors.Is(err, mediaretry.ErrDecrypt) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := mediaretry.Decrypt(bytes.Repeat([]byte{8}, 32), mediaretry.Notification{MessageID: id, Ciphertext: sealed, IV: iv}); !errors.Is(err, mediaretry.ErrDecrypt) {
		t.Fatalf("another media key: %v", err)
	}
	if _, _, err := mediaretry.Encrypt(bytes.NewReader([]byte{1, 2}), mediaKey, id); err == nil {
		t.Fatal("an iv source that runs dry must fail")
	}
	for _, bad := range [][]byte{nil, mediaKey[:31], append(bytes.Clone(mediaKey), 0)} {
		if _, _, err := mediaretry.Encrypt(rand.Reader, bad, id); !errors.Is(err, mediaretry.ErrKey) {
			t.Errorf("a media key of %d bytes: %v", len(bad), err)
		}
		if _, err := mediaretry.Decrypt(bad, mediaretry.Notification{MessageID: id, Ciphertext: sealed, IV: iv}); !errors.Is(err, mediaretry.ErrKey) {
			t.Errorf("opening with a media key of %d bytes: %v", len(bad), err)
		}
	}
}

func TestRetryRequests(t *testing.T) {
	t.Parallel()
	ct, iv := []byte{1, 2, 3}, []byte{4, 5, 6}
	attrs := func(n node.Node) map[string]string {
		out := map[string]string{}
		for _, a := range n.Attrs {
			out[a.Key], _ = a.Value.Text()
			if j, ok := a.Value.JID(); ok {
				out[a.Key] = j.String()
			}
		}
		return out
	}
	direct := mediaretry.Request(lid, "3EB0AA", mediaretry.Target{Chat: node.JID{User: bob.User, Device: 3, Server: bob.Server}, Participant: bob}, ct, iv)
	rmr, ok := direct.Child("rmr")
	enc, hasEnc := direct.Child("encrypt")
	got := attrs(direct)
	if !ok || !hasEnc || got["type"] != "server-error" || got["to"] != "111111111111111@lid" || got["id"] != "3EB0AA" || attrs(rmr)["jid"] != bob.String() ||
		attrs(rmr)["from_me"] != "false" || attrs(rmr)["participant"] != "" || !bytes.Equal(enc.Children[0].Bytes, ct) || !bytes.Equal(enc.Children[1].Bytes, iv) {
		t.Fatalf("request = %s", direct)
	}
	inGroup := mediaretry.Request(lid, "3EB0AA", mediaretry.Target{Chat: group, FromMe: true, Participant: node.JID{User: "40733333333", Device: 2, Server: node.ServerUser}}, ct, iv)
	rmr, _ = inGroup.Child("rmr")
	if attrs(rmr)["participant"] != "40733333333@s.whatsapp.net" || attrs(rmr)["from_me"] != "true" {
		t.Fatalf("group request = %s", inGroup)
	}
	unknownSender := mediaretry.Request(lid, "3EB0AA", mediaretry.Target{Chat: group}, ct, iv)
	rmr, _ = unknownSender.Child("rmr")
	if _, ok := attrs(rmr)["participant"]; ok {
		t.Fatalf("a group request without a sender = %s", unknownSender)
	}
	history := mediaretry.HistoryRequest(node.JID{User: "40711111111", Device: 5, Server: node.ServerUser}, "3EB0HIST", ct, iv)
	got = attrs(history)
	if _, hasRMR := history.Child("rmr"); hasRMR || got["to"] != "40711111111@s.whatsapp.net" || got["category"] != "peer" || got["type"] != "server-error" || got["id"] != "3EB0HIST" {
		t.Fatalf("history request = %s", history)
	}
}

func TestParsingRetryNotifications(t *testing.T) {
	t.Parallel()
	text := func(k, v string) node.Attr { return node.Attr{Key: k, Value: node.Text(v)} }
	rmr := node.Node{Tag: "rmr", Attrs: []node.Attr{{Key: "jid", Value: node.Address(group)}, text("from_me", "true"), {Key: "participant", Value: node.Address(bob)}}}
	encrypted := node.Node{Tag: "encrypt", Children: []node.Node{{Tag: "enc_p", Bytes: []byte{1}}, {Tag: "enc_iv", Bytes: []byte{2}}}}
	notification := func(kind, id string, children ...node.Node) node.Node {
		return node.Node{Tag: "notification", Attrs: []node.Attr{text("type", kind), text("id", id)}, Children: children}
	}
	got, err := mediaretry.Parse(notification("mediaretry", "3EB0AA", rmr, encrypted))
	if err != nil || got.MessageID != "3EB0AA" || got.Target != (mediaretry.Target{Chat: group, FromMe: true, Participant: bob}) || !bytes.Equal(got.Ciphertext, []byte{1}) || !bytes.Equal(got.IV, []byte{2}) || got.ErrorCode != "" {
		t.Fatalf("Parse() = %+v, %v", got, err)
	}
	failed, err := mediaretry.Parse(notification("mediaretry", "3EB0AA", rmr, node.Node{Tag: "error", Attrs: []node.Attr{text("code", "404")}}))
	if err != nil || failed.ErrorCode != "404" || failed.Ciphertext != nil {
		t.Fatalf("an error notification = %+v, %v", failed, err)
	}
	for name, n := range map[string]node.Node{
		"another type":       notification("devices", "3EB0AA", rmr, encrypted),
		"no id":              notification("mediaretry", "", rmr, encrypted),
		"not a notification": {Tag: "message", Attrs: []node.Attr{text("type", "mediaretry"), text("id", "x")}},
		"no rmr":             notification("mediaretry", "3EB0AA", encrypted),
		"no payload":         notification("mediaretry", "3EB0AA", rmr),
		"no iv":              notification("mediaretry", "3EB0AA", rmr, node.Node{Tag: "encrypt", Children: []node.Node{{Tag: "enc_p", Bytes: []byte{1}}}}),
		"no ciphertext":      notification("mediaretry", "3EB0AA", rmr, node.Node{Tag: "encrypt", Children: []node.Node{{Tag: "enc_iv", Bytes: []byte{1}}}}),
	} {
		if _, err := mediaretry.Parse(n); !errors.Is(err, mediaretry.ErrNotification) {
			t.Errorf("%s: %v", name, err)
		}
	}
}
