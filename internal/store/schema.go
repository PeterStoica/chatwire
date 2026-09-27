package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var ErrNewer = errors.New("store: made by a newer version")

const schemaV1 = `
CREATE TABLE IF NOT EXISTS chats (
	jid TEXT PRIMARY KEY,
	name TEXT NOT NULL DEFAULT '',
	last_message INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS messages (
	key INTEGER PRIMARY KEY,
	chat TEXT NOT NULL,
	id TEXT NOT NULL,
	author TEXT NOT NULL,
	from_me INTEGER NOT NULL,
	t INTEGER NOT NULL,
	push_name TEXT NOT NULL,
	text TEXT NOT NULL,
	raw BLOB NOT NULL,
	UNIQUE (chat, id)
);
CREATE INDEX IF NOT EXISTS messages_by_chat ON messages (chat, key);
CREATE INDEX IF NOT EXISTS messages_by_id ON messages (id);
CREATE VIRTUAL TABLE IF NOT EXISTS messages_text USING fts5(text, content='messages', content_rowid='key', tokenize='unicode61 remove_diacritics 2');
CREATE TRIGGER IF NOT EXISTS messages_added AFTER INSERT ON messages BEGIN
	INSERT INTO messages_text(rowid, text) VALUES (new.key, new.text);
END;
CREATE TRIGGER IF NOT EXISTS messages_changed AFTER UPDATE OF text ON messages BEGIN
	INSERT INTO messages_text(messages_text, rowid, text) VALUES ('delete', old.key, old.text);
	INSERT INTO messages_text(rowid, text) VALUES (new.key, new.text);
END;
CREATE TABLE IF NOT EXISTS names (
	jid TEXT PRIMARY KEY,
	contact TEXT NOT NULL DEFAULT '',
	first TEXT NOT NULL DEFAULT '',
	push TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS lids (
	lid TEXT PRIMARY KEY,
	pn TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS sync_collections (
	name TEXT PRIMARY KEY,
	version INTEGER NOT NULL,
	hash BLOB NOT NULL
);
CREATE TABLE IF NOT EXISTS sync_records (
	collection TEXT NOT NULL,
	index_mac TEXT NOT NULL,
	value_mac BLOB NOT NULL,
	PRIMARY KEY (collection, index_mac)
);
`

const schemaV2 = `
ALTER TABLE messages ADD COLUMN edited INTEGER NOT NULL DEFAULT 0;
ALTER TABLE messages ADD COLUMN revoked INTEGER NOT NULL DEFAULT 0;
CREATE TABLE reactions (
	chat TEXT NOT NULL,
	message_id TEXT NOT NULL,
	reactor TEXT NOT NULL,
	emoji TEXT NOT NULL,
	t INTEGER NOT NULL,
	PRIMARY KEY (chat, message_id, reactor)
);
`

const schemaV3 = `
ALTER TABLE chats ADD COLUMN unread INTEGER NOT NULL DEFAULT 0;
ALTER TABLE messages ADD COLUMN status INTEGER NOT NULL DEFAULT 0;
`

const schemaV4 = `
CREATE TABLE votes (
	chat TEXT NOT NULL,
	message_id TEXT NOT NULL,
	voter TEXT NOT NULL,
	options TEXT NOT NULL,
	t INTEGER NOT NULL,
	PRIMARY KEY (chat, message_id, voter)
);
`

const schemaV5 = `
ALTER TABLE chats ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0;
ALTER TABLE chats ADD COLUMN archived INTEGER NOT NULL DEFAULT 0;
ALTER TABLE chats ADD COLUMN muted_until INTEGER NOT NULL DEFAULT 0;
`

const schemaV7 = `
CREATE TABLE tokens (
	jid TEXT PRIMARY KEY,
	theirs BLOB NOT NULL DEFAULT x'',
	given INTEGER NOT NULL DEFAULT 0,
	ours INTEGER NOT NULL DEFAULT 0
);
`

const schemaV8 = `
CREATE INDEX IF NOT EXISTS lids_by_pn ON lids (pn);
`

const schemaV9 = `
UPDATE messages SET author = (SELECT pn FROM lids WHERE lid = messages.author) WHERE author IN (SELECT lid FROM lids);
`

const schemaV10 = `
ALTER TABLE chats ADD COLUMN expiration INTEGER NOT NULL DEFAULT 0;
ALTER TABLE chats ADD COLUMN expiration_set INTEGER NOT NULL DEFAULT 0;
`

func migrations() []string {
	return []string{schemaV1, schemaV2, schemaV3, schemaV4, schemaV5, foldLIDs, schemaV7, schemaV8, schemaV9, schemaV10}
}

func (s *Store) migrate(ctx context.Context) error {
	steps := migrations()
	return s.inTx(ctx, func(tx *sql.Tx) error {
		var version int
		if err := tx.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
			return fmt.Errorf("store: read version: %w", err)
		}
		if version > len(steps) {
			return fmt.Errorf("%w: schema %d, this build knows %d", ErrNewer, version, len(steps))
		}
		for i := version; i < len(steps); i++ {
			if _, err := tx.ExecContext(ctx, steps[i]); err != nil {
				return fmt.Errorf("store: schema %d: %w", i+1, err)
			}
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", len(steps))); err != nil {
			return fmt.Errorf("store: set version: %w", err)
		}
		return nil
	})
}
