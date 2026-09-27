package messenger

import (
	"context"
	"time"

	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/store"
	"github.com/PeterStoica/chatwire/internal/wire"
)

func (m *Messenger) Older(ctx context.Context, chat node.JID) (int, error) {
	c, err := m.connected(ctx)
	if err != nil {
		return 0, err
	}
	return m.older(ctx, chat, func(ctx context.Context, request *wire.Message) error {
		_, err := c.SendPeer(ctx, request)
		return err
	})
}

func (m *Messenger) older(ctx context.Context, chat node.JID, ask func(context.Context, *wire.Message) error) (int, error) {
	oldest, ok, err := m.store.Oldest(ctx, chat)
	if err != nil || !ok {
		return 0, err
	}
	m.mu.Lock()
	arrived := m.onDemand
	m.mu.Unlock()
	request := message.HistoryRequest(message.Older{
		Chat: oldest.Chat, OldestID: oldest.ID, OldestMine: oldest.FromMe, OldestAt: oldest.Time.UnixMilli(), Count: olderCount,
	})
	if err := ask(ctx, request); err != nil {
		return 0, err
	}
	wait, cancel := context.WithTimeout(ctx, olderWait)
	defer cancel()
	for {
		select {
		case <-wait.Done():
			return 0, nil
		case <-arrived:
		}
		earlier, err := m.store.Messages(ctx, store.Query{Chat: chat, Before: oldest.Time.Add(time.Second), Limit: olderCount + 1})
		if err != nil {
			return 0, err
		}
		if n := countBefore(earlier, oldest); n > 0 {
			return n, nil
		}
		m.mu.Lock()
		arrived = m.onDemand
		m.mu.Unlock()
	}
}

func countBefore(messages []store.Message, oldest store.Message) int {
	n := 0
	for _, msg := range messages {
		if msg.ID != oldest.ID && !msg.Time.After(oldest.Time) {
			n++
		}
	}
	return n
}
