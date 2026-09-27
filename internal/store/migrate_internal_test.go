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
