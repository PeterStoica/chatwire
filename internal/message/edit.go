package message

import (
	"google.golang.org/protobuf/proto"

	"github.com/PeterStoica/chatwire/internal/media"
	"github.com/PeterStoica/chatwire/internal/wire"
)

const maxEditWrappers = 3

func editedContent(m *wire.Message) *wire.Message {
	for range maxEditWrappers {
		var inner *wire.Message
		switch {
		case m.GetAssociatedChildMessage() != nil:
			inner = m.GetAssociatedChildMessage().GetMessage()
		case m.GetSpoilerMessage() != nil:
			inner = m.GetSpoilerMessage().GetMessage()
		case m.GetGroupMentionedMessage() != nil:
			inner = m.GetGroupMentionedMessage().GetMessage()
		}
		if inner == nil {
			break
		}
		m = inner
	}
	return m
}

func ApplyEdit(original, edited *wire.Message) (*wire.Message, bool) {
	e := editedContent(edited)
	if e.GetConversation() != "" || e.GetExtendedTextMessage() != nil {
		return &wire.Message{Conversation: e.Conversation, ExtendedTextMessage: e.GetExtendedTextMessage()}, true
	}
	caption, ok := editedCaption(e)
	if !ok || caption == "" {
		return nil, false
	}
	out := proto.CloneOf(original)
	switch target := media.Unwrap(out); {
	case target.GetImageMessage() != nil:
		target.ImageMessage.Caption = &caption
	case target.GetVideoMessage() != nil:
		target.VideoMessage.Caption = &caption
	case target.GetDocumentMessage() != nil:
		target.DocumentMessage.Caption = &caption
	default:
		return nil, false
	}
	return out, true
}

func editedCaption(e *wire.Message) (string, bool) {
	switch {
	case e.GetImageMessage() != nil:
		return e.GetImageMessage().GetCaption(), true
	case e.GetVideoMessage() != nil:
		return e.GetVideoMessage().GetCaption(), true
	case e.GetDocumentMessage() != nil:
		return e.GetDocumentMessage().GetCaption(), true
	case e.GetDocumentWithCaptionMessage() != nil:
		return e.GetDocumentWithCaptionMessage().GetMessage().GetDocumentMessage().GetCaption(), true
	default:
		return "", false
	}
}
