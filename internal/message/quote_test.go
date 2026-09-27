package message_test

import (
	"bytes"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/wire"
)

func TestRepliesAndQuotes(t *testing.T) {
	t.Parallel()
	quotedWithItsOwnQuote := &wire.Message{
		DeviceSentMessage: &wire.Message_DeviceSentMessage{Message: &wire.Message{
			ExtendedTextMessage: &wire.Message_ExtendedTextMessage{Text: new("second"), ContextInfo: &wire.ContextInfo{StanzaId: new("3EB0FIRST")}},
		}},
		MessageContextInfo: &wire.MessageContextInfo{},
	}
	reply := message.Reply("third", message.Quote{ID: "3EB0SECOND", Author: bobDev, Chat: group, Message: quotedWithItsOwnQuote}, true)
	context := reply.GetExtendedTextMessage().GetContextInfo()
	quoted := context.GetQuotedMessage()
	if reply.GetExtendedTextMessage().GetText() != "third" || context.GetStanzaId() != "3EB0SECOND" || context.GetParticipant() != bob.String() || context.GetRemoteJid() != group.String() {
		t.Fatalf("reply = %v", reply)
	}
	if quoted.GetDeviceSentMessage() != nil || quoted.GetMessageContextInfo() != nil || quoted.GetExtendedTextMessage().GetContextInfo() != nil || quoted.GetExtendedTextMessage().GetText() != "second" {
		t.Fatalf("the quoted message must be unwrapped and carry no quote of its own: %v", quoted)
	}
	if quotedWithItsOwnQuote.GetDeviceSentMessage().GetMessage().GetExtendedTextMessage().GetContextInfo() == nil {
		t.Fatal("quoting changed the original")
	}
	if inChat := message.Reply("ok", message.Quote{ID: "x", Author: bob, Chat: bob, Message: &wire.Message{Conversation: new("hi")}}, false); inChat.GetExtendedTextMessage().GetContextInfo().RemoteJid != nil {
		t.Fatalf("a reply in the same chat names the chat: %v", inChat)
	}
	if empty := message.Reply("ok", message.Quote{ID: "x", Author: bob}, false); empty.GetExtendedTextMessage().GetContextInfo().GetQuotedMessage() != nil {
		t.Fatalf("a reply to nothing quotes something: %v", empty)
	}

	back, ok := message.QuoteOf(reply)
	if !ok || back.ID != "3EB0SECOND" || back.Author != bob || back.Chat != group || back.Message.GetExtendedTextMessage().GetText() != "second" {
		t.Fatalf("QuoteOf(reply) = %+v, %v", back, ok)
	}
	photoReply := &wire.Message{EphemeralMessage: &wire.Message_FutureProofMessage{Message: &wire.Message{ImageMessage: &wire.Message_ImageMessage{
		Caption: new("like this?"), ContextInfo: &wire.ContextInfo{StanzaId: new("3EB0ASK"), Participant: new(me.String()), QuotedMessage: &wire.Message{Conversation: new("send a photo")}},
	}}}}
	if q, ok := message.QuoteOf(photoReply); !ok || q.ID != "3EB0ASK" || q.Author != me || q.Chat != (node.JID{}) || q.Message.GetConversation() != "send a photo" {
		t.Fatalf("QuoteOf(photo reply) = %+v, %v", q, ok)
	}
	for name, m := range map[string]*wire.Message{
		"plain text":                       {Conversation: new("hi")},
		"a context without a quoted id":    {ExtendedTextMessage: &wire.Message_ExtendedTextMessage{Text: new("hi"), ContextInfo: &wire.ContextInfo{IsForwarded: new(true)}}},
		"a forwarded photo":                {ImageMessage: &wire.Message_ImageMessage{ContextInfo: &wire.ContextInfo{ForwardingScore: new(uint32(2))}}},
		"a poll, whose options are a list": {PollCreationMessage: &wire.Message_PollCreationMessage{Name: new("?"), Options: []*wire.Message_PollCreationMessage_Option{{OptionName: new("a")}}}},
		"nothing":                          nil,
	} {
		if q, ok := message.QuoteOf(m); ok {
			t.Errorf("%s: QuoteOf() = %+v", name, q)
		}
	}
}

func TestTheFirstContextWins(t *testing.T) {
	t.Parallel()
	m := &wire.Message{
		ImageMessage:        &wire.Message_ImageMessage{ContextInfo: &wire.ContextInfo{StanzaId: new("FIRST")}},
		ExtendedTextMessage: &wire.Message_ExtendedTextMessage{ContextInfo: &wire.ContextInfo{StanzaId: new("SECOND")}},
	}
	if q, ok := message.QuoteOf(m); !ok || q.ID != "FIRST" {
		t.Fatalf("QuoteOf() = %+v, %v", q, ok)
	}
	if message.ContextOf(&wire.Message{Conversation: new("hi")}) != nil {
		t.Fatal("a plain text has no context")
	}
}

func TestForwardedCopies(t *testing.T) {
	t.Parallel()
	quoted := &wire.ContextInfo{StanzaId: new("3EB0Q"), Participant: new("40722222222@s.whatsapp.net"), MentionedJid: []string{"40733333333@s.whatsapp.net"}, IsForwarded: new(true), ForwardingScore: new(uint32(2))}
	forwarded := func(score uint32) *wire.ContextInfo {
		return &wire.ContextInfo{IsForwarded: new(true), ForwardingScore: new(score)}
	}
	secret := &wire.MessageContextInfo{MessageSecret: bytes.Repeat([]byte{1}, 32)}
	tests := []struct {
		name string
		in   *wire.Message
		want *wire.Message
	}{
		{name: "plain text", in: &wire.Message{Conversation: new("hi"), MessageContextInfo: secret},
			want: &wire.Message{ExtendedTextMessage: &wire.Message_ExtendedTextMessage{Text: new("hi"), ContextInfo: forwarded(1)}}},
		{name: "a reply loses its quote and mentions", in: &wire.Message{ExtendedTextMessage: &wire.Message_ExtendedTextMessage{Text: new("yes @40733333333"), ContextInfo: quoted}},
			want: &wire.Message{ExtendedTextMessage: &wire.Message_ExtendedTextMessage{Text: new("yes @40733333333"), ContextInfo: forwarded(1)}}},
		{name: "a photo keeps its caption", in: &wire.Message{ImageMessage: &wire.Message_ImageMessage{Caption: new("sea"), DirectPath: new("/v/p"), ContextInfo: quoted}},
			want: &wire.Message{ImageMessage: &wire.Message_ImageMessage{Caption: new("sea"), DirectPath: new("/v/p"), ContextInfo: forwarded(1)}}},
		{name: "a place", in: &wire.Message{LocationMessage: &wire.Message_LocationMessage{DegreesLatitude: new(1.5), Name: new("Home")}},
			want: &wire.Message{LocationMessage: &wire.Message_LocationMessage{DegreesLatitude: new(1.5), Name: new("Home"), ContextInfo: forwarded(1)}}},
		{name: "a wrapped text", in: &wire.Message{EphemeralMessage: &wire.Message_FutureProofMessage{Message: &wire.Message{Conversation: new("gone soon")}}},
			want: &wire.Message{ExtendedTextMessage: &wire.Message_ExtendedTextMessage{Text: new("gone soon"), ContextInfo: forwarded(1)}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			before := proto.CloneOf(tt.in)
			got, ok := message.Forwarded(tt.in, 1)
			if !ok || !proto.Equal(got, tt.want) {
				t.Fatalf("Forwarded() = %v, %v\nwant %v", got, ok, tt.want)
			}
			if !proto.Equal(tt.in, before) {
				t.Fatalf("Forwarded() changed the original: %v", tt.in)
			}
		})
	}
	if got, ok := message.Forwarded(&wire.Message{ReactionMessage: &wire.Message_ReactionMessage{Text: new("👍")}}, 1); ok {
		t.Fatalf("a reaction was forwarded: %v", got)
	}
	if got, ok := message.Forwarded(nil, 1); ok || got != nil {
		t.Fatalf("Forwarded(nil) = %v, %v", got, ok)
	}
}
