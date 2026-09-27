package message

import (
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/PeterStoica/chatwire/internal/wire"
)

func Disappearing(m *wire.Message, seconds uint32, set time.Time) *wire.Message {
	if seconds == 0 || m == nil {
		return m
	}
	out := proto.CloneOf(m)
	if text := out.GetConversation(); text != "" {
		out.Conversation, out.ExtendedTextMessage = nil, &wire.Message_ExtendedTextMessage{Text: new(text)}
	}
	out.ProtoReflect().Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		if field.Message() == nil || field.IsList() || field.IsMap() {
			return true
		}
		sub := value.Message()
		context := sub.Descriptor().Fields().ByName(fieldContextInfo)
		if context == nil {
			return true
		}
		info, _ := sub.Mutable(context).Message().Interface().(*wire.ContextInfo)
		info.Expiration = new(seconds)
		if !set.IsZero() {
			info.EphemeralSettingTimestamp = new(set.Unix())
		}
		return true
	})
	return out
}
