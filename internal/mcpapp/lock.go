package mcpapp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	lockName  = "chatwire.lock"
	lockWait  = 15 * time.Second
	lockPause = 200 * time.Millisecond
)

var ErrInUse = errors.New("mcpapp: another Chatwire is already using this WhatsApp link")

func acquire(ctx context.Context, dir string, wait time.Duration) (*os.File, error) {
	path := filepath.Join(dir, lockName)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("mcpapp: lock file: %w", err)
	}
	deadline := time.Now().Add(wait)
	for {
		if lockFile(f) == nil {
			if err := f.Truncate(0); err == nil {
				_, _ = f.WriteAt([]byte(strconv.Itoa(os.Getpid())), 0)
			}
			return f, nil
		}
		if time.Now().After(deadline) {
			_ = f.Close()
			holder, _ := os.ReadFile(path)
			if pid := strings.TrimSpace(string(holder)); pid != "" {
				return nil, fmt.Errorf("%w (process %s)", ErrInUse, pid)
			}
			return nil, ErrInUse
		}
		select {
		case <-ctx.Done():
			_ = f.Close()
			return nil, ctx.Err()
		case <-time.After(lockPause):
		}
	}
}
