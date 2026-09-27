package store

import (
	"context"
	"database/sql"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"modernc.org/sqlite"
)

func TestMigratingAVersionOneStore(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "old.db")
	connector, err := sqlite.NewConnector("file:" + (&url.URL{Path: path}).EscapedPath())
	if err != nil {
		t.Fatal(err)
	}
	old := sql.OpenDB(connector)
	if _, err := old.ExecContext(ctx, schemaV1+`PRAGMA user_version = 1;
		INSERT INTO messages (key, chat, id, author, from_me, t, push_name, text, raw) VALUES (1, '40722222222@s.whatsapp.net', '3EB0OLD', '40722222222@s.whatsapp.net', 0, 1790000000, 'Bob', 'from before', x'0a0b66726f6d206265666f7265');`); err != nil {
		t.Fatal(err)
	}
	if err := old.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var version int
	if err := s.db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil || version != len(migrations()) {
		t.Fatalf("version %d after migrating, %v", version, err)
	}
	kept, err := s.Messages(ctx, Query{Text: "before", Limit: 5})
	if err != nil || len(kept) != 1 || kept[0].Message.GetConversation() != "from before" || kept[0].Revoked || !kept[0].Edited.IsZero() || len(kept[0].Reactions) != 0 {
		t.Fatalf("a message from version one: %+v, %v", kept, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	again, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("reopening a migrated store: %v", err)
	}
	if err := again.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestMigratingMovesOldAuthorsToTheirNumber(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "v8.db")
	connector, err := sqlite.NewConnector("file:" + (&url.URL{Path: path}).EscapedPath())
	if err != nil {
		t.Fatal(err)
	}
	old := sql.OpenDB(connector)
	steps := migrations()[:8]
	for _, step := range steps {
		if _, err := old.ExecContext(ctx, step); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := old.ExecContext(ctx, `PRAGMA user_version = 8;
		INSERT INTO lids (lid, pn) VALUES ('99001@lid', '40722222222@s.whatsapp.net');
		INSERT INTO messages (key, chat, id, author, from_me, t, push_name, text, raw) VALUES
			(1, '120363000000000031@g.us', 'G1', '99001@lid', 0, 1790000000, 'Bob', 'hi', x''),
			(2, '120363000000000031@g.us', 'G2', '99002@lid', 0, 1790000001, 'Eve', 'hey', x'');`); err != nil {
		t.Fatal(err)
	}
	if err := old.Close(); err != nil {
		t.Fatal(err)
	}
	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	authors := map[string]string{}
	rows, err := s.db.QueryContext(ctx, `SELECT id, author FROM messages`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, author string
		if err := rows.Scan(&id, &author); err != nil {
			t.Fatal(err)
		}
		authors[id] = author
	}
	if authors["G1"] != "40722222222@s.whatsapp.net" || authors["G2"] != "99002@lid" {
		t.Fatalf("authors after migrating: %v", authors)
	}
}
