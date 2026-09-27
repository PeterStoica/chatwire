package store

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/PeterStoica/chatwire/internal/node"
)

const foldLIDs = `
INSERT INTO messages_text(messages_text, rowid, text)
	SELECT 'delete', m.key, m.text FROM messages m JOIN lids l ON m.chat = l.lid
	WHERE EXISTS (SELECT 1 FROM messages p WHERE p.chat = l.pn AND p.id = m.id);
DELETE FROM messages WHERE key IN (
	SELECT m.key FROM messages m JOIN lids l ON m.chat = l.lid
	WHERE EXISTS (SELECT 1 FROM messages p WHERE p.chat = l.pn AND p.id = m.id));
UPDATE messages SET chat = (SELECT pn FROM lids WHERE lid = messages.chat) WHERE chat IN (SELECT lid FROM lids);
DELETE FROM reactions WHERE rowid IN (
	SELECT r.rowid FROM reactions r JOIN lids l ON r.chat = l.lid
	WHERE EXISTS (SELECT 1 FROM reactions p WHERE p.chat = l.pn AND p.message_id = r.message_id AND p.reactor = r.reactor));
UPDATE reactions SET chat = (SELECT pn FROM lids WHERE lid = reactions.chat) WHERE chat IN (SELECT lid FROM lids);
DELETE FROM reactions WHERE rowid IN (
	SELECT r.rowid FROM reactions r JOIN lids l ON r.reactor = l.lid
	WHERE EXISTS (SELECT 1 FROM reactions p WHERE p.reactor = l.pn AND p.chat = r.chat AND p.message_id = r.message_id));
UPDATE reactions SET reactor = (SELECT pn FROM lids WHERE lid = reactions.reactor) WHERE reactor IN (SELECT lid FROM lids);
DELETE FROM votes WHERE rowid IN (
	SELECT v.rowid FROM votes v JOIN lids l ON v.chat = l.lid
	WHERE EXISTS (SELECT 1 FROM votes p WHERE p.chat = l.pn AND p.message_id = v.message_id AND p.voter = v.voter));
UPDATE votes SET chat = (SELECT pn FROM lids WHERE lid = votes.chat) WHERE chat IN (SELECT lid FROM lids);
DELETE FROM votes WHERE rowid IN (
	SELECT v.rowid FROM votes v JOIN lids l ON v.voter = l.lid
	WHERE EXISTS (SELECT 1 FROM votes p WHERE p.voter = l.pn AND p.chat = v.chat AND p.message_id = v.message_id));
UPDATE votes SET voter = (SELECT pn FROM lids WHERE lid = votes.voter) WHERE voter IN (SELECT lid FROM lids);
INSERT INTO chats (jid, name, last_message, unread, pinned, archived, muted_until)
	SELECT l.pn, c.name, c.last_message, c.unread, c.pinned, c.archived, c.muted_until FROM chats c JOIN lids l ON c.jid = l.lid WHERE true
	ON CONFLICT (jid) DO UPDATE SET
		name = CASE WHEN chats.name = '' THEN excluded.name ELSE chats.name END,
		last_message = max(chats.last_message, excluded.last_message),
		unread = chats.unread + excluded.unread,
		pinned = max(chats.pinned, excluded.pinned),
		archived = max(chats.archived, excluded.archived),
		muted_until = CASE WHEN chats.muted_until = -1 OR excluded.muted_until = -1 THEN -1 ELSE max(chats.muted_until, excluded.muted_until) END;
DELETE FROM chats WHERE jid IN (SELECT lid FROM lids);
INSERT INTO names (jid, contact, first, push)
	SELECT l.pn, n.contact, n.first, n.push FROM names n JOIN lids l ON n.jid = l.lid WHERE true
	ON CONFLICT (jid) DO UPDATE SET
		contact = CASE WHEN names.contact = '' THEN excluded.contact ELSE names.contact END,
		first = CASE WHEN names.first = '' THEN excluded.first ELSE names.first END,
		push = CASE WHEN names.push = '' THEN excluded.push ELSE names.push END;
DELETE FROM names WHERE jid IN (SELECT lid FROM lids);
`

type querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type resolver struct {
	q     querier
	known map[node.JID]node.JID
}

func newResolver(q querier) *resolver {
	return &resolver{q: q, known: map[node.JID]node.JID{}}
}

func (r *resolver) of(ctx context.Context, j node.JID) (node.JID, error) {
	if j.Server != node.ServerLID {
		return j, nil
	}
	bare := j.WithoutDevice()
	if pn, ok := r.known[bare]; ok {
		return pn, nil
	}
	var raw string
	err := r.q.QueryRowContext(ctx, `SELECT pn FROM lids WHERE lid = ?`, bare.String()).Scan(&raw)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		r.known[bare] = bare
		return bare, nil
	case err != nil:
		return node.JID{}, fmt.Errorf("store: number of %s: %w", bare, err)
	}
	pn, err := node.ParseJID(raw)
	if err != nil {
		pn = bare
	}
	r.known[bare] = pn
	return pn, nil
}

func (r *resolver) names(ctx context.Context, in map[node.JID]Name) (map[node.JID]Name, error) {
	if len(in) == 0 {
		return in, nil
	}
	out := make(map[node.JID]Name, len(in))
	for j, n := range in {
		c, err := r.of(ctx, j)
		if err != nil {
			return nil, err
		}
		prev := out[c]
		out[c] = Name{Contact: cmp.Or(n.Contact, prev.Contact), First: cmp.Or(n.First, prev.First), Push: cmp.Or(n.Push, prev.Push)}
	}
	return out, nil
}

func (r *resolver) changes(ctx context.Context, c Changes) (Changes, error) {
	var err error
	one := func(j *node.JID) {
		if err == nil {
			*j, err = r.of(ctx, *j)
		}
	}
	c.Chats = cloned(c.Chats)
	for i := range c.Chats {
		one(&c.Chats[i].JID)
	}
	c.Messages = cloned(c.Messages)
	for i := range c.Messages {
		one(&c.Messages[i].Chat)
		one(&c.Messages[i].Author)
	}
	c.Edits = cloned(c.Edits)
	for i := range c.Edits {
		one(&c.Edits[i].Chat)
	}
	c.Revokes = cloned(c.Revokes)
	for i := range c.Revokes {
		one(&c.Revokes[i].Chat)
	}
	c.Reactions = cloned(c.Reactions)
	for i := range c.Reactions {
		one(&c.Reactions[i].Chat)
		one(&c.Reactions[i].By)
	}
	c.Votes = cloned(c.Votes)
	for i := range c.Votes {
		one(&c.Votes[i].Chat)
		one(&c.Votes[i].By)
	}
	c.Seen = cloned(c.Seen)
	for i := range c.Seen {
		one(&c.Seen[i])
	}
	c.Ticks = cloned(c.Ticks)
	for i := range c.Ticks {
		one(&c.Ticks[i].Chat)
	}
	if err != nil {
		return Changes{}, err
	}
	if c.Unread, err = rekey(ctx, r, c.Unread); err != nil {
		return Changes{}, err
	}
	if c.Names, err = r.names(ctx, c.Names); err != nil {
		return Changes{}, err
	}
	return c, nil
}

func cloned[T any](in []T) []T {
	if in == nil {
		return nil
	}
	return append([]T(nil), in...)
}

func learn(ctx context.Context, tx *sql.Tx, lids map[node.JID]node.JID) error {
	if len(lids) == 0 {
		return nil
	}
	for _, lid := range sortedKeys(lids) {
		pn := lids[lid]
		if _, err := tx.ExecContext(ctx, `INSERT INTO lids (lid, pn) VALUES (?, ?) ON CONFLICT (lid) DO UPDATE SET pn = excluded.pn`, lid.WithoutDevice().String(), pn.WithoutDevice().String()); err != nil {
			return fmt.Errorf("store: lid %s: %w", lid, err)
		}
	}
	if _, err := tx.ExecContext(ctx, foldLIDs); err != nil {
		return fmt.Errorf("store: fold chats kept under a private id: %w", err)
	}
	return nil
}

func sortedKeys(m map[node.JID]node.JID) []node.JID {
	return slices.SortedFunc(maps.Keys(m), func(a, b node.JID) int { return strings.Compare(a.String(), b.String()) })
}

func (s *Store) canonical(ctx context.Context, j node.JID) (node.JID, error) {
	return newResolver(s.db).of(ctx, j)
}

func (r *resolver) sync(ctx context.Context, c SyncChanges) (SyncChanges, error) {
	var err error
	c.Contacts = cloned(c.Contacts)
	for i := range c.Contacts {
		if err == nil {
			c.Contacts[i].JID, err = r.of(ctx, c.Contacts[i].JID)
		}
	}
	if err != nil {
		return SyncChanges{}, err
	}
	if c.Pins, err = rekey(ctx, r, c.Pins); err != nil {
		return SyncChanges{}, err
	}
	if c.Archives, err = rekey(ctx, r, c.Archives); err != nil {
		return SyncChanges{}, err
	}
	if c.Mutes, err = rekey(ctx, r, c.Mutes); err != nil {
		return SyncChanges{}, err
	}
	return c, nil
}

func rekey[V any](ctx context.Context, r *resolver, in map[node.JID]V) (map[node.JID]V, error) {
	if len(in) == 0 {
		return in, nil
	}
	out := make(map[node.JID]V, len(in))
	for j, v := range in {
		c, err := r.of(ctx, j)
		if err != nil {
			return nil, err
		}
		out[c] = v
	}
	return out, nil
}
