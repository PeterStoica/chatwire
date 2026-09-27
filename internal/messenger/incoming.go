package messenger

import (
	"context"
	"time"

	"github.com/PeterStoica/chatwire/internal/client"
	"github.com/PeterStoica/chatwire/internal/history"
	"github.com/PeterStoica/chatwire/internal/media"
	"github.com/PeterStoica/chatwire/internal/message"
	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/pairing"
	"github.com/PeterStoica/chatwire/internal/store"
	"github.com/PeterStoica/chatwire/internal/wire"
)

func (m *Messenger) received(r client.Received) {
	m.dispatch(context.Background(), r)
}

func (m *Messenger) dispatch(ctx context.Context, r client.Received) {
	if len(r.Pairs) > 0 {
		m.keep(ctx, func(ctx context.Context) error { return m.store.Apply(ctx, store.Changes{LIDs: r.Pairs}) })
	}
	inner := media.Unwrap(r.Message)
	switch p := inner.GetProtocolMessage(); {
	case p.GetKey() != nil && p.GetType() == wire.Message_ProtocolMessage_MESSAGE_EDIT:
		m.keep(ctx, func(ctx context.Context) error { return m.edit(ctx, r, p) })
		return
	case p.GetKey() != nil && p.Type != nil && p.GetType() == wire.Message_ProtocolMessage_REVOKE:
		m.keep(ctx, func(ctx context.Context) error { return m.revoke(ctx, r, p) })
		return
	}
	if update := inner.GetPollUpdateMessage(); update != nil {
		m.keep(ctx, func(ctx context.Context) error { return m.vote(ctx, r, update) })
		return
	}
	if reaction := inner.GetReactionMessage(); reaction != nil {
		if reaction.GetKey().GetId() == "" {
			return
		}
		react := store.React{Chat: r.Chat, ID: reaction.GetKey().GetId(), By: r.Author, Emoji: reaction.GetText(), Time: sentAt(reaction.GetSenderTimestampMs(), r.Time)}
		m.keep(ctx, func(ctx context.Context) error {
			return m.store.Apply(ctx, store.Changes{Reactions: []store.React{react}})
		})
		return
	}
	fromMe := m.Mine(r.Author)
	changes := store.Changes{Messages: []store.Message{{ID: r.ID, Chat: r.Chat, Author: r.Author, FromMe: fromMe, Time: r.Time, PushName: r.Name, Message: r.Message, Unread: !fromMe && !r.Chat.IsStatus()}}}
	if r.Name != "" && !fromMe {
		changes.Names = map[node.JID]store.Name{r.Author.WithoutDevice(): {Push: r.Name}}
	}
	m.keep(ctx, func(ctx context.Context) error { return m.store.Apply(ctx, changes) })
}

func (m *Messenger) history(chunk history.Chunk) {
	m.progress(chunk.Type, chunk.Progress)
	chats := make([]store.Chat, 0, len(chunk.Chats))
	for _, c := range chunk.Chats {
		chats = append(chats, store.Chat{JID: c.JID, Name: c.Name, LastMessage: c.LastMessage})
	}
	messages := make([]store.Message, 0, len(chunk.Messages))
	var (
		reactions []store.React
		votes     []store.Vote
	)
	for _, h := range chunk.Messages {
		messages = append(messages, store.Message{ID: h.ID, Chat: h.Chat, Author: h.Author, FromMe: h.FromMe, Time: h.Time, PushName: h.PushName, Message: h.Message})
		for _, r := range h.Reactions {
			reactions = append(reactions, store.React{Chat: h.Chat, ID: h.ID, By: r.By, Emoji: r.Emoji, Time: r.Time})
		}
		for _, v := range h.Votes {
			votes = append(votes, store.Vote{Chat: h.Chat, ID: h.ID, By: v.By, Options: v.Options, Time: v.Time})
		}
	}
	names := make(map[node.JID]store.Name, len(chunk.PushNames)+len(chunk.Contacts))
	for jid, push := range chunk.PushNames {
		names[jid] = store.Name{Push: push}
	}
	for _, c := range chunk.Contacts {
		for _, jid := range []node.JID{c.JID, c.LID} {
			if jid.Server != "" {
				n := names[jid]
				n.Contact, n.First = c.FullName, c.FirstName
				names[jid] = n
			}
		}
	}
	unread := make(map[node.JID]int, len(chunk.Chats))
	for _, c := range chunk.Chats {
		unread[c.JID] = int(c.Unread)
	}
	changes := store.Changes{Chats: chats, Messages: messages, Names: names, LIDs: chunk.LIDs, Reactions: reactions, Votes: votes, Unread: unread}
	m.keep(context.Background(), func(ctx context.Context) error { return m.store.Apply(ctx, changes) })
}

func (m *Messenger) progress(kind wire.HistorySync_HistorySyncType, percent uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if kind == wire.HistorySync_FULL {
		m.synced = HistorySync{Started: true, Percent: 100}
	} else if kind == wire.HistorySync_INITIAL_BOOTSTRAP || kind == wire.HistorySync_RECENT {
		m.synced = HistorySync{Started: true, Percent: min(max(m.synced.Percent, percent), 100)}
	}
}

func (m *Messenger) edit(ctx context.Context, r client.Received, p *wire.Message_ProtocolMessage) error {
	original, ok, err := m.store.MessageIn(ctx, r.Chat, p.GetKey().GetId())
	if err != nil || !ok {
		return err
	}
	if same, err := m.sameSender(ctx, original.Author, r.Author); err != nil || !same {
		return err
	}
	merged, ok := message.ApplyEdit(original.Message, p.GetEditedMessage())
	if !ok {
		return nil
	}
	return m.store.Apply(ctx, store.Changes{Edits: []store.Edit{{Chat: r.Chat, ID: original.ID, Message: merged, Time: sentAt(p.GetTimestampMs(), r.Time)}}})
}

func (m *Messenger) revoke(ctx context.Context, r client.Received, p *wire.Message_ProtocolMessage) error {
	original, ok, err := m.store.MessageIn(ctx, r.Chat, p.GetKey().GetId())
	if err != nil || !ok {
		return err
	}
	same, err := m.sameSender(ctx, original.Author, r.Author)
	if err != nil {
		return err
	}
	byAdmin := r.Edit == message.EditAdminRevoke && r.Chat.Server == node.ServerGroup
	if !same && !byAdmin {
		return nil
	}
	return m.store.Apply(ctx, store.Changes{Revokes: []store.Revoke{{Chat: r.Chat, ID: original.ID}}})
}

func (m *Messenger) vote(ctx context.Context, r client.Received, u *wire.Message_PollUpdateMessage) error {
	poll, ok, err := m.store.MessageIn(ctx, r.Chat, u.GetPollCreationMessageKey().GetId())
	creation := message.PollOf(media.Unwrap(poll.Message))
	if err != nil || !ok || creation == nil {
		return err
	}
	creators, err := m.addresses(ctx, poll.Author)
	if err != nil {
		return err
	}
	voters, err := m.addresses(ctx, r.Author)
	if err != nil {
		return err
	}
	chosen, ok := open(creation, message.Ballot{Secret: poll.Message.GetMessageContextInfo().GetMessageSecret(), PollID: poll.ID}, creators, voters, u.GetVote())
	if !ok {
		return nil
	}
	return m.store.Apply(ctx, store.Changes{Votes: []store.Vote{{Chat: r.Chat, ID: poll.ID, By: r.Author, Options: chosen, Time: sentAt(u.GetSenderTimestampMs(), r.Time)}}})
}

func open(poll *wire.Message_PollCreationMessage, ballot message.Ballot, creators, voters []node.JID, enc *wire.Message_PollEncValue) ([]string, bool) {
	for _, creator := range creators {
		for _, voter := range voters {
			ballot.Creator, ballot.Voter = creator, voter
			if opened, err := message.OpenVote(ballot, enc); err == nil {
				chosen, err := message.Chosen(poll, opened)
				return chosen, err == nil
			}
		}
	}
	return nil, false
}

func (m *Messenger) addresses(ctx context.Context, j node.JID) ([]node.JID, error) {
	j = j.WithoutDevice()
	m.mu.Lock()
	var account pairing.Account
	if m.state != nil {
		account = m.state.Linked.Account
	}
	m.mu.Unlock()
	if account.Owns(j) {
		return []node.JID{account.JID.WithoutDevice(), account.LID.WithoutDevice()}, nil
	}
	lids, err := m.store.LIDs(ctx)
	if err != nil {
		return nil, err
	}
	out := []node.JID{j}
	for lid, pn := range lids {
		switch j {
		case lid:
			out = append(out, pn)
		case pn:
			out = append(out, lid)
		}
	}
	return out, nil
}

func (m *Messenger) receipt(r message.Receipt) {
	var changes store.Changes
	switch {
	case r.Self && (r.Ack == message.AckRead || r.Ack == message.AckPlayed):
		changes.Seen = []node.JID{r.From.WithoutDevice()}
	case r.Self || r.From.Server == node.ServerGroup || r.From.Server == node.ServerBroadcast:
		return
	case r.Ack == message.AckDelivered:
		changes.Ticks = []store.Tick{{Chat: r.From.WithoutDevice(), IDs: r.IDs, Status: store.StatusDelivered}}
	case r.Ack == message.AckRead:
		changes.Ticks = []store.Tick{{Chat: r.From.WithoutDevice(), IDs: r.IDs, Status: store.StatusRead}}
	case r.Ack == message.AckPlayed:
		changes.Ticks = []store.Tick{{Chat: r.From.WithoutDevice(), IDs: r.IDs, Status: store.StatusPlayed}}
	default:
		return
	}
	m.keep(context.Background(), func(ctx context.Context) error { return m.store.Apply(ctx, changes) })
}

func sentAt(ms int64, fallback time.Time) time.Time {
	if ms > 0 {
		return time.UnixMilli(ms)
	}
	return fallback
}

func (m *Messenger) sameSender(ctx context.Context, a, b node.JID) (bool, error) {
	a, b = a.WithoutDevice(), b.WithoutDevice()
	if a == b {
		return true, nil
	}
	lids, err := m.store.LIDs(ctx)
	if err != nil {
		return false, err
	}
	return lids[a] == b || lids[b] == a, nil
}
