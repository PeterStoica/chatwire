package client

import (
	"context"
	"fmt"

	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/wire"
)

func (c *Client) SendPeer(ctx context.Context, m *wire.Message) (string, error) {
	phone := c.Self().WithoutDevice()
	if err := c.startSessions(ctx, []node.JID{phone}); err != nil {
		return "", err
	}
	padded, err := message.Encode(c.cfg.Link.Random, m)
	if err != nil {
		return "", err
	}
	c.mu.Lock()
	parts, err := c.sealLocked([]node.JID{phone}, func(node.JID) ([]byte, error) { return padded, nil })
	if err == nil && len(parts) == 0 {
		err = fmt.Errorf("%w: no session with our phone", ErrNoTarget)
	}
	if err == nil {
		err = c.keepLocked()
	}
	c.mu.Unlock()
	if err != nil {
		return "", err
	}
	id, err := message.NewID(c.cfg.Link.Now(), phone, c.cfg.Link.Random)
	if err != nil {
		return "", err
	}
	return id, c.deliver(ctx, message.Peer(id, phone, m, parts[0], c.state.Linked.Account.SignedIdentity))
}
