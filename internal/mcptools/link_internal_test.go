package mcptools

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/PeterStoica/chatwire/internal/limits"
	"github.com/PeterStoica/chatwire/internal/linkflow"
	"github.com/PeterStoica/chatwire/internal/messenger"
)

func TestStatusSaysWhyItIsNotConnected(t *testing.T) {
	retry := time.Date(2026, 9, 27, 18, 30, 0, 0, time.Local)
	for _, tt := range []struct {
		name      string
		c         messenger.Connection
		wantState string
		wantText  string
	}{
		{name: "connected", c: messenger.Connection{Connected: true}, wantState: "connected", wantText: "ready"},
		{name: "banned", c: messenger.Connection{Err: linkflow.Ban{Code: 101, For: time.Hour}, Retry: retry}, wantState: "banned", wantText: "reason 101"},
		{name: "replaced", c: messenger.Connection{Err: linkflow.ErrReplaced, Retry: retry}, wantState: "replaced", wantText: "close the other program"},
		{name: "outdated", c: messenger.Connection{Err: linkflow.ErrOutdated}, wantState: "refused", wantText: "updating Chatwire"},
		{name: "offline a while", c: messenger.Connection{Err: errors.New("dial tcp: no route to host"), Failures: 12}, wantState: "offline", wantText: "after 12 tries"},
		{name: "a hiccup", c: messenger.Connection{Err: errors.New("EOF"), Failures: 1, Retry: retry}, wantState: "reconnecting", wantText: "Next try: 18:30 on Sep 27"},
		{name: "starting", c: messenger.Connection{}, wantState: "connecting", wantText: "Connecting"},
	} {
		state, detail := connection(tt.c, "Linked.")
		if state != tt.wantState || !strings.Contains(detail, tt.wantText) {
			t.Errorf("%s: %q, %q; want %q containing %q", tt.name, state, detail, tt.wantState, tt.wantText)
		}
	}
}

func TestStatusNamesWhatsAppsLimits(t *testing.T) {
	now := time.Date(2026, 9, 27, 18, 0, 0, 0, time.Local)
	ends := now.Add(2 * time.Hour)
	for _, tt := range []struct {
		name string
		c    messenger.Connection
		want string
	}{
		{name: "timelock", c: messenger.Connection{Timelock: limits.Timelock{Active: true, Ends: ends, Kind: "BULK_MESSAGING"}}, want: "until 20:00 on Sep 27 (BULK_MESSAGING)"},
		{name: "allowance used up", c: messenger.Connection{Allowance: limits.Cap{Status: limits.Capped, Used: 50, Total: 50, Ends: ends}}, want: "allowance of 50 messages"},
		{name: "warning", c: messenger.Connection{Allowance: limits.Cap{Status: limits.FirstWarning, Used: 40, Total: 50, Ends: ends}}, want: "40 of this account's 50"},
		{name: "over", c: messenger.Connection{Timelock: limits.Timelock{Active: true, Ends: now}, Allowance: limits.Cap{Status: limits.Capped, Ends: now.Add(-time.Second)}}},
	} {
		got := limitsText(tt.c, now)
		if tt.want == "" && got != "" || !strings.Contains(got, tt.want) {
			t.Errorf("%s: %q, want %q", tt.name, got, tt.want)
		}
	}
}
