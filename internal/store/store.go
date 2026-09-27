package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"time"

	"modernc.org/sqlite"

	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/wire"
)

const (
	timeShift    = 20
	sqliteBusy   = 5
	openRetry    = 10 * time.Millisecond
	maxOpenRetry = 2 * time.Second
)

type Store struct {
	db *sql.DB
}

type Message struct {
	ID        string
	Chat      node.JID
	Author    node.JID
	FromMe    bool
	Time      time.Time
	PushName  string
	Message   *wire.Message
	Edited    time.Time
	Revoked   bool
	Reactions []Reaction
	Votes     []Vote
	Status    Status
	Unread    bool
}

type Status int

const (
	StatusSent Status = iota
	StatusDelivered
	StatusRead
	StatusPlayed
)

type Tick struct {
	Chat   node.JID
	IDs    []string
	Status Status
}

type Reaction struct {
	By    node.JID
	Emoji string
	Time  time.Time
}

type Vote struct {
	Chat    node.JID
	ID      string
	By      node.JID
	Options []string
	Time    time.Time
}

type Edit struct {
	Chat    node.JID
	ID      string
	Message *wire.Message
	Time    time.Time
}

type Revoke struct {
	Chat node.JID
	ID   string
}

type React struct {
	Chat  node.JID
	ID    string
	By    node.JID
	Emoji string
	Time  time.Time
}

type Chat struct {
	JID         node.JID
	Name        string
	LastMessage time.Time
	Unread      int
	Pinned      time.Time
	Archived    bool
	Mute        Mute
}

type Mute int64

const MuteForever Mute = -1

func (m Mute) Active(now time.Time) bool {
	return m == MuteForever || int64(m) > now.Unix()
}

type Name struct {
	Contact string
	First   string
	Push    string
}

type Query struct {
	Chat     node.JID
	Text     string
	Before   time.Time
	BeforeID string
	Limit    int
	Incoming bool
	From     []node.JID
	Mine     bool
}

func Open(ctx context.Context, path string) (*Store, error) {
	if err := private(path); err != nil {
		return nil, err
	}
	dsn := "file:" + (&url.URL{Path: path}).EscapedPath() + "?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_txlock=immediate"
	connector, err := sqlite.NewConnector(dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	wait := openRetry
	for {
		err := s.migrate(ctx)
		if err == nil {
			return s, nil
		}
		if !retry(err, wait) {
			_ = db.Close()
			return nil, err
		}
		select {
		case <-ctx.Done():
			_ = db.Close()
			return nil, fmt.Errorf("store: open: %w", ctx.Err())
		case <-time.After(wait):
		}
		wait *= 2
	}
}

func retry(err error, wait time.Duration) bool {
	return busy(err) && wait <= maxOpenRetry
}

func busy(err error) bool {
	var e *sqlite.Error
	return errors.As(err, &e) && e.Code()&0xff == sqliteBusy
}

func private(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("store: open: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("store: open: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("store: keep private: %w", err)
	}
	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) inTx(ctx context.Context, work func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin: %w", err)
	}
	if err := work(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit: %w", err)
	}
	return nil
}
