package mcpapp

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/PeterStoica/chatwire/internal/update"
)

const (
	checkEvery    = 24 * time.Hour
	checkTimeout  = 20 * time.Second
	updateTimeout = 5 * time.Minute
	noCheck       = "CHATWIRE_NO_UPDATE_CHECK"
	updateFile    = "update.json"
)

type updateCheck struct {
	Checked time.Time `json:"checked"`
	Latest  string    `json:"latest"`
}

type updates struct {
	path   string
	client *http.Client
	mu     sync.Mutex
	last   updateCheck
}

func newUpdates(dir string) *updates {
	u := &updates{path: filepath.Join(dir, updateFile), client: &http.Client{Timeout: checkTimeout}}
	if raw, err := os.ReadFile(u.path); err == nil {
		_ = json.Unmarshal(raw, &u.last)
	}
	return u
}

func (u *updates) watch(ctx context.Context) {
	if os.Getenv(noCheck) != "" || !update.IsRelease(version()) {
		return
	}
	for {
		u.mu.Lock()
		due := checkEvery - time.Since(u.last.Checked)
		u.mu.Unlock()
		if due <= 0 {
			u.check(ctx)
			due = checkEvery
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(due):
		}
	}
}

func (u *updates) check(ctx context.Context) {
	latest, err := update.Latest(ctx, u.client, update.Home)
	u.mu.Lock()
	defer u.mu.Unlock()
	u.last.Checked = time.Now()
	if err == nil {
		u.last.Latest = latest
	}
	if raw, err := json.Marshal(u.last); err == nil {
		_ = os.WriteFile(u.path, raw, 0o600)
	}
}

func (u *updates) available() (string, bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.last.Latest, update.Newer(version(), u.last.Latest)
}

func runUpdate(ctx context.Context, out io.Writer, onlyCheck, asJSON bool) error {
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()
	client := &http.Client{Timeout: updateTimeout}
	current := version()
	latest, err := update.Latest(ctx, client, update.Home)
	if err != nil {
		return err
	}
	report := func(state, text string) error {
		if asJSON {
			raw, err := json.Marshal(map[string]string{"state": state, "current": current, "latest": latest})
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(out, string(raw))
			return err
		}
		_, err := fmt.Fprintln(out, text)
		return err
	}
	if !update.Newer(current, latest) {
		return report("up_to_date", fmt.Sprintf("Chatwire %s is the latest (%s published).", current, latest))
	}
	if onlyCheck {
		return report("available", fmt.Sprintf("Chatwire %s is available (this is %s). Run: chatwire update", latest, current))
	}
	target, _, err := self()
	if err != nil {
		return err
	}
	binary, err := update.Download(ctx, client, update.Home, latest)
	if err != nil {
		return err
	}
	if err := update.Install(target, binary); err != nil {
		return err
	}
	return report("updated", fmt.Sprintf("Updated Chatwire from %s to %s. AI apps pick it up in their next session.", current, latest))
}
