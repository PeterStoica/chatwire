package client

import (
	"context"
	"errors"
	"fmt"

	"github.com/PeterStoica/chatwire/internal/media"
	"github.com/PeterStoica/chatwire/internal/mediaretry"
	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/wire"
)

func (c *Client) RetryMedia(ctx context.Context, ref media.Reference, messageID string, target mediaretry.Target) (string, error) {
	ciphertext, iv, err := mediaretry.Encrypt(c.cfg.Link.Random, ref.MediaKey, messageID)
	if err != nil {
		return "", err
	}
	waiter := make(chan mediaretry.Notification, 1)
	c.mu.Lock()
	c.retries[messageID] = waiter
	own := c.state.Linked.Account.LID
	if own.Server == "" {
		own = c.state.Linked.Account.JID
	}
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.retries, messageID)
		c.mu.Unlock()
	}()
	if err := c.online.Session.Send(ctx, mediaretry.Request(own, messageID, target, ciphertext, iv)); err != nil {
		return "", fmt.Errorf("client: ask for the file again: %w", err)
	}
	select {
	case n := <-waiter:
		if n.ErrorCode != "" {
			return "", fmt.Errorf("%w: error %s", ErrRetry, n.ErrorCode)
		}
		answer, err := mediaretry.Decrypt(ref.MediaKey, n)
		if err != nil {
			return "", err
		}
		if answer.GetResult() != wire.MediaRetryNotification_SUCCESS || answer.GetDirectPath() == "" {
			return "", fmt.Errorf("%w: %s", ErrRetry, answer.GetResult())
		}
		return answer.GetDirectPath(), nil
	case <-ctx.Done():
		return "", fmt.Errorf("client: waiting for the phone to upload the file again: %w", ctx.Err())
	case <-c.done:
		return "", ErrClosed
	}
}

func (c *Client) retried(n node.Node) {
	notification, err := mediaretry.Parse(n)
	if err != nil {
		return
	}
	c.mu.Lock()
	waiter, ok := c.retries[notification.MessageID]
	c.mu.Unlock()
	if ok {
		select {
		case waiter <- notification:
		default:
		}
	}
}

func (c *Client) askForHistoryAgain(ctx context.Context, in message.Incoming, notice *wire.Message_HistorySyncNotification) {
	ciphertext, iv, err := mediaretry.Encrypt(c.cfg.Link.Random, notice.GetMediaKey(), in.ID)
	if err != nil {
		return
	}
	_ = c.online.Session.Send(ctx, mediaretry.HistoryRequest(in.Chat, in.ID, ciphertext, iv))
}

func (c *Client) MarkRead(ctx context.Context, chat, sender node.JID, ids []string) error {
	var failures []error
	for _, receipt := range message.ReadReceipts(chat, sender, ids) {
		if err := c.online.Session.Send(ctx, receipt); err != nil {
			failures = append(failures, err)
		}
	}
	if err := errors.Join(failures...); err != nil {
		return fmt.Errorf("client: read receipts: %w", err)
	}
	return nil
}
