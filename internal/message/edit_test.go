package message_test

import (
	"testing"

	"github.com/PeterStoica/chatwire/internal/media"
	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/wire"
)

func TestApplyingEdits(t *testing.T) {
	t.Parallel()
	photo := &wire.Message{ImageMessage: &wire.Message_ImageMessage{DirectPath: new("/v/photo"), MediaKey: make([]byte, 32), Caption: new("old")}}
	video := &wire.Message{VideoMessage: &wire.Message_VideoMessage{DirectPath: new("/v/video"), MediaKey: make([]byte, 32)}}
	doc := &wire.Message{DocumentWithCaptionMessage: &wire.Message_FutureProofMessage{Message: &wire.Message{DocumentMessage: &wire.Message_DocumentMessage{DirectPath: new("/v/doc"), MediaKey: make([]byte, 32), FileName: new("a.pdf")}}}}
	text := &wire.Message{Conversation: new("helo")}
	wrap := func(kind string, m *wire.Message) *wire.Message {
		fp := &wire.Message_FutureProofMessage{Message: m}
		switch kind {
		case "child":
			return &wire.Message{AssociatedChildMessage: fp}
		case "spoiler":
			return &wire.Message{SpoilerMessage: fp}
		default:
			return &wire.Message{GroupMentionedMessage: fp}
		}
	}
	captionOf := func(m *wire.Message) string {
		inner := media.Unwrap(m)
		return inner.GetImageMessage().GetCaption() + inner.GetVideoMessage().GetCaption() + inner.GetDocumentMessage().GetCaption()
	}
	tests := []struct {
		name     string
		original *wire.Message
		edited   *wire.Message
		ok       bool
		text     string
		caption  string
	}{
		{name: "plain text", original: text, edited: &wire.Message{Conversation: new("hello")}, ok: true, text: "hello"},
		{name: "extended text", original: text, edited: &wire.Message{ExtendedTextMessage: &wire.Message_ExtendedTextMessage{Text: new("see link")}}, ok: true, text: "see link"},
		{name: "photo caption", original: photo, edited: &wire.Message{ImageMessage: &wire.Message_ImageMessage{Caption: new("new")}}, ok: true, caption: "new"},
		{name: "video caption", original: video, edited: &wire.Message{VideoMessage: &wire.Message_VideoMessage{Caption: new("clip")}}, ok: true, caption: "clip"},
		{name: "document caption", original: doc, edited: &wire.Message{DocumentMessage: &wire.Message_DocumentMessage{Caption: new("signed")}}, ok: true, caption: "signed"},
		{name: "document caption in its wrapper", original: doc, edited: &wire.Message{DocumentWithCaptionMessage: &wire.Message_FutureProofMessage{Message: &wire.Message{DocumentMessage: &wire.Message_DocumentMessage{Caption: new("v2")}}}}, ok: true, caption: "v2"},
		{name: "inside an associated child", original: text, edited: wrap("child", &wire.Message{Conversation: new("a")}), ok: true, text: "a"},
		{name: "inside a spoiler", original: text, edited: wrap("spoiler", &wire.Message{Conversation: new("b")}), ok: true, text: "b"},
		{name: "three wrappers deep", original: text, edited: wrap("mention", wrap("spoiler", wrap("child", &wire.Message{Conversation: new("c")}))), ok: true, text: "c"},
		{name: "four wrappers deep is too deep", original: text, edited: wrap("child", wrap("mention", wrap("spoiler", wrap("child", &wire.Message{Conversation: new("d")})))), ok: false},
		{name: "an empty caption", original: photo, edited: &wire.Message{ImageMessage: &wire.Message_ImageMessage{}}, ok: false},
		{name: "a caption for a text", original: text, edited: &wire.Message{ImageMessage: &wire.Message_ImageMessage{Caption: new("x")}}, ok: false},
		{name: "a sticker", original: text, edited: &wire.Message{StickerMessage: &wire.Message_StickerMessage{}}, ok: false},
		{name: "nothing", original: text, edited: nil, ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := message.ApplyEdit(tt.original, tt.edited)
			if ok != tt.ok {
				t.Fatalf("ApplyEdit() ok = %v", ok)
			}
			if !ok {
				return
			}
			if tt.text != "" && got.GetConversation()+got.GetExtendedTextMessage().GetText() != tt.text {
				t.Fatalf("text = %v", got)
			}
			if tt.caption != "" {
				ref, hasMedia := media.ReferenceOf(got)
				if captionOf(got) != tt.caption || !hasMedia || ref.DirectPath == "" {
					t.Fatalf("caption edit lost the media or the caption: %v", got)
				}
				if captionOf(tt.original) == tt.caption {
					t.Fatal("the original was changed")
				}
			}
		})
	}
}
