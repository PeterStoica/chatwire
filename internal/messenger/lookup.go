package messenger

import (
	"context"

	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/store"
	"github.com/PeterStoica/chatwire/internal/usync"
)

func (m *Messenger) LookUp(ctx context.Context, numbers []string) ([]usync.Contact, error) {
	c, err := m.connected(ctx)
	if err != nil {
		return nil, err
	}
	found, err := c.LookUp(ctx, numbers)
	if err != nil {
		return nil, err
	}
	pairs := map[node.JID]node.JID{}
	for _, f := range found {
		if f.OnWhatsApp && f.LID.Server == node.ServerLID && f.JID.Server == node.ServerUser {
			pairs[f.LID] = f.JID
		}
	}
	if len(pairs) > 0 {
		m.keep(ctx, func(ctx context.Context) error { return m.store.Apply(ctx, store.Changes{LIDs: pairs}) })
	}
	return found, nil
}
