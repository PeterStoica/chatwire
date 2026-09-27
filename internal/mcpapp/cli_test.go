package mcpapp_test

import (
	"os/exec"
	"strings"
	"testing"
)

func TestTheCommandLineNamesCommandsNotTools(t *testing.T) {
	w := newWorld(t, shortHome(t))
	cmd := exec.CommandContext(t.Context(), builds[0], "status", "-state", w.state, "-linger", "1s")
	cmd.Env = append(cmd.Environ(), "HOME="+w.home, "XDG_CACHE_HOME=", "XDG_CONFIG_HOME=")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("chatwire status: %v (%s)", err, out)
	}
	if text := string(out); strings.Contains(text, "link_whatsapp") || !strings.Contains(text, "Run chatwire link") {
		t.Fatalf("chatwire status told a person to call a tool: %q", text)
	}
}
