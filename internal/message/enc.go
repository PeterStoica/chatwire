package message

import "github.com/PeterStoica/chatwire/internal/wire"

const maxMediaWrappers = 8

func mediaInner(m *wire.Message) *wire.Message {
	for _, wrapper := range []*wire.Message_FutureProofMessage{
		m.GetEphemeralMessage(), m.GetGroupMentionedMessage(), m.GetViewOnceMessageV2Extension(), m.GetViewOnceMessage(),
		m.GetDocumentWithCaptionMessage(), m.GetSpoilerMessage(), m.GetBotInvokeMessage(), m.GetPollCreationOptionImageMessage(),
		m.GetQuestionMessage(), m.GetQuestionReplyMessage(),
	} {
		if wrapper != nil {
			return wrapper.GetMessage()
		}
	}
	if sent := m.GetDeviceSentMessage(); sent != nil {
		return sent.GetMessage()
	}
	if child := m.GetAssociatedChildMessage(); child != nil {
		return child.GetMessage()
	}
	return nil
}

func MediaType(m *wire.Message) string {
	for range maxMediaWrappers {
		inner := mediaInner(m)
		if inner == nil {
			break
		}
		m = inner
	}
	switch {
	case m.GetImageMessage() != nil:
		return "image"
	case m.GetStickerMessage() != nil, m.GetLottieStickerMessage() != nil:
		return "sticker"
	case m.GetStickerPackMessage() != nil:
		return "sticker_pack"
	case m.GetLocationMessage() != nil && m.GetLocationMessage().GetIsLive():
		return "livelocation"
	case m.GetLocationMessage() != nil:
		return "location"
	case m.GetContactMessage() != nil:
		return "vcard"
	case m.GetContactsArrayMessage() != nil:
		return "contact_array"
	case m.GetDocumentMessage() != nil:
		return "document"
	case m.GetAudioMessage() != nil && m.GetAudioMessage().GetPtt():
		return "ptt"
	case m.GetAudioMessage() != nil:
		return "audio"
	case m.GetVideoMessage() != nil && m.GetVideoMessage().GetGifPlayback():
		return "gif"
	case m.GetVideoMessage() != nil:
		return "video"
	case m.GetPtvMessage() != nil:
		return "ptv"
	case m.GetButtonsMessage() != nil:
		return "button"
	case m.GetButtonsResponseMessage() != nil:
		return "button_response"
	case m.GetListMessage() != nil:
		return "list"
	case m.GetListResponseMessage() != nil:
		return "list_response"
	case m.GetOrderMessage() != nil:
		return "order"
	case m.GetProductMessage() != nil:
		return "product"
	case m.GetGroupInviteMessage() != nil:
		return "url"
	case m.GetInteractiveResponseMessage() != nil:
		return "native_flow_response"
	case m.GetExtendedTextMessage().GetMatchedText() != "" && trimmed(m.GetExtendedTextMessage().GetMatchedText()):
		return "url"
	}
	return ""
}

func trimmed(s string) bool {
	for _, r := range s {
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			return true
		}
	}
	return false
}

func HidesDecryptFailure(m *wire.Message) bool {
	if inner, wrapped := unwrap(m); wrapped {
		m = inner
	}
	switch {
	case m.GetReactionMessage() != nil, m.GetEncReactionMessage() != nil, m.GetPollUpdateMessage().GetVote() != nil,
		m.GetKeepInChatMessage() != nil, m.GetEditedMessage() != nil, m.GetPinInChatMessage() != nil,
		m.GetEncEventResponseMessage() != nil, m.GetMessageHistoryNotice() != nil:
		return true
	}
	p := m.GetProtocolMessage()
	if p == nil {
		return false
	}
	switch p.GetType() {
	case wire.Message_ProtocolMessage_EPHEMERAL_SYNC_RESPONSE, wire.Message_ProtocolMessage_REQUEST_WELCOME_MESSAGE,
		wire.Message_ProtocolMessage_GROUP_MEMBER_LABEL_CHANGE, wire.Message_ProtocolMessage_MESSAGE_UNSCHEDULE,
		wire.Message_ProtocolMessage_AI_METADATA_OPERATION:
		return true
	}
	return p.GetEditedMessage() != nil
}
