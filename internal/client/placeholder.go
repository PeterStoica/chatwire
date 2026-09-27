package client

import (
	"context"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/wire"
)

const resendWait = 5 * time.Second

func (c *Client) expectResend(in message.Incoming) {
	if c.mine(in.Author) || in.ID == "" {
		return
	}
	key := &wire.MessageKey{RemoteJid: new(in.Chat.String()), FromMe: new(false), Id: new(in.ID)}
	if in.Chat.Server == node.ServerGroup || in.Chat.Server == node.ServerBroadcast {
		key.Participant = new(in.Author.WithoutDevice().String())
	}
	c.mu.Lock()
	_, waiting := c.awaiting[in.ID]
	c.awaiting[in.ID] = key
	c.mu.Unlock()
	if !waiting {
		time.AfterFunc(resendWait, func() { c.enqueue(func(ctx context.Context) { c.askPhoneFor(ctx, in.ID) }) })
	}
}

func (c *Client) readable(id string) {
	c.mu.Lock()
	delete(c.awaiting, id)
	c.mu.Unlock()
}

func (c *Client) askPhoneFor(ctx context.Context, id string) {
	c.mu.Lock()
	key, waiting := c.awaiting[id]
	delete(c.awaiting, id)
	c.mu.Unlock()
	if waiting {
		_, _ = c.SendPeer(ctx, message.ResendRequest(key))
	}
}

func (c *Client) resent(p *wire.Message_ProtocolMessage) {
	response := p.GetPeerDataOperationRequestResponseMessage()
	if response.GetPeerDataOperationRequestType() != wire.Message_PLACEHOLDER_MESSAGE_RESEND || c.cfg.Receive == nil {
		return
	}
	for _, result := range response.GetPeerDataOperationResult() {
		raw := result.GetPlaceholderMessageResendResponse().GetWebMessageInfoBytes()
		info := &wire.MessageInfo{}
		if len(raw) == 0 || proto.Unmarshal(raw, info) != nil {
			continue
		}
		if r, ok := c.fromInfo(info); ok {
			c.readable(r.ID)
			c.cfg.Receive(r)
		}
	}
}

func (c *Client) fromInfo(info *wire.MessageInfo) (Received, bool) {
	key := info.GetKey()
	chat, err := node.ParseJID(key.GetRemoteJid())
	if err != nil || key.GetId() == "" || !hasContent(info.GetMessage()) {
		return Received{}, false
	}
	author := chat
	switch {
	case key.GetFromMe():
		author = c.Self()
	case key.GetParticipant() != "":
		if author, err = node.ParseJID(key.GetParticipant()); err != nil {
			return Received{}, false
		}
	case info.GetParticipant() != "":
		if author, err = node.ParseJID(info.GetParticipant()); err != nil {
			return Received{}, false
		}
	}
	return Received{
		ID: key.GetId(), Chat: chat, Author: author, Time: time.Unix(int64(info.GetMessageTimestamp()), 0),
		Name: info.GetPushName(), Message: info.GetMessage(),
	}, true
}
