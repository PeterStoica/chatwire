package setup

import (
	"errors"
	"testing"
)

func TestOnlyRealCommentsKeepAFileUntouched(t *testing.T) {
	for _, tt := range []struct {
		name     string
		raw      string
		comments bool
	}{
		{name: "a glob in another server's arguments", raw: `{"mcpServers": {"lint": {"command": "eslint", "args": ["src/**/*.ts"]}}}`},
		{name: "a url", raw: `{"mcpServers": {"web": {"url": "https://example.com/mcp"}}}`},
		{name: "an escaped quote before a slash", raw: `{"mcpServers": {"x": {"args": ["say \"hi\" //not a comment"]}}}`},
		{name: "a line comment", raw: "{\n  // mine\n  \"mcpServers\": {}\n}", comments: true},
		{name: "a block comment", raw: `{ /* mine */ "mcpServers": {} }`, comments: true},
		{name: "a comment after a string", raw: "{\"mcpServers\": {} // trailing\n}", comments: true},
	} {
		_, _, err := editJSON([]byte(tt.raw), "mcpServers", map[string]any{"command": "chatwire"})
		if got := errors.Is(err, ErrComments); got != tt.comments {
			t.Errorf("%s: comments = %v (%v), want %v", tt.name, got, err, tt.comments)
		}
	}
}
