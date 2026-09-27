package mcptools

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/PeterStoica/chatwire/internal/client"
	"github.com/PeterStoica/chatwire/internal/messenger"
)

func TestWhatsAppRefusalsBecomeNextSteps(t *testing.T) {
	for _, tt := range []struct {
		err       error
		wantState string
		wantText  string
	}{
		{err: client.Rejection{Code: 463}, wantState: "restricted", wantText: "Do not retry"},
		{err: fmt.Errorf("messenger: send: %w", client.Rejection{Code: 403}), wantState: "not_allowed", wantText: "blocked"},
		{err: client.Rejection{Code: 405}, wantState: "unsupported", wantText: "linked device"},
		{err: client.Rejection{Code: 475}, wantState: "new_chat_limit", wantText: "new chats"},
		{err: client.Rejection{Code: 479}, wantState: "try_again", wantText: "Try once more"},
		{err: client.Rejection{Code: 421}, wantState: "try_again", wantText: "error 421"},
		{err: client.Rejection{Code: 400}, wantState: "try_again", wantText: "error 400"},
		{err: client.Rejection{Code: 599}, wantState: "rejected", wantText: "error 599"},
		{err: fmt.Errorf("%w until tomorrow", messenger.ErrRestricted), wantState: "restricted", wantText: "held for 24 hours"},
		{err: messenger.ErrTooFast, wantState: "slow_down", wantText: "a minute"},
	} {
		state, detail, ok := stopped(tt.err)
		if !ok || state != tt.wantState || !strings.Contains(detail, tt.wantText) {
			t.Errorf("stopped(%v) = %q, %q, %v; want %q containing %q", tt.err, state, detail, ok, tt.wantState, tt.wantText)
		}
	}
	for _, err := range []error{nil, errors.New("network down")} {
		if state, _, ok := stopped(err); ok {
			t.Errorf("stopped(%v) = %q, want not stopped", err, state)
		}
	}
}
