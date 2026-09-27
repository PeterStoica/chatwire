package message

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/PeterStoica/chatwire/internal/media"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/wire"
)

const fieldContextInfo = "contextInfo"

type Quote struct {
	ID      string
	Author  node.JID
	Chat    node.JID
	Message *wire.Message
}

func Reply(text string, q Quote, fromOtherChat bool) *wire.Message {
	context := &wire.ContextInfo{StanzaId: new(q.ID), Participant: new(q.Author.WithoutDevice().String()), QuotedMessage: quotable(q.Message)}
	if fromOtherChat {
		context.RemoteJid = new(q.Chat.WithoutDevice().String())
	}
	return &wire.Message{ExtendedTextMessage: &wire.Message_ExtendedTextMessage{Text: new(text), ContextInfo: context}}
}

func quotable(m *wire.Message) *wire.Message {
	out := proto.CloneOf(media.Unwrap(m))
	if out == nil {
		return nil
	}
	out.MessageContextInfo = nil
	out.ProtoReflect().Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		if field.Message() == nil || field.IsList() || field.IsMap() {
			return true
		}
		sub := value.Message()
		if context := sub.Descriptor().Fields().ByName(fieldContextInfo); context != nil {
			sub.Clear(context)
		}
		return true
	})
	return out
}

func ContextOf(m *wire.Message) *wire.ContextInfo {
	var found *wire.ContextInfo
	media.Unwrap(m).ProtoReflect().Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		if field.Message() == nil || field.IsList() || field.IsMap() {
			return true
		}
		sub := value.Message()
		context := sub.Descriptor().Fields().ByName(fieldContextInfo)
		if context == nil || !sub.Has(context) {
			return true
		}
		found, _ = sub.Get(context).Message().Interface().(*wire.ContextInfo)
		return found == nil
	})
	return found
}

func QuoteOf(m *wire.Message) (Quote, bool) {
	found := ContextOf(m)
	if found.GetStanzaId() == "" {
		return Quote{}, false
	}
	q := Quote{ID: found.GetStanzaId(), Message: found.GetQuotedMessage()}
	q.Author, _ = node.ParseJID(found.GetParticipant())
	q.Chat, _ = node.ParseJID(found.GetRemoteJid())
	return q, true
}

func Forwarded(m *wire.Message, score uint32) (*wire.Message, bool) {
	inner := media.Unwrap(m)
	if inner.GetConversation() != "" {
		inner = &wire.Message{ExtendedTextMessage: &wire.Message_ExtendedTextMessage{Text: new(inner.GetConversation())}}
	}
	out := proto.CloneOf(inner)
	if out == nil {
		return nil, false
	}
	out.MessageContextInfo = nil
	marked := false
	out.ProtoReflect().Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		if field.Message() == nil || field.IsList() || field.IsMap() {
			return true
		}
		sub := value.Message()
		if context := sub.Descriptor().Fields().ByName(fieldContextInfo); context != nil {
			sub.Set(context, protoreflect.ValueOfMessage((&wire.ContextInfo{IsForwarded: new(true), ForwardingScore: new(score)}).ProtoReflect()))
			marked = true
		}
		return true
	})
	return out, marked
}
