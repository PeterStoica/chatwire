package store

import (
	"context"
	"database/sql"
	"encoding/json/v2"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/privacy"
)

type Changes struct {
	Chats     []Chat
	Messages  []Message
	Names     map[node.JID]Name
	LIDs      map[node.JID]node.JID
	Edits     []Edit
	Revokes   []Revoke
	Reactions []React
	Votes     []Vote
	Unread    map[node.JID]int
	Seen      []node.JID
	Ticks     []Tick
	Tokens    []privacy.Token
}

func (s *Store) Apply(ctx context.Context, c Changes) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if err := learn(ctx, tx, c.LIDs); err != nil {
			return err
		}
		c, err := newResolver(tx).changes(ctx, c)
		if err != nil {
			return err
		}
		for _, step := range []func(context.Context, *sql.Tx, Changes) error{applyRecords, applyLifecycle, applyVotes, applyCounts} {
			if err := step(ctx, tx, c); err != nil {
				return err
			}
		}
		return nil
	})
}

func applyRecords(ctx context.Context, tx *sql.Tx, c Changes) error {
	for _, chat := range c.Chats {
		if err := addChat(ctx, tx, chat); err != nil {
			return err
		}
	}
	for _, m := range c.Messages {
		if err := addMessage(ctx, tx, m); err != nil {
			return err
		}
	}
	for jid, n := range c.Names {
		if err := addName(ctx, tx, jid, n); err != nil {
			return err
		}
	}
	for _, t := range c.Tokens {
		if err := addToken(ctx, tx, t); err != nil {
			return err
		}
	}
	return nil
}

func addToken(ctx context.Context, tx *sql.Tx, t privacy.Token) error {
	contact, theirs := t.Contact.WithoutDevice(), t.Theirs
	if theirs == nil {
		theirs = []byte{}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO tokens (jid, theirs, given, ours) VALUES (?, ?, ?, ?) ON CONFLICT (jid) DO UPDATE SET`+keepNewest,
		contact.String(), theirs, unixOrZero(t.Given), unixOrZero(t.Ours)); err != nil {
		return fmt.Errorf("store: token of %s: %w", contact, err)
	}
	return nil
}

func unixOrZero(at time.Time) int64 {
	if at.IsZero() {
		return 0
	}
	return at.Unix()
}

func applyCounts(ctx context.Context, tx *sql.Tx, c Changes) error {
	for chat, count := range c.Unread {
		if _, err := tx.ExecContext(ctx, `INSERT INTO chats (jid, unread) VALUES (?, ?) ON CONFLICT (jid) DO UPDATE SET unread = excluded.unread`, chat.String(), max(count, 0)); err != nil {
			return fmt.Errorf("store: unread of %s: %w", chat, err)
		}
	}
	for _, chat := range c.Seen {
		if _, err := tx.ExecContext(ctx, `UPDATE chats SET unread = 0 WHERE jid = ?`, chat.String()); err != nil {
			return fmt.Errorf("store: seen %s: %w", chat, err)
		}
	}
	for _, tick := range c.Ticks {
		for _, id := range tick.IDs {
			if _, err := tx.ExecContext(ctx, `UPDATE messages SET status = max(status, ?) WHERE chat = ? AND id = ? AND from_me = 1`, int(tick.Status), tick.Chat.String(), id); err != nil {
				return fmt.Errorf("store: status of %s: %w", id, err)
			}
		}
	}
	return nil
}

func applyLifecycle(ctx context.Context, tx *sql.Tx, c Changes) error {
	for _, e := range c.Edits {
		raw, err := proto.Marshal(e.Message)
		if err != nil {
			return fmt.Errorf("store: encode edit of %s: %w", e.ID, err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE messages SET raw = ?, text = ?, edited = ? WHERE chat = ? AND id = ? AND revoked = 0`,
			raw, Text(e.Message), max(e.Time.Unix(), 1), e.Chat.String(), e.ID); err != nil {
			return fmt.Errorf("store: edit %s: %w", e.ID, err)
		}
	}
	for _, r := range c.Revokes {
		if _, err := tx.ExecContext(ctx, `UPDATE messages SET raw = x'', text = '', revoked = 1 WHERE chat = ? AND id = ?`, r.Chat.String(), r.ID); err != nil {
			return fmt.Errorf("store: delete %s: %w", r.ID, err)
		}
		for _, statement := range []string{`DELETE FROM reactions WHERE chat = ? AND message_id = ?`, `DELETE FROM votes WHERE chat = ? AND message_id = ?`} {
			if _, err := tx.ExecContext(ctx, statement, r.Chat.String(), r.ID); err != nil {
				return fmt.Errorf("store: reactions and votes of %s: %w", r.ID, err)
			}
		}
	}
	for _, r := range c.Reactions {
		var err error
		if r.Emoji == "" {
			_, err = tx.ExecContext(ctx, `DELETE FROM reactions WHERE chat = ? AND message_id = ? AND reactor = ?`, r.Chat.String(), r.ID, r.By.WithoutDevice().String())
		} else {
			_, err = tx.ExecContext(ctx, `INSERT INTO reactions (chat, message_id, reactor, emoji, t)
				SELECT ?1, ?2, ?3, ?4, ?5 WHERE NOT EXISTS (SELECT 1 FROM messages WHERE chat = ?1 AND id = ?2 AND revoked = 1)
				ON CONFLICT (chat, message_id, reactor) DO UPDATE SET emoji = excluded.emoji, t = excluded.t WHERE excluded.t >= reactions.t`,
				r.Chat.String(), r.ID, r.By.WithoutDevice().String(), r.Emoji, max(r.Time.Unix(), 0))
		}
		if err != nil {
			return fmt.Errorf("store: reaction to %s: %w", r.ID, err)
		}
	}
	return nil
}

func applyVotes(ctx context.Context, tx *sql.Tx, c Changes) error {
	for _, v := range c.Votes {
		var err error
		if len(v.Options) == 0 {
			_, err = tx.ExecContext(ctx, `DELETE FROM votes WHERE chat = ? AND message_id = ? AND voter = ?`, v.Chat.String(), v.ID, v.By.WithoutDevice().String())
		} else {
			var options []byte
			if options, err = json.Marshal(v.Options); err != nil {
				return fmt.Errorf("store: encode vote on %s: %w", v.ID, err)
			}
			_, err = tx.ExecContext(ctx, `INSERT INTO votes (chat, message_id, voter, options, t)
				SELECT ?1, ?2, ?3, ?4, ?5 WHERE NOT EXISTS (SELECT 1 FROM messages WHERE chat = ?1 AND id = ?2 AND revoked = 1)
				ON CONFLICT (chat, message_id, voter) DO UPDATE SET options = excluded.options, t = excluded.t WHERE excluded.t >= votes.t`,
				v.Chat.String(), v.ID, v.By.WithoutDevice().String(), string(options), max(v.Time.Unix(), 0))
		}
		if err != nil {
			return fmt.Errorf("store: vote on %s: %w", v.ID, err)
		}
	}
	return nil
}

func addChat(ctx context.Context, tx *sql.Tx, c Chat) error {
	if _, err := tx.ExecContext(ctx, `INSERT INTO chats (jid, name, last_message) VALUES (?, ?, ?)
		ON CONFLICT (jid) DO UPDATE SET name = CASE WHEN excluded.name = '' THEN name ELSE excluded.name END, last_message = max(last_message, excluded.last_message)`,
		c.JID.String(), c.Name, max(c.LastMessage.Unix(), 0)); err != nil {
		return fmt.Errorf("store: chat %s: %w", c.JID, err)
	}
	return nil
}

func addName(ctx context.Context, tx *sql.Tx, jid node.JID, n Name) error {
	if _, err := tx.ExecContext(ctx, `INSERT INTO names (jid, contact, first, push) VALUES (?, ?, ?, ?)
		ON CONFLICT (jid) DO UPDATE SET
			contact = CASE WHEN excluded.contact = '' THEN contact ELSE excluded.contact END,
			first = CASE WHEN excluded.first = '' THEN first ELSE excluded.first END,
			push = CASE WHEN excluded.push = '' THEN push ELSE excluded.push END`,
		jid.String(), n.Contact, n.First, n.Push); err != nil {
		return fmt.Errorf("store: name of %s: %w", jid, err)
	}
	return nil
}

func addMessage(ctx context.Context, tx *sql.Tx, m Message) error {
	raw, err := proto.Marshal(m.Message)
	if err != nil {
		return fmt.Errorf("store: encode message %s: %w", m.ID, err)
	}
	chat, seconds := m.Chat.String(), max(m.Time.Unix(), 0)
	values := []any{m.Author.WithoutDevice().String(), m.FromMe, seconds, m.PushName, Text(m.Message), raw, chat, m.ID}
	result, err := tx.ExecContext(ctx, `UPDATE messages SET author = ?, from_me = ?, t = ?, push_name = ?, text = CASE WHEN revoked = 1 OR edited > 0 THEN text ELSE ? END, raw = CASE WHEN revoked = 1 OR edited > 0 THEN raw ELSE ? END WHERE chat = ? AND id = ?`, values...)
	if err != nil {
		return fmt.Errorf("store: update message %s: %w", m.ID, err)
	}
	if updated, err := result.RowsAffected(); err != nil || updated == 0 {
		if err := insertMessage(ctx, tx, m, values, seconds); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO chats (jid, last_message) VALUES (?, ?) ON CONFLICT (jid) DO UPDATE SET last_message = max(last_message, excluded.last_message)`, chat, seconds); err != nil {
		return fmt.Errorf("store: chat of message %s: %w", m.ID, err)
	}
	return nil
}

func insertMessage(ctx context.Context, tx *sql.Tx, m Message, values []any, seconds int64) error {
	key, err := freeKey(ctx, tx, seconds)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO messages (author, from_me, t, push_name, text, raw, chat, id, key) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, append(values, key)...); err != nil {
		return fmt.Errorf("store: insert message %s: %w", m.ID, err)
	}
	if !m.Unread {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO chats (jid, unread) VALUES (?, 1) ON CONFLICT (jid) DO UPDATE SET unread = unread + 1`, m.Chat.String()); err != nil {
		return fmt.Errorf("store: unread of %s: %w", m.Chat, err)
	}
	return nil
}

func freeKey(ctx context.Context, tx *sql.Tx, seconds int64) (int64, error) {
	base := min(seconds, 1<<(63-timeShift)-2) << timeShift
	var last sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT max(key) FROM messages WHERE key >= ? AND key < ?`, base, base+1<<timeShift).Scan(&last); err != nil {
		return 0, fmt.Errorf("store: next key: %w", err)
	}
	if !last.Valid {
		return base, nil
	}
	return last.Int64 + 1, nil
}
