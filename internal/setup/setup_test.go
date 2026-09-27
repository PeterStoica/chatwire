package setup_test

import (
	"context"
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/PeterStoica/chatwire/internal/setup"
)

const codexBefore = `model = "gpt-5"

[mcp_servers.figma]
command = "npx"
args = ["-y", "figma"]

[mcp_servers.figma.env]
TOKEN = "x"
`

const desktopBefore = "{\n\t\"coworkUserFilesPath\": \"/tmp/files\",\n\t\"preferences\": {\n\t\t\"theme\": \"dark\"\n\t}\n}\n"

type world struct {
	env  setup.Env
	home string
	runs [][]string
}

func newWorld(t *testing.T) *world {
	t.Helper()
	home := t.TempDir()
	w := &world{home: home}
	w.env = setup.Env{
		Home: home, OS: "darwin",
		LookPath: func(name string) (string, error) {
			if name == "claude" || name == "gemini" {
				return "/usr/local/bin/" + name, nil
			}
			return "", errors.New("not found")
		},
		Run: func(_ context.Context, name string, args ...string) error {
			w.runs = append(w.runs, append([]string{name}, args...))
			if slices.Contains(args, "remove") && len(w.runs) == 1 {
				return errors.New("no server named chatwire")
			}
			return nil
		},
	}
	w.write(t, "Library/Application Support/Claude/claude_desktop_config.json", desktopBefore)
	w.write(t, ".codex/config.toml", codexBefore)
	w.write(t, "Library/Application Support/Code/User/settings.json", "{\n  // editor settings\n  \"files.autoSave\": \"afterDelay\"\n}\n")
	w.write(t, "Library/Application Support/Code/User/globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json", "{\n  /* keep */\n  \"mcpServers\": {}\n}\n")
	w.write(t, ".config/zed/settings.json", "{}\n")
	if err := os.MkdirAll(filepath.Join(home, ".cursor"), 0o700); err != nil {
		t.Fatal(err)
	}
	return w
}

func (w *world) write(t *testing.T, rel, content string) {
	t.Helper()
	path := filepath.Join(w.home, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (w *world) read(t *testing.T, rel string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(w.home, rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func (w *world) found() []setup.Client {
	var out []setup.Client
	for _, c := range setup.Clients(w.env) {
		if c.Found() {
			out = append(out, c)
		}
	}
	return out
}

func outcomes(results []setup.Result) map[string]setup.Outcome {
	out := map[string]setup.Outcome{}
	for _, r := range results {
		out[r.Client] = r.Outcome
	}
	return out
}

func TestSetupFindsTheAppsOnThisComputer(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	var ids []string
	for _, c := range w.found() {
		ids = append(ids, c.ID)
	}
	want := []string{"claude-code", "claude-desktop", "codex", "gemini", "cursor", "vscode", "cline", "zed"}
	if !slices.Equal(ids, want) {
		t.Fatalf("found %v, want %v", ids, want)
	}
}

func TestSetupAddsChatwireWithoutDisturbingAnythingElse(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	ctx := t.Context()
	got := outcomes(setup.Apply(ctx, w.env, w.found(), "/opt/chatwire", false))
	want := map[string]setup.Outcome{
		"claude-code": setup.Added, "claude-desktop": setup.Added, "codex": setup.Added, "gemini": setup.Added,
		"cursor": setup.Added, "vscode": setup.Added, "cline": setup.Skipped, "zed": setup.Skipped,
	}
	for id, outcome := range want {
		if got[id] != outcome {
			t.Errorf("%s: %s, want %s", id, got[id], outcome)
		}
	}

	desktop := w.read(t, "Library/Application Support/Claude/claude_desktop_config.json")
	wantDesktop := "{\n\t\"coworkUserFilesPath\": \"/tmp/files\",\n\t\"preferences\": {\n\t\t\"theme\": \"dark\"\n\t},\n\t\"mcpServers\": {\n\t\t\"chatwire\": {\n\t\t\t\"command\": \"/opt/chatwire\",\n\t\t\t\"args\": []\n\t\t}\n\t}\n}\n"
	if desktop != wantDesktop {
		t.Fatalf("Claude Desktop config:\n%s\nwant\n%s", desktop, wantDesktop)
	}
	if backup := w.read(t, "Library/Application Support/Claude/claude_desktop_config.json.before-chatwire"); backup != desktopBefore {
		t.Fatalf("backup = %q", backup)
	}
	var cursor map[string]map[string]map[string]any
	if err := json.Unmarshal([]byte(w.read(t, ".cursor/mcp.json")), &cursor); err != nil || cursor["mcpServers"]["chatwire"]["command"] != "/opt/chatwire" {
		t.Fatalf("cursor = %v, %v", cursor, err)
	}
	var vscode map[string]map[string]map[string]any
	if err := json.Unmarshal([]byte(w.read(t, "Library/Application Support/Code/User/mcp.json")), &vscode); err != nil || vscode["servers"]["chatwire"]["type"] != "stdio" {
		t.Fatalf("vscode = %v, %v", vscode, err)
	}
	if settings := w.read(t, "Library/Application Support/Code/User/settings.json"); !strings.Contains(settings, "// editor settings") {
		t.Fatal("the VS Code settings file was touched")
	}
	codex := w.read(t, ".codex/config.toml")
	if !strings.HasPrefix(codex, codexBefore) || !strings.HasSuffix(codex, "\n[mcp_servers.chatwire]\ncommand = \"/opt/chatwire\"\nargs = []\n") {
		t.Fatalf("codex config:\n%s", codex)
	}
	if len(w.runs) != 2 || strings.Join(w.runs[1], " ") != "claude mcp add -s user chatwire -- /opt/chatwire" {
		t.Fatalf("claude runs = %v", w.runs)
	}

	again := outcomes(setup.Apply(ctx, w.env, w.found(), "/opt/chatwire", false))
	for _, id := range []string{"claude-desktop", "codex", "gemini", "cursor", "vscode"} {
		if again[id] != setup.Unchanged {
			t.Errorf("second run %s: %s, want %s", id, again[id], setup.Unchanged)
		}
	}
	moved := outcomes(setup.Apply(ctx, w.env, w.found(), `C:\Program Files\chatwire.exe`, false))
	if moved["codex"] != setup.Updated || moved["claude-desktop"] != setup.Updated {
		t.Fatalf("a new path: %v", moved)
	}
	if codex := w.read(t, ".codex/config.toml"); !strings.Contains(codex, `command = "C:\\Program Files\\chatwire.exe"`) || strings.Count(codex, "[mcp_servers.chatwire]") != 1 {
		t.Fatalf("codex after a move:\n%s", codex)
	}
	if backup := w.read(t, "Library/Application Support/Claude/claude_desktop_config.json.before-chatwire"); backup != desktopBefore {
		t.Fatal("the backup of the original was overwritten")
	}
}

func TestSetupAgainKeepsEachAppsOwnSettings(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	ctx := t.Context()
	w.write(t, "Library/Application Support/Claude/claude_desktop_config.json",
		"{\n\t\"mcpServers\": {\n\t\t\"chatwire\": {\n\t\t\t\"command\": \"/opt/chatwire\",\n\t\t\t\"args\": [],\n\t\t\t\"env\": {\n\t\t\t\t\"CHATWIRE_READ_ONLY\": \"1\"\n\t\t\t}\n\t\t}\n\t}\n}\n")
	w.write(t, ".codex/config.toml", "[mcp_servers.chatwire]\ncommand = \"/opt/chatwire\"\nargs = []\nstartup_timeout_sec = 20\n\n[mcp_servers.chatwire.env]\nCHATWIRE_READ_ONLY = \"1\"\n\n[mcp_servers.figma]\ncommand = \"npx\"\n")
	w.write(t, ".claude.json", `{"numStartups": 3, "mcpServers": {"chatwire": {"type": "stdio", "command": "/opt/chatwire", "args": [], "env": {"CHATWIRE_READ_ONLY": "1", "CHATWIRE_FILES": "/work"}}}}`)
	clients := func(ids ...string) []setup.Client {
		var out []setup.Client
		for _, c := range w.found() {
			if slices.Contains(ids, c.ID) {
				out = append(out, c)
			}
		}
		return out
	}
	chosen := clients("claude-code", "claude-desktop", "codex")

	same := outcomes(setup.Apply(ctx, w.env, chosen, "/opt/chatwire", false))
	for _, id := range []string{"claude-code", "claude-desktop", "codex"} {
		if same[id] != setup.Unchanged {
			t.Errorf("setup again with the same path: %s %s, want %s", id, same[id], setup.Unchanged)
		}
	}
	if len(w.runs) != 0 {
		t.Fatalf("Claude Code was registered again although nothing changed: %v", w.runs)
	}

	moved := outcomes(setup.Apply(ctx, w.env, chosen, "/new/chatwire", false))
	if moved["claude-desktop"] != setup.Updated || moved["codex"] != setup.Updated || moved["claude-code"] != setup.Updated {
		t.Fatalf("setup with a new path: %v", moved)
	}
	var desktop map[string]map[string]map[string]any
	if err := json.Unmarshal([]byte(w.read(t, "Library/Application Support/Claude/claude_desktop_config.json")), &desktop); err != nil {
		t.Fatal(err)
	}
	entry := desktop["mcpServers"]["chatwire"]
	if entry["command"] != "/new/chatwire" || entry["env"].(map[string]any)["CHATWIRE_READ_ONLY"] != "1" {
		t.Fatalf("Claude Desktop entry after a move: %v", entry)
	}
	codex := w.read(t, ".codex/config.toml")
	for _, want := range []string{`command = "/new/chatwire"`, "startup_timeout_sec = 20", "[mcp_servers.chatwire.env]\nCHATWIRE_READ_ONLY = \"1\"", "[mcp_servers.figma]"} {
		if !strings.Contains(codex, want) {
			t.Errorf("codex config lost %q:\n%s", want, codex)
		}
	}
	if strings.Count(codex, "command = ") != 2 || strings.Count(codex, "[mcp_servers.chatwire]") != 1 {
		t.Errorf("codex config:\n%s", codex)
	}
	if last := strings.Join(w.runs[len(w.runs)-1], " "); last != "claude mcp add -s user -e CHATWIRE_FILES=/work -e CHATWIRE_READ_ONLY=1 chatwire -- /new/chatwire" {
		t.Fatalf("Claude Code was registered as: %s", last)
	}
}

func TestSetupRemoveRestoresTheFiles(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	ctx := t.Context()
	setup.Apply(ctx, w.env, w.found(), "/opt/chatwire", false)
	got := outcomes(setup.Apply(ctx, w.env, w.found(), "/opt/chatwire", true))
	for _, id := range []string{"claude-desktop", "codex", "gemini", "cursor", "vscode", "claude-code"} {
		if got[id] != setup.Removed {
			t.Errorf("%s: %s, want %s", id, got[id], setup.Removed)
		}
	}
	if desktop := w.read(t, "Library/Application Support/Claude/claude_desktop_config.json"); desktop != desktopBefore {
		t.Fatalf("Claude Desktop after removal:\n%q\nwant\n%q", desktop, desktopBefore)
	}
	if codex := w.read(t, ".codex/config.toml"); strings.TrimRight(codex, "\n") != strings.TrimRight(codexBefore, "\n") {
		t.Fatalf("codex after removal:\n%s", codex)
	}
	if again := outcomes(setup.Apply(ctx, w.env, w.found(), "/opt/chatwire", true)); again["codex"] != setup.Absent || again["claude-desktop"] != setup.Absent {
		t.Fatalf("removing twice: %v", again)
	}
}
