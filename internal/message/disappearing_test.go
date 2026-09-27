package message_test

import (
	"testing"
	"time"

	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/wire"
)

func TestMessagesInADisappearingChatCarryItsTimer(t *testing.T) {
	t.Parallel()
	week := uint32(604800)
	set := time.Unix(1_900_000_000, 0)
	text := message.Disappearing(&wire.Message{Conversation: new("hi")}, week, set)
	if text.GetConversation() != "" || text.GetExtendedTextMessage().GetText() != "hi" ||
		text.GetExtendedTextMessage().GetContextInfo().GetExpiration() != week || text.GetExtendedTextMessage().GetContextInfo().GetEphemeralSettingTimestamp() != set.Unix() {
		t.Fatalf("text = %v", text)
	}
	quoted := &wire.Message{ImageMessage: &wire.Message_ImageMessage{Caption: new("look"), ContextInfo: &wire.ContextInfo{StanzaId: new("3EB0Q")}}}
	photo := message.Disappearing(quoted, week, time.Time{})
	if info := photo.GetImageMessage().GetContextInfo(); info.GetExpiration() != week || info.GetStanzaId() != "3EB0Q" || info.EphemeralSettingTimestamp != nil {
		t.Fatalf("photo context = %v", info)
	}
	if quoted.GetImageMessage().GetContextInfo().Expiration != nil {
		t.Fatal("the original message was changed")
	}
	reaction := &wire.Message{ReactionMessage: &wire.Message_ReactionMessage{Text: new("👍")}}
	if got := message.Disappearing(reaction, week, set); got.GetReactionMessage().GetText() != "👍" || got.GetExtendedTextMessage() != nil {
		t.Fatalf("reaction = %v", got)
	}
	plain := &wire.Message{Conversation: new("stays")}
	if message.Disappearing(plain, 0, set) != plain {
		t.Fatal("a chat without a timer changed the message")
	}
}
