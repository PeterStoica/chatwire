package message_test

import (
	"bytes"
	"crypto/rand"
	"errors"
	"testing"

	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/wire"
)

func TestEncryptedEditsOpenOnlyForTheRightMessageAndSender(t *testing.T) {
	secret := bytes.Repeat([]byte{7}, message.SecretSize)
	bob := node.JID{User: "40722222222", Server: node.ServerUser}
	bobLID := node.JID{User: "99001", Server: node.ServerLID}
	a := message.Addon{Secret: secret, ID: "3EB0ORIG", Original: bobLID, Sender: bobLID}
	sealed, err := message.SealEdit(rand.Reader, a, &wire.Message{Conversation: new("fixed typo")})
	if err != nil {
		t.Fatal(err)
	}
	enc := message.EncryptedEdit(sealed)
	if enc == nil || enc.GetTargetMessageKey().GetId() != "3EB0ORIG" {
		t.Fatalf("EncryptedEdit(%v) = %v", sealed, enc)
	}
	opened, err := message.OpenEdit(a, enc)
	if err != nil || opened.GetConversation() != "fixed typo" {
		t.Fatalf("OpenEdit = %v, %v", opened, err)
	}
	for _, wrong := range []message.Addon{
		{Secret: secret, ID: "3EB0OTHER", Original: bobLID, Sender: bobLID},
		{Secret: secret, ID: "3EB0ORIG", Original: bob, Sender: bobLID},
		{Secret: bytes.Repeat([]byte{8}, message.SecretSize), ID: "3EB0ORIG", Original: bobLID, Sender: bobLID},
	} {
		if _, err := message.OpenEdit(wrong, enc); !errors.Is(err, message.ErrEdit) {
			t.Errorf("OpenEdit(%+v) = %v, want ErrEdit", wrong, err)
		}
	}
	if _, err := message.OpenEdit(message.Addon{Secret: []byte{1}, ID: "3EB0ORIG"}, enc); !errors.Is(err, message.ErrSecret) {
		t.Errorf("a short secret: %v", err)
	}
	if message.EncryptedEdit(&wire.Message{SecretEncryptedMessage: &wire.Message_SecretEncryptedMessage{SecretEncType: wire.Message_SecretEncryptedMessage_POLL_EDIT.Enum()}}) != nil {
		t.Error("a poll edit was taken for a message edit")
	}
}

func TestSecretsGoOnContentButNotOnChanges(t *testing.T) {
	secret := bytes.Repeat([]byte{7}, message.SecretSize)
	for _, tt := range []struct {
		name string
		m    *wire.Message
		want bool
	}{
		{name: "text", m: &wire.Message{Conversation: new("hi")}, want: true},
		{name: "photo", m: &wire.Message{ImageMessage: &wire.Message_ImageMessage{}}, want: true},
		{name: "already has one", m: &wire.Message{Conversation: new("hi"), MessageContextInfo: &wire.MessageContextInfo{MessageSecret: secret}}},
		{name: "reaction", m: &wire.Message{ReactionMessage: &wire.Message_ReactionMessage{}}},
		{name: "vote", m: &wire.Message{PollUpdateMessage: &wire.Message_PollUpdateMessage{}}},
		{name: "delete", m: &wire.Message{ProtocolMessage: &wire.Message_ProtocolMessage{Type: wire.Message_ProtocolMessage_REVOKE.Enum()}}},
	} {
		if got := message.NeedsSecret(tt.m); got != tt.want {
			t.Errorf("%s: NeedsSecret = %v", tt.name, got)
		}
	}
	original := &wire.Message{Conversation: new("hi")}
	with := message.WithSecret(original, secret)
	if original.GetMessageContextInfo() != nil || !bytes.Equal(with.GetMessageContextInfo().GetMessageSecret(), secret) || with.GetConversation() != "hi" {
		t.Fatalf("WithSecret changed the original or lost the text: %v, %v", original, with)
	}
	edited, ok := message.ApplyEdit(with, &wire.Message{Conversation: new("hello")})
	if !ok || edited.GetConversation() != "hello" || !bytes.Equal(edited.GetMessageContextInfo().GetMessageSecret(), secret) {
		t.Fatalf("an edit lost the message secret: %v", edited)
	}
}

func TestPollsWrappedAsVersion4AreFound(t *testing.T) {
	poll := message.NewPoll("Lunch?", []string{"Pizza", "Sushi"}, false)
	inner := message.PollOf(poll)
	wrapped := &wire.Message{PollCreationMessageV4: &wire.Message_FutureProofMessage{Message: poll}}
	if got := message.PollOf(wrapped); got == nil || got != inner {
		t.Fatalf("PollOf(v4) = %v", got)
	}
}
