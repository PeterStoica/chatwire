package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"modernc.org/sqlite"
)

func TestOnlyABusyStoreIsRetriedAndOnlyForAWhile(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "busy.db")
	open := func() *sql.DB {
		connector, err := sqlite.NewConnector("file:" + (&url.URL{Path: path}).EscapedPath() + "?_pragma=busy_timeout(0)&_txlock=immediate")
		if err != nil {
			t.Fatal(err)
		}
		db := sql.OpenDB(connector)
		db.SetMaxOpenConns(1)
		t.Cleanup(func() { _ = db.Close() })
		return db
	}
	holder, waiter := open(), open()
	if _, err := holder.ExecContext(ctx, `CREATE TABLE t (x INTEGER)`); err != nil {
		t.Fatal(err)
	}
	held, err := holder.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = held.Rollback() }()
	_, err = waiter.BeginTx(ctx, nil)
	if err == nil {
		t.Fatal("a second writer got in")
	}
	wrapped := fmt.Errorf("store: begin: %w", err)
	for _, tt := range []struct {
		name string
		err  error
		wait time.Duration
		want bool
	}{
		{name: "busy", err: err, wait: openRetry, want: true},
		{name: "busy, wrapped", err: wrapped, wait: openRetry, want: true},
		{name: "busy at the last wait", err: err, wait: maxOpenRetry, want: true},
		{name: "busy after waiting enough", err: err, wait: maxOpenRetry + 1, want: false},
		{name: "another error", err: errors.New("disk full"), wait: openRetry, want: false},
		{name: "no error", err: nil, wait: openRetry, want: false},
	} {
		if got := retry(tt.err, tt.wait); got != tt.want {
			t.Errorf("%s: retry() = %v", tt.name, got)
		}
	}
}
