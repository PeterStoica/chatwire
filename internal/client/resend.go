package client

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/prekeys"
	"github.com/PeterStoica/chatwire/internal/signal"
	"github.com/PeterStoica/chatwire/internal/wire"
)

const (
	maxResends    = 5
	recentSends   = 256
	recreateAfter = time.Hour
)

var ErrNotSentHere = errors.New("client: that message was not sent from here")

type sentMessage struct {
	chat    node.JID
	message *wire.Message
}

func (c *Client) rememberLocked(id string, chat node.JID, m *wire.Message) {
	if _, known := c.recent[id]; !known {
		c.recentOrder = append(c.recentOrder, id)
	}
	c.recent[id] = sentMessage{chat: chat, message: m}
	for len(c.recentOrder) > recentSends {
		delete(c.recent, c.recentOrder[0])
		c.recentOrder = c.recentOrder[1:]
	}
}

func (c *Client) chatOf(req message.RetryRequest) node.JID {
	switch {
	case req.Group():
		return req.From
	case c.mine(req.From) && req.Recipient.Server != "":
		return req.Recipient.WithoutDevice()
	}
	return req.From.WithoutDevice()
}

func (c *Client) original(ctx context.Context, chat node.JID, id string) (*wire.Message, error) {
	c.mu.Lock()
	sent, ok := c.recent[id]
	c.mu.Unlock()
	if ok {
		return sent.message, nil
	}
	if c.cfg.Sent != nil {
		if m, ok := c.cfg.Sent(ctx, chat, id); ok {
			return m, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrNotSentHere, id)
}

func (c *Client) resend(ctx context.Context, req message.RetryRequest) error {
	device := req.Device()
	c.mu.Lock()
	key := c.addressLocked(device).String() + "/" + req.ID
	c.resends[key]++
	tries := c.resends[key]
	c.mu.Unlock()
	if tries > maxResends {
		return nil
	}
	chat := c.chatOf(req)
	m, err := c.original(ctx, chat, req.ID)
	if err != nil {
		return err
	}
	payload, err := c.retryPayload(req, chat, m)
	if err != nil {
		return err
	}
	if err := c.sessionForRetry(ctx, device, req); err != nil {
		return err
	}
	padded, err := message.Encode(c.cfg.Link.Random, payload)
	if err != nil {
		return err
	}
	c.mu.Lock()
	parts, err := c.sealLocked([]node.JID{device}, func(node.JID) ([]byte, error) { return padded, nil })
	if err == nil && len(parts) == 0 {
		err = fmt.Errorf("client: no session with %s to answer a retry", device)
	}
	if err == nil {
		err = c.keepLocked()
	}
	c.mu.Unlock()
	if err != nil {
		return err
	}
	return c.deliver(ctx, message.Resend(req, m, parts[0], c.state.Linked.Account.SignedIdentity, c.cfg.Link.Now()))
}

func (c *Client) retryPayload(req message.RetryRequest, chat node.JID, m *wire.Message) (*wire.Message, error) {
	switch {
	case req.Group():
		c.mu.Lock()
		key := c.ownKeys[chat]
		c.mu.Unlock()
		if key == nil {
			return m, nil
		}
		distribution, err := key.Distribution()
		if err != nil {
			return nil, err
		}
		out := proto.CloneOf(m)
		out.SenderKeyDistributionMessage = message.SenderKeyDistribution(chat, distribution).GetSenderKeyDistributionMessage()
		return out, nil
	case c.mine(req.From):
		return message.SentByUs(chat, m), nil
	}
	return m, nil
}

func (c *Client) sessionForRetry(ctx context.Context, device node.JID, req message.RetryRequest) error {
	if req.Keys != nil {
		bundle, err := prekeys.RetryBundle(device, req.Registration, *req.Keys)
		if err != nil {
			return err
		}
		return c.initiate([]prekeys.Bundle{bundle})
	}
	c.mu.Lock()
	address := c.addressLocked(device)
	has, last := c.sessions[address] != nil, c.recreated[address]
	c.mu.Unlock()
	if has && (req.Count < 2 || c.cfg.Link.Now().Sub(last) < recreateAfter) {
		return nil
	}
	return c.fetchSessions(ctx, []node.JID{device})
}

func (c *Client) fetchSessions(ctx context.Context, devices []node.JID) error {
	reply, err := c.online.Session.Query(ctx, prekeys.FetchRequest(devices))
	if err != nil {
		return fmt.Errorf("client: key bundles: %w", err)
	}
	bundles, _, err := prekeys.ParseBundles(reply)
	if err != nil {
		return err
	}
	return c.initiate(bundles)
}

func (c *Client) initiate(bundles []prekeys.Bundle) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, b := range bundles {
		address := c.addressLocked(b.Device)
		session, err := signal.Initiate(c.cfg.Link.Random, c.identity.Signal(), c.sessions[address], b.Keys)
		if err != nil {
			return fmt.Errorf("client: session with %s: %w", b.Device, err)
		}
		c.sessions[address] = session
		c.recreated[address] = c.cfg.Link.Now()
	}
	return nil
}
