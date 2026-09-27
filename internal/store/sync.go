package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/PeterStoica/chatwire/internal/node"
)

type SyncState struct {
	Version uint64
	Hash    []byte
	MACs    map[string][]byte
}

type Contact struct {
	JID  node.JID
	Name Name
}

func (s *Store) SyncState(ctx context.Context, collection string) (SyncState, error) {
	out := SyncState{MACs: map[string][]byte{}}
	var version int64
	err := s.db.QueryRowContext(ctx, `SELECT version, hash FROM sync_collections WHERE name = ?`, collection).Scan(&version, &out.Hash)
	if errors.Is(err, sql.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return SyncState{}, fmt.Errorf("store: sync state of %s: %w", collection, err)
	}
	out.Version = uint64(max(version, 0))
	err = s.each(ctx, "sync records of "+collection, `SELECT index_mac, value_mac FROM sync_records WHERE collection = ?`, []any{collection}, func(rows *sql.Rows) error {
		var (
			index string
			value []byte
		)
		if err := rows.Scan(&index, &value); err != nil {
			return err
		}
		out.MACs[index] = value
		return nil
	})
	if err != nil {
		return SyncState{}, err
	}
	return out, nil
}

type SyncChanges struct {
	Contacts []Contact
	Pins     map[node.JID]time.Time
	Archives map[node.JID]bool
	Mutes    map[node.JID]Mute
}

func (s *Store) SaveSync(ctx context.Context, collection string, state SyncState, changes SyncChanges) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		changes, err := newResolver(tx).sync(ctx, changes)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO sync_collections (name, version, hash) VALUES (?, ?, ?) ON CONFLICT (name) DO UPDATE SET version = excluded.version, hash = excluded.hash`,
			collection, int64(min(state.Version, 1<<62)), state.Hash); err != nil {
			return fmt.Errorf("store: save sync state of %s: %w", collection, err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM sync_records WHERE collection = ?`, collection); err != nil {
			return fmt.Errorf("store: clear sync records of %s: %w", collection, err)
		}
		for index, value := range state.MACs {
			if _, err := tx.ExecContext(ctx, `INSERT INTO sync_records (collection, index_mac, value_mac) VALUES (?, ?, ?)`, collection, index, value); err != nil {
				return fmt.Errorf("store: save sync record: %w", err)
			}
		}
		for _, c := range changes.Contacts {
			if _, err := tx.ExecContext(ctx, `INSERT INTO names (jid, contact, first) VALUES (?, ?, ?) ON CONFLICT (jid) DO UPDATE SET contact = excluded.contact, first = excluded.first`,
				c.JID.String(), c.Name.Contact, c.Name.First); err != nil {
				return fmt.Errorf("store: contact %s: %w", c.JID, err)
			}
		}
		return applyChatFlags(ctx, tx, changes)
	})
}

func applyChatFlags(ctx context.Context, tx *sql.Tx, changes SyncChanges) error {
	for jid, at := range changes.Pins {
		pinned := int64(0)
		if !at.IsZero() {
			pinned = max(at.UnixMilli(), 1)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO chats (jid, pinned) VALUES (?1, ?2)
			ON CONFLICT (jid) DO UPDATE SET pinned = excluded.pinned, archived = CASE WHEN excluded.pinned > 0 THEN 0 ELSE archived END`, jid.String(), pinned); err != nil {
			return fmt.Errorf("store: pin %s: %w", jid, err)
		}
	}
	for jid, archived := range changes.Archives {
		if _, err := tx.ExecContext(ctx, `INSERT INTO chats (jid, archived) VALUES (?, ?) ON CONFLICT (jid) DO UPDATE SET archived = excluded.archived`, jid.String(), archived); err != nil {
			return fmt.Errorf("store: archive %s: %w", jid, err)
		}
	}
	for jid, mute := range changes.Mutes {
		if _, err := tx.ExecContext(ctx, `INSERT INTO chats (jid, muted_until) VALUES (?, ?) ON CONFLICT (jid) DO UPDATE SET muted_until = excluded.muted_until`, jid.String(), int64(mute)); err != nil {
			return fmt.Errorf("store: mute %s: %w", jid, err)
		}
	}
	return nil
}
