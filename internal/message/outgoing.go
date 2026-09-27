package message

import (
	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"

	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/signal"
	"github.com/PeterStoica/chatwire/internal/wire"
)

const (
	TypeText     = "text"
	TypeMedia    = "media"
	TypeReaction = "reaction"
	TypePoll     = "poll"
	TypeEvent    = "event"
	encVersion   = "2"
)

type Part struct {
	Device     node.JID
	Ciphertext signal.Ciphertext
}

func TypeOf(m *wire.Message) string {
	if inner, wrapped := unwrap(m); wrapped {
		if inner == nil {
			return TypeText
		}
		return TypeOf(inner)
	}
	secret := m.GetSecretEncryptedMessage().GetSecretEncType()
	switch {
	case m.GetReactionMessage() != nil || m.GetEncReactionMessage() != nil:
		return TypeReaction
	case m.GetEventMessage() != nil || m.GetEncEventResponseMessage() != nil || secret == wire.Message_SecretEncryptedMessage_EVENT_EDIT:
		return TypeEvent
	case secret == wire.Message_SecretEncryptedMessage_MESSAGE_EDIT:
		return TypeText
	case poll(m) || secret == wire.Message_SecretEncryptedMessage_POLL_EDIT || secret == wire.Message_SecretEncryptedMessage_POLL_ADD_OPTION:
		return TypePoll
	case strings.TrimSpace(m.GetExtendedTextMessage().GetMatchedText()) != "":
		return TypeMedia
	case textual(m):
		return TypeText
	case m.GetPollResultSnapshotMessage() != nil || m.GetPollResultSnapshotMessageV3() != nil:
		return TypeText
	default:
		return TypeMedia
	}
}

func unwrap(m *wire.Message) (*wire.Message, bool) {
	for _, wrapper := range []*wire.Message_FutureProofMessage{m.GetEphemeralMessage(), m.GetGroupMentionedMessage(), m.GetBotInvokeMessage(), m.GetBotForwardedMessage()} {
		if wrapper != nil {
			return wrapper.GetMessage(), true
		}
	}
	if sent := m.GetDeviceSentMessage(); sent != nil {
		return sent.GetMessage(), true
	}
	return nil, false
}

func poll(m *wire.Message) bool {
	return PollOf(m) != nil || m.GetPollUpdateMessage() != nil
}

func textual(m *wire.Message) bool {
	return m.Conversation != nil || m.GetExtendedTextMessage() != nil || m.GetTemplateButtonReplyMessage() != nil ||
		m.GetProtocolMessage() != nil || m.GetInteractiveMessage() != nil || m.GetKeepInChatMessage() != nil ||
		m.GetRequestPhoneNumberMessage() != nil || m.GetEditedMessage() != nil || m.GetPinInChatMessage() != nil ||
		m.GetEncCommentMessage() != nil || m.GetNewsletterAdminInviteMessage() != nil ||
		m.GetNewsletterFollowerInviteMessageV2() != nil || m.GetMessageHistoryNotice() != nil ||
		m.GetAlbumMessage() != nil || m.GetRichResponseMessage() != nil
}

func SentByUs(to node.JID, m *wire.Message) *wire.Message {
	inner := proto.CloneOf(m)
	inner.MessageContextInfo = nil
	return &wire.Message{
		DeviceSentMessage:  &wire.Message_DeviceSentMessage{DestinationJid: new(to.String()), Message: inner},
		MessageContextInfo: m.GetMessageContextInfo(),
	}
}

func EditOf(m *wire.Message) Edit {
	if inner, wrapped := unwrap(m); wrapped {
		return EditOf(inner)
	}
	if p := m.GetProtocolMessage(); p != nil && p.Type != nil {
		if p.GetType() == wire.Message_ProtocolMessage_REVOKE {
			return EditSenderRevoke
		}
		if p.GetType() == wire.Message_ProtocolMessage_MESSAGE_EDIT {
			return EditMessage
		}
	}
	if r := m.GetReactionMessage(); r != nil && r.Text != nil && r.GetText() == "" {
		return EditSenderRevoke
	}
	return EditNone
}

func PollType(m *wire.Message) string {
	if inner, wrapped := unwrap(m); wrapped {
		return PollType(inner)
	}
	switch {
	case PollOf(m) != nil:
		return "creation"
	case m.GetPollUpdateMessage().GetVote() != nil:
		return "vote"
	}
	return ""
}

func header(id string, to node.JID, m *wire.Message) []node.Attr {
	attrs := []node.Attr{{Key: "id", Value: node.Text(id)}, {Key: "to", Value: node.Address(to)}, {Key: attrType, Value: node.Text(TypeOf(m))}}
	if edit := EditOf(m); edit != EditNone {
		attrs = append(attrs, node.Attr{Key: "edit", Value: node.Text(strconv.Itoa(int(edit)))})
	}
	return attrs
}

func meta(m *wire.Message) []node.Node {
	if kind := PollType(m); kind != "" {
		return []node.Node{{Tag: "meta", Attrs: []node.Attr{{Key: "polltype", Value: node.Text(kind)}}}}
	}
	return nil
}

func Outgoing(id string, to node.JID, m *wire.Message, parts []Part, deviceIdentity []byte) node.Node {
	out := node.Node{Tag: tagMessage, Attrs: header(id, to, m)}
	prekey := false
	targets := make([]node.Node, len(parts))
	for i, part := range parts {
		prekey = prekey || part.Ciphertext.Type == signal.TypePreKeyMessage
		targets[i] = node.Node{Tag: "to", Attrs: []node.Attr{{Key: "jid", Value: node.Device(part.Device)}}, Children: []node.Node{encNode(part.Ciphertext, m)}}
	}
	if len(parts) == 1 && parts[0].Device.Device == 0 {
		out.Children = targets[0].Children
	} else {
		out.Children = []node.Node{{Tag: "participants", Children: targets}}
	}
	if prekey && deviceIdentity != nil {
		out.Children = append(out.Children, node.Node{Tag: tagIdentity, Bytes: deviceIdentity})
	}
	out.Children = append(out.Children, meta(m)...)
	return out
}

func encNode(c signal.Ciphertext, m *wire.Message) node.Node {
	kind := "msg"
	if c.Type == signal.TypePreKeyMessage {
		kind = "pkmsg"
	}
	attrs := withMediaType([]node.Attr{{Key: "v", Value: node.Text(encVersion)}, {Key: attrType, Value: node.Text(kind)}}, m)
	if HidesDecryptFailure(m) {
		attrs = append(attrs, node.Attr{Key: "decrypt-fail", Value: node.Text("hide")})
	}
	return node.Node{Tag: "enc", Attrs: attrs, Bytes: c.Bytes}
}

func withMediaType(attrs []node.Attr, m *wire.Message) []node.Attr {
	if kind := MediaType(m); kind != "" {
		return append(attrs, node.Attr{Key: "mediatype", Value: node.Text(kind)})
	}
	return attrs
}

func OutgoingGroup(id string, group node.JID, m *wire.Message, addressingMode string, parts []Part, senderKeyMessage, deviceIdentity []byte) node.Node {
	attrs := header(id, group, m)
	if addressingMode != "" {
		attrs = append(attrs, node.Attr{Key: "addressing_mode", Value: node.Text(addressingMode)})
	}
	out := node.Node{Tag: tagMessage, Attrs: attrs}
	prekey := false
	if len(parts) > 0 {
		targets := make([]node.Node, len(parts))
		for i, part := range parts {
			prekey = prekey || part.Ciphertext.Type == signal.TypePreKeyMessage
			targets[i] = node.Node{Tag: "to", Attrs: []node.Attr{{Key: "jid", Value: node.Device(part.Device)}}, Children: []node.Node{encNode(part.Ciphertext, m)}}
		}
		out.Children = append(out.Children, node.Node{Tag: "participants", Children: targets})
	}
	out.Children = append(out.Children, node.Node{Tag: "enc", Attrs: withMediaType([]node.Attr{{Key: "v", Value: node.Text(encVersion)}, {Key: attrType, Value: node.Text("skmsg")}}, m), Bytes: senderKeyMessage})
	if prekey && deviceIdentity != nil {
		out.Children = append(out.Children, node.Node{Tag: tagIdentity, Bytes: deviceIdentity})
	}
	out.Children = append(out.Children, meta(m)...)
	return out
}

func SenderKeyDistribution(group node.JID, distribution []byte) *wire.Message {
	return &wire.Message{SenderKeyDistributionMessage: &wire.Message_SenderKeyDistributionMessage{
		GroupId: new(group.String()), AxolotlSenderKeyDistributionMessage: distribution,
	}}
}
