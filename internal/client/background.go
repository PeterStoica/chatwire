package client

import (
	"context"
	"errors"

	"github.com/PeterStoica/chatwire/internal/history"
	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/wire"
)

func (c *Client) work() {
	for {
		select {
		case job := <-c.jobs:
			job(c.life)
		case <-c.life.Done():
			return
		}
	}
}

func (c *Client) enqueue(job func(context.Context)) bool {
	select {
	case c.jobs <- job:
		return true
	case <-c.life.Done():
		return false
	}
}

func (c *Client) fromOurPhone(in message.Incoming, p *wire.Message_ProtocolMessage) bool {
	if share := p.GetAppStateSyncKeyShare(); share != nil {
		c.addSyncKeys(share)
		c.queueAppStateSync(nil)
	}
	notice := p.GetHistorySyncNotification()
	if notice == nil || c.cfg.History == nil {
		return false
	}
	return c.enqueue(func(ctx context.Context) { c.syncHistory(ctx, in, notice) })
}

func (c *Client) syncHistory(ctx context.Context, in message.Incoming, notice *wire.Message_HistorySyncNotification) {
	chunk, err := c.fetchHistory(ctx, notice)
	if errors.Is(err, ErrGone) {
		c.askForHistoryAgain(ctx, in, notice)
	}
	if err != nil {
		return
	}
	c.Learn(chunk.LIDs)
	c.cfg.History(chunk)
	_ = c.online.Session.Send(ctx, message.DeliveryReceipt(c.mine, in))
	_ = c.online.Session.Send(ctx, message.HistorySyncReceipt(in))
}

func (c *Client) fetchHistory(ctx context.Context, notice *wire.Message_HistorySyncNotification) (history.Chunk, error) {
	data, ref, err := history.Payload(notice)
	if err != nil {
		return history.Chunk{}, err
	}
	if data == nil {
		if data, err = c.Download(ctx, ref); err != nil {
			return history.Chunk{}, err
		}
	}
	inflated, err := history.Inflate(data)
	if err != nil {
		return history.Chunk{}, err
	}
	return history.Parse(inflated, c.Self())
}
