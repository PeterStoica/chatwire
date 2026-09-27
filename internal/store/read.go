package store

import (
	"context"
	"database/sql"
	"encoding/json/v2"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/privacy"
	"github.com/PeterStoica/chatwire/internal/wire"
)

var ErrNoSuchMessage = errors.New("store: no such message")

func (s *Store) Messages(ctx context.Context, q Query) ([]Message, error) {
	chat, err := s.canonical(ctx, q.Chat)
	if err != nil {
		return nil, err
	}
	q.Chat = chat
	var (
		where []string
		args  []any
	)
	from, order := `messages m`, `m.key`
	if text := match(q.Text); text != "" {
		from, order = `messages_text f JOIN messages m ON m.key = f.rowid`, `f.rowid`
		where, args = append(where, `messages_text MATCH ?`), append(args, text)
	}
	if q.Chat.Server != "" {
		where, args = append(where, `m.chat = ?`), append(args, q.Chat.String())
	} else {
		where, args = append(where, `m.chat != ?`), append(args, node.StatusBroadcast().String())
	}
	if q.Incoming {
		where = append(where, `m.from_me = 0`)
	}
	if q.Mine {
		where = append(where, `m.from_me = 1`)
	}
	if len(q.From) > 0 {
		marks := make([]string, 0, len(q.From))
		for _, j := range q.From {
			author, err := s.canonical(ctx, j.WithoutDevice())
			if err != nil {
				return nil, err
			}
			marks, args = append(marks, "?"), append(args, author.String())
		}
		where = append(where, `m.author IN (`+strings.Join(marks, ", ")+`)`)
	}
	if !q.Before.IsZero() {
		where, args = append(where, order+` < ?`), append(args, max(q.Before.Unix(), 0)<<timeShift)
	}
	if q.BeforeID != "" {
		key, err := s.keyOf(ctx, q.Chat, q.BeforeID)
		if err != nil {
			return nil, err
		}
		where, args = append(where, order+` < ?`), append(args, key)
	}
	query := `SELECT m.chat, m.id, m.author, m.from_me, m.t, m.push_name, m.raw, m.edited, m.revoked, m.status FROM ` + from + ` WHERE ` + strings.Join(where, ` AND `)
	out, err := s.messages(ctx, query+` ORDER BY `+order+` DESC LIMIT ?`, append(args, max(q.Limit, 1))...)
	slices.Reverse(out)
	return out, err
}

func (s *Store) Message(ctx context.Context, id string) (Message, bool, error) {
	found, err := s.messages(ctx, `SELECT m.chat, m.id, m.author, m.from_me, m.t, m.push_name, m.raw, m.edited, m.revoked, m.status FROM messages m WHERE m.id = ? ORDER BY m.key DESC LIMIT 1`, id)
	if err != nil || len(found) == 0 {
		return Message{}, false, err
	}
	return found[0], true, nil
}

func (s *Store) Timer(ctx context.Context, chat node.JID) (Timer, error) {
	chat, err := s.canonical(ctx, chat.WithoutDevice())
	if err != nil {
		return Timer{}, err
	}
	t := Timer{Chat: chat}
	var set int64
	err = s.db.QueryRowContext(ctx, `SELECT expiration, expiration_set FROM chats WHERE jid = ?`, chat.String()).Scan(&t.Seconds, &set)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return t, nil
	case err != nil:
		return t, fmt.Errorf("store: disappearing timer of %s: %w", chat, err)
	}
	if set > 0 {
		t.Set = time.Unix(set, 0)
	}
	return t, nil
}

func (s *Store) Token(ctx context.Context, contact node.JID) (privacy.Token, error) {
	contact, err := s.canonical(ctx, contact.WithoutDevice())
	if err != nil {
		return privacy.Token{}, err
	}
	t := privacy.Token{Contact: contact}
	var given, ours int64
	err = s.db.QueryRowContext(ctx, `SELECT theirs, given, ours FROM tokens WHERE jid = ?`, contact.String()).Scan(&t.Theirs, &given, &ours)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return t, nil
	case err != nil:
		return t, fmt.Errorf("store: token of %s: %w", contact, err)
	}
	if given > 0 {
		t.Given = time.Unix(given, 0)
	}
	if ours > 0 {
		t.Ours = time.Unix(ours, 0)
	}
	return t, nil
}

func (s *Store) Oldest(ctx context.Context, chat node.JID) (Message, bool, error) {
	chat, err := s.canonical(ctx, chat)
	if err != nil {
		return Message{}, false, err
	}
	found, err := s.messages(ctx, `SELECT m.chat, m.id, m.author, m.from_me, m.t, m.push_name, m.raw, m.edited, m.revoked, m.status FROM messages m WHERE m.chat = ? ORDER BY m.key LIMIT 1`, chat.String())
	if err != nil || len(found) == 0 {
		return Message{}, false, err
	}
	return found[0], true, nil
}

func (s *Store) MessageIn(ctx context.Context, chat node.JID, id string) (Message, bool, error) {
	chat, err := s.canonical(ctx, chat)
	if err != nil {
		return Message{}, false, err
	}
	found, err := s.messages(ctx, `SELECT m.chat, m.id, m.author, m.from_me, m.t, m.push_name, m.raw, m.edited, m.revoked, m.status FROM messages m WHERE m.chat = ? AND m.id = ?`, chat.String(), id)
	if err != nil || len(found) == 0 {
		return Message{}, false, err
	}
	return found[0], true, nil
}

func (s *Store) messages(ctx context.Context, query string, args ...any) ([]Message, error) {
	var out []Message
	err := s.each(ctx, "messages", query, args, func(rows *sql.Rows) error {
		var (
			m               Message
			chat, author    string
			seconds, edited int64
			raw             []byte
		)
		if err := rows.Scan(&chat, &m.ID, &author, &m.FromMe, &seconds, &m.PushName, &raw, &edited, &m.Revoked, &m.Status); err != nil {
			return err
		}
		m.Chat, _ = node.ParseJID(chat)
		m.Author, _ = node.ParseJID(author)
		m.Time = time.Unix(seconds, 0)
		if edited > 0 {
			m.Edited = time.Unix(edited, 0)
		}
		m.Message = &wire.Message{}
		if err := proto.Unmarshal(raw, m.Message); err != nil {
			return fmt.Errorf("decode message %s: %w", m.ID, err)
		}
		out = append(out, m)
		return nil
	})
	if err != nil || len(out) == 0 {
		return out, err
	}
	return out, s.addOns(ctx, out)
}

func (s *Store) addOns(ctx context.Context, messages []Message) error {
	at := make(map[[2]string]int, len(messages))
	placeholders := make([]string, 0, len(messages))
	args := make([]any, 0, len(messages))
	for i, m := range messages {
		at[[2]string{m.Chat.String(), m.ID}] = i
		placeholders = append(placeholders, "(?, ?)")
		args = append(args, m.Chat.String(), m.ID)
	}
	keys := `(chat, message_id) IN (VALUES ` + strings.Join(placeholders, ", ") + `)`
	err := s.each(ctx, "reactions", `SELECT chat, message_id, reactor, emoji, t FROM reactions WHERE `+keys+` ORDER BY t, reactor`, args, func(rows *sql.Rows) error {
		var (
			chat, id, reactor, emoji string
			seconds                  int64
		)
		if err := rows.Scan(&chat, &id, &reactor, &emoji, &seconds); err != nil {
			return err
		}
		by, _ := node.ParseJID(reactor)
		i := at[[2]string{chat, id}]
		messages[i].Reactions = append(messages[i].Reactions, Reaction{By: by, Emoji: emoji, Time: time.Unix(seconds, 0)})
		return nil
	})
	if err != nil {
		return err
	}
	return s.each(ctx, "votes", `SELECT chat, message_id, voter, options, t FROM votes WHERE `+keys+` ORDER BY t, voter`, args, func(rows *sql.Rows) error {
		var (
			chat, id, voter, options string
			seconds                  int64
		)
		if err := rows.Scan(&chat, &id, &voter, &options, &seconds); err != nil {
			return err
		}
		v := Vote{ID: id, Time: time.Unix(seconds, 0)}
		if err := json.Unmarshal([]byte(options), &v.Options); err != nil {
			return fmt.Errorf("decode vote on %s: %w", id, err)
		}
		v.Chat, _ = node.ParseJID(chat)
		v.By, _ = node.ParseJID(voter)
		i := at[[2]string{chat, id}]
		messages[i].Votes = append(messages[i].Votes, v)
		return nil
	})
}

func (s *Store) each(ctx context.Context, what, query string, args []any, scan func(*sql.Rows) error) error {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("store: %s: %w", what, err)
	}
	defer rows.Close()
	for rows.Next() {
		if err := scan(rows); err != nil {
			return fmt.Errorf("store: read %s: %w", what, err)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("store: read %s: %w", what, err)
	}
	return nil
}

func (s *Store) Chats(ctx context.Context, limit int) ([]Chat, error) {
	var out []Chat
	err := s.each(ctx, "chats", `SELECT jid, name, last_message, unread, pinned, archived, muted_until FROM chats WHERE jid != ? ORDER BY archived, pinned DESC, last_message DESC, jid LIMIT ?`, []any{node.StatusBroadcast().String(), max(limit, 1)}, func(rows *sql.Rows) error {
		var (
			c               Chat
			jid             string
			seconds, pinned int64
		)
		if err := rows.Scan(&jid, &c.Name, &seconds, &c.Unread, &pinned, &c.Archived, &c.Mute); err != nil {
			return err
		}
		c.JID, _ = node.ParseJID(jid)
		c.LastMessage = time.Unix(seconds, 0)
		if pinned > 0 {
			c.Pinned = time.UnixMilli(pinned)
		}
		out = append(out, c)
		return nil
	})
	return out, err
}

func (s *Store) Names(ctx context.Context) (map[node.JID]Name, error) {
	out := map[node.JID]Name{}
	err := s.each(ctx, "names", `SELECT jid, contact, first, push FROM names`, nil, func(rows *sql.Rows) error {
		var (
			jid string
			n   Name
		)
		if err := rows.Scan(&jid, &n.Contact, &n.First, &n.Push); err != nil {
			return err
		}
		if parsed, err := node.ParseJID(jid); err == nil {
			out[parsed] = n
		}
		return nil
	})
	return out, err
}

func (s *Store) LIDs(ctx context.Context) (map[node.JID]node.JID, error) {
	out := map[node.JID]node.JID{}
	err := s.each(ctx, "lids", `SELECT lid, pn FROM lids`, nil, func(rows *sql.Rows) error {
		var lid, pn string
		if err := rows.Scan(&lid, &pn); err != nil {
			return err
		}
		l, lidErr := node.ParseJID(lid)
		p, pnErr := node.ParseJID(pn)
		if lidErr == nil && pnErr == nil {
			out[l] = p
		}
		return nil
	})
	return out, err
}

type Pace struct {
	LastMinute int
	NewChats   int
	Known      bool
}

func (s *Store) Pace(ctx context.Context, chat node.JID, now time.Time) (Pace, error) {
	chat, err := s.canonical(ctx, chat)
	if err != nil {
		return Pace{}, err
	}
	var p Pace
	minute := max(now.Add(-time.Minute).Unix(), 0) << timeShift
	day := max(now.Add(-24*time.Hour).Unix(), 0) << timeShift
	err = s.db.QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM messages WHERE from_me = 1 AND key >= ?1),
		(SELECT COUNT(*) FROM messages m WHERE m.from_me = 1 AND m.key >= ?2 AND (m.chat LIKE ?4 OR m.chat LIKE ?5)
			AND NOT EXISTS (SELECT 1 FROM messages o WHERE o.chat = m.chat AND o.key < m.key)),
		EXISTS (SELECT 1 FROM messages WHERE chat = ?3)`, minute, day, chat.String(), "%@"+string(node.ServerUser), "%@"+string(node.ServerLID)).Scan(&p.LastMinute, &p.NewChats, &p.Known)
	if err != nil {
		return Pace{}, fmt.Errorf("store: pace: %w", err)
	}
	return p, nil
}

func (s *Store) keyOf(ctx context.Context, chat node.JID, id string) (int64, error) {
	query, args := `SELECT key FROM messages WHERE id = ? ORDER BY key DESC LIMIT 1`, []any{id}
	if chat.Server != "" {
		query, args = `SELECT key FROM messages WHERE chat = ? AND id = ?`, []any{chat.String(), id}
	}
	var key int64
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&key)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("%w: %s", ErrNoSuchMessage, id)
	}
	if err != nil {
		return 0, fmt.Errorf("store: find message %s: %w", id, err)
	}
	return key, nil
}
