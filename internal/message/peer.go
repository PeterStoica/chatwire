package message

import (
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/signal"
	"github.com/PeterStoica/chatwire/internal/wire"
)

func Peer(id string, phone node.JID, m *wire.Message, part Part, deviceIdentity []byte) node.Node {
	attrs := append(header(id, phone, m),
		node.Attr{Key: "category", Value: node.Text(categoryPeer)}, node.Attr{Key: "push_priority", Value: node.Text("high")})
	out := node.Node{Tag: tagMessage, Attrs: attrs, Children: []node.Node{encNode(part.Ciphertext, m)}}
	if part.Ciphertext.Type == signal.TypePreKeyMessage && deviceIdentity != nil {
		out.Children = append(out.Children, node.Node{Tag: tagIdentity, Bytes: deviceIdentity})
	}
	out.Children = append(out.Children, node.Node{Tag: "meta", Attrs: []node.Attr{{Key: "appdata", Value: node.Text("default")}}})
	return out
}

func KeyRequest(ids [][]byte) *wire.Message {
	keys := make([]*wire.Message_AppStateSyncKeyId, 0, len(ids))
	for _, id := range ids {
		keys = append(keys, &wire.Message_AppStateSyncKeyId{KeyId: id})
	}
	return &wire.Message{ProtocolMessage: &wire.Message_ProtocolMessage{
		Type:                   wire.Message_ProtocolMessage_APP_STATE_SYNC_KEY_REQUEST.Enum(),
		AppStateSyncKeyRequest: &wire.Message_AppStateSyncKeyRequest{KeyIds: keys},
	}}
}

func ResendRequest(keys ...*wire.MessageKey) *wire.Message {
	wanted := make([]*wire.Message_PeerDataOperationRequestMessage_PlaceholderMessageResendRequest, 0, len(keys))
	for _, k := range keys {
		wanted = append(wanted, &wire.Message_PeerDataOperationRequestMessage_PlaceholderMessageResendRequest{MessageKey: k})
	}
	return peerRequest(&wire.Message_PeerDataOperationRequestMessage{
		PeerDataOperationRequestType:    wire.Message_PLACEHOLDER_MESSAGE_RESEND.Enum(),
		PlaceholderMessageResendRequest: wanted,
	})
}

type Older struct {
	Chat       node.JID
	OldestID   string
	OldestMine bool
	OldestAt   int64
	Count      int32
	AccountLID node.JID
}

func HistoryRequest(o Older) *wire.Message {
	request := &wire.Message_PeerDataOperationRequestMessage_HistorySyncOnDemandRequest{
		ChatJid: new(o.Chat.WithoutDevice().String()), OldestMsgId: &o.OldestID, OldestMsgFromMe: &o.OldestMine,
		OnDemandMsgCount: &o.Count, OldestMsgTimestampMs: &o.OldestAt,
	}
	if o.AccountLID.Server != "" {
		request.AccountLid = new(o.AccountLID.WithoutDevice().String())
	}
	return peerRequest(&wire.Message_PeerDataOperationRequestMessage{
		PeerDataOperationRequestType: wire.Message_HISTORY_SYNC_ON_DEMAND.Enum(),
		HistorySyncOnDemandRequest:   request,
	})
}

func peerRequest(r *wire.Message_PeerDataOperationRequestMessage) *wire.Message {
	return &wire.Message{ProtocolMessage: &wire.Message_ProtocolMessage{
		Type:                            wire.Message_ProtocolMessage_PEER_DATA_OPERATION_REQUEST_MESSAGE.Enum(),
		PeerDataOperationRequestMessage: r,
	}}
}
