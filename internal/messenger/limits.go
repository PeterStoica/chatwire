package messenger

import (
	"context"
	"fmt"
	"time"

	"github.com/PeterStoica/chatwire/internal/limits"
	"github.com/PeterStoica/chatwire/internal/node"
)

type Timelocked struct {
	Until time.Time
	Kind  string
}

func (t Timelocked) Error() string {
	return fmt.Sprintf("messenger: WhatsApp restricts messages to people who have not messaged this account until %s (%s)", t.Until.Format(time.RFC3339), t.Kind)
}

type CapReached struct {
	Used  int
	Total int
	Until time.Time
}

func (c CapReached) Error() string {
	return fmt.Sprintf("messenger: WhatsApp's allowance of messages to people who have not replied is used up (%d of %d) until %s", c.Used, c.Total, c.Until.Format(time.RFC3339))
}

func (m *Messenger) limitsChanged(n limits.Notice) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if n.Timelock != nil {
		m.timelock = *n.Timelock
	}
	if n.Cap != nil && !n.Cap.SentAt.Before(m.allowance.SentAt) {
		m.allowance = *n.Cap
	}
}

func (m *Messenger) limitedFor(ctx context.Context, to node.JID) error {
	if to.Server != node.ServerUser && to.Server != node.ServerLID {
		return nil
	}
	now := m.link.Now()
	m.mu.Lock()
	lock, allowance := m.timelock, m.allowance
	m.mu.Unlock()
	locked, capped := lock.On(now), allowance.Reached(now)
	if !locked && !capped || m.tokenOf(ctx, to).Usable(now) {
		return nil
	}
	if locked {
		return Timelocked{Until: lock.Ends, Kind: lock.Kind}
	}
	return CapReached{Used: allowance.Used, Total: allowance.Total, Until: allowance.Ends}
}
