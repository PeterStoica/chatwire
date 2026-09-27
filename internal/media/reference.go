package media

import (
	"cmp"
	"github.com/PeterStoica/chatwire/internal/wire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type Reference struct {
	Type          Type
	DirectPath    string
	MediaKey      []byte
	FileSHA256    []byte
	FileEncSHA256 []byte
	Mimetype      string
	Caption       string
	FileName      string
	Length        uint64
}

type source interface {
	GetDirectPath() string
	GetMediaKey() []byte
	GetFileSha256() []byte
	GetFileEncSha256() []byte
	GetMimetype() string
	GetFileLength() uint64
}

func Unwrap(m *wire.Message) *wire.Message {
	for depth := 0; m != nil && depth < 8; depth++ {
		var inner *wire.Message
		switch {
		case m.GetDeviceSentMessage() != nil:
			inner = m.GetDeviceSentMessage().GetMessage()
		case m.GetEphemeralMessage() != nil:
			inner = m.GetEphemeralMessage().GetMessage()
		case m.GetViewOnceMessage() != nil:
			inner = m.GetViewOnceMessage().GetMessage()
		case m.GetViewOnceMessageV2() != nil:
			inner = m.GetViewOnceMessageV2().GetMessage()
		case m.GetViewOnceMessageV2Extension() != nil:
			inner = m.GetViewOnceMessageV2Extension().GetMessage()
		case m.GetDocumentWithCaptionMessage() != nil:
			inner = m.GetDocumentWithCaptionMessage().GetMessage()
		default:
			return m
		}
		if inner == nil {
			return m
		}
		m = inner
	}
	return m
}

func ReferenceOf(m *wire.Message) (Reference, bool) {
	m = Unwrap(m)
	var (
		src     source
		kind    Type
		caption string
		name    string
	)
	switch {
	case m.GetImageMessage() != nil:
		src, kind, caption = m.GetImageMessage(), Image, m.GetImageMessage().GetCaption()
	case m.GetVideoMessage() != nil:
		src, kind, caption = m.GetVideoMessage(), Video, m.GetVideoMessage().GetCaption()
		if m.GetVideoMessage().GetGifPlayback() {
			kind = GIF
		}
	case m.GetPtvMessage() != nil:
		src, kind = m.GetPtvMessage(), Video
	case m.GetAudioMessage() != nil:
		src, kind = m.GetAudioMessage(), Audio
		if m.GetAudioMessage().GetPtt() {
			kind = Voice
		}
	case m.GetDocumentMessage() != nil:
		src, kind, caption, name = m.GetDocumentMessage(), Document, m.GetDocumentMessage().GetCaption(), cmp.Or(m.GetDocumentMessage().GetFileName(), m.GetDocumentMessage().GetTitle())
	case m.GetStickerMessage() != nil:
		src, kind = m.GetStickerMessage(), Sticker
	default:
		return Reference{}, false
	}
	if src.GetDirectPath() == "" || len(src.GetMediaKey()) != KeySize {
		return Reference{}, false
	}
	return Reference{
		Type: kind, DirectPath: src.GetDirectPath(), MediaKey: src.GetMediaKey(),
		FileSHA256: src.GetFileSha256(), FileEncSHA256: src.GetFileEncSha256(),
		Mimetype: src.GetMimetype(), Caption: caption, FileName: name, Length: src.GetFileLength(),
	}, true
}

func WithDirectPath(m *wire.Message, path string) *wire.Message {
	out, _ := move(m, map[protoreflect.Name]protoreflect.Value{"directPath": protoreflect.ValueOfString(path)})
	return out
}

type Hosted struct {
	URL           string
	DirectPath    string
	MediaKey      []byte
	FileSHA256    []byte
	FileEncSHA256 []byte
	FileLength    uint64
	Stamp         int64
}

func Rehost(m *wire.Message, h Hosted) (*wire.Message, bool) {
	return move(m, map[protoreflect.Name]protoreflect.Value{
		"url": protoreflect.ValueOfString(h.URL), "directPath": protoreflect.ValueOfString(h.DirectPath),
		"mediaKey": protoreflect.ValueOfBytes(h.MediaKey), "fileSha256": protoreflect.ValueOfBytes(h.FileSHA256),
		"fileEncSha256": protoreflect.ValueOfBytes(h.FileEncSHA256), "fileLength": protoreflect.ValueOfUint64(h.FileLength),
		"mediaKeyTimestamp": protoreflect.ValueOfInt64(h.Stamp),
	})
}

func move(m *wire.Message, values map[protoreflect.Name]protoreflect.Value) (*wire.Message, bool) {
	out := proto.CloneOf(m)
	moved := false
	Unwrap(out).ProtoReflect().Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		if field.Message() == nil || field.IsList() || field.IsMap() {
			return true
		}
		sub := value.Message()
		fields := sub.Descriptor().Fields()
		if fields.ByName("directPath") == nil || fields.ByName("mediaKey") == nil {
			return true
		}
		for name, v := range values {
			if f := fields.ByName(name); f != nil {
				sub.Set(f, v)
			}
		}
		moved = true
		return false
	})
	return out, moved
}
