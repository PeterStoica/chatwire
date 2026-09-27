package setup

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

const (
	Name         = "chatwire"
	backupSuffix = ".before-chatwire"
	fileMode     = 0o600
	dirMode      = 0o700
)

var ErrComments = errors.New("setup: the file has comments, so it is left for you to edit")

type Env struct {
	Home     string
	AppData  string
	OS       string
	LookPath func(string) (string, error)
	Run      func(ctx context.Context, name string, args ...string) error
}

func System() (Env, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Env{}, fmt.Errorf("setup: home folder: %w", err)
	}
	return Env{
		Home: home, AppData: os.Getenv("APPDATA"), OS: runtime.GOOS,
		LookPath: exec.LookPath,
		Run: func(ctx context.Context, name string, args ...string) error {
			out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
			if err != nil {
				return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args[:min(2, len(args))], " "), err, bytes.TrimSpace(out))
			}
			return nil
		},
	}, nil
}

type format int

const (
	jsonFile format = iota
	tomlFile
	claudeCLI
	manual
)

type Client struct {
	ID     string
	Name   string
	Path   string
	format format
	key    string
	entry  func(command string) any
	found  bool
	note   string
}

func (c Client) Found() bool {
	return c.found
}

func Clients(env Env) []Client {
	join := filepath.Join
	onPath := func(name string) bool {
		_, err := env.LookPath(name)
		return err == nil
	}
	exists := func(path string) bool {
		_, err := os.Stat(path)
		return err == nil
	}
	appDir := func(mac, windows, linux string) string {
		switch env.OS {
		case "darwin":
			return join(env.Home, "Library", "Application Support", mac)
		case "windows":
			return join(env.AppData, windows)
		default:
			return join(env.Home, ".config", linux)
		}
	}
	command := func(command string) any { return stdio{Command: command, Args: []string{}} }
	claudeDesktop := appDir("Claude", "Claude", "Claude")
	vscode := appDir(join("Code", "User"), join("Code", "User"), join("Code", "User"))
	cline := join(vscode, "globalStorage", "saoudrizwan.claude-dev", "settings")
	zed := join(env.Home, ".config", "zed")
	return []Client{
		{ID: "claude-code", Name: "Claude Code", format: claudeCLI, found: onPath("claude")},
		{ID: "claude-desktop", Name: "Claude Desktop", Path: join(claudeDesktop, "claude_desktop_config.json"), format: jsonFile, key: "mcpServers", entry: command, found: exists(claudeDesktop)},
		{ID: "codex", Name: "Codex", Path: join(env.Home, ".codex", "config.toml"), format: tomlFile, found: exists(join(env.Home, ".codex")) || onPath("codex")},
		{ID: "grok", Name: "Grok Build", Path: join(env.Home, ".grok", "config.toml"), format: tomlFile, found: exists(join(env.Home, ".grok")) || onPath("grok")},
		{ID: "gemini", Name: "Gemini CLI", Path: join(env.Home, ".gemini", "settings.json"), format: jsonFile, key: "mcpServers", entry: command, found: exists(join(env.Home, ".gemini")) || onPath("gemini")},
		{ID: "cursor", Name: "Cursor", Path: join(env.Home, ".cursor", "mcp.json"), format: jsonFile, key: "mcpServers", entry: command, found: exists(join(env.Home, ".cursor"))},
		{ID: "vscode", Name: "VS Code", Path: join(vscode, "mcp.json"), format: jsonFile, key: "servers", found: exists(vscode),
			entry: func(command string) any { return typedStdio{Type: "stdio", Command: command, Args: []string{}} }},
		{ID: "cline", Name: "Cline", Path: join(cline, "cline_mcp_settings.json"), format: jsonFile, key: "mcpServers", found: exists(cline),
			entry: func(command string) any { return switchable{Command: command, Args: []string{}} }},
		{ID: "windsurf", Name: "Windsurf", Path: join(env.Home, ".codeium", "windsurf", "mcp_config.json"), format: jsonFile, key: "mcpServers", entry: command, found: exists(join(env.Home, ".codeium", "windsurf"))},
		{ID: "opencode", Name: "opencode", Path: join(env.Home, ".config", "opencode", "opencode.json"), format: jsonFile, key: "mcp", found: exists(join(env.Home, ".config", "opencode")) || onPath("opencode"),
			entry: func(command string) any { return local{Type: "local", Command: []string{command}, Enabled: true} }},
		{ID: "lmstudio", Name: "LM Studio", Path: join(env.Home, ".lmstudio", "mcp.json"), format: jsonFile, key: "mcpServers", entry: command, found: exists(join(env.Home, ".lmstudio"))},
		{ID: "zed", Name: "Zed", Path: join(zed, "settings.json"), format: manual, found: exists(zed),
			note: `add under "context_servers": "chatwire": {"command": "<path to chatwire>", "args": []}`},
	}
}

type stdio struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

type typedStdio struct {
	Type    string   `json:"type"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

type switchable struct {
	Command  string   `json:"command"`
	Args     []string `json:"args"`
	Disabled bool     `json:"disabled"`
}

type local struct {
	Type    string   `json:"type"`
	Command []string `json:"command"`
	Enabled bool     `json:"enabled"`
}

type Outcome string

const (
	Added     Outcome = "added"
	Updated   Outcome = "updated"
	Unchanged Outcome = "already set"
	Removed   Outcome = "removed"
	Absent    Outcome = "not there"
	Skipped   Outcome = "skipped"
	Failed    Outcome = "failed"
)

type Result struct {
	Client  string  `json:"client"`
	Name    string  `json:"name"`
	Outcome Outcome `json:"outcome"`
	Path    string  `json:"path,omitempty"`
	Detail  string  `json:"detail,omitempty"`
}

func Apply(ctx context.Context, env Env, clients []Client, command string, remove bool) []Result {
	out := make([]Result, 0, len(clients))
	for _, c := range clients {
		r := Result{Client: c.ID, Name: c.Name, Path: c.Path}
		var err error
		switch c.format {
		case jsonFile:
			r.Outcome, err = editFile(c.Path, func(raw []byte) ([]byte, Outcome, error) { return editJSON(raw, c.key, entryOf(c, command, remove)) })
		case tomlFile:
			r.Outcome, err = editFile(c.Path, func(raw []byte) ([]byte, Outcome, error) { return editTOML(raw, command, remove) })
		case claudeCLI:
			r.Path = ""
			r.Outcome, err = viaClaude(ctx, env, command, remove)
		case manual:
			r.Outcome, r.Detail = Skipped, c.note
		}
		if errors.Is(err, ErrComments) {
			r.Outcome, r.Detail = Skipped, err.Error()
		} else if err != nil {
			r.Outcome, r.Detail = Failed, err.Error()
		}
		out = append(out, r)
	}
	return out
}

func entryOf(c Client, command string, remove bool) any {
	if remove {
		return nil
	}
	return c.entry(command)
}

func viaClaude(ctx context.Context, env Env, command string, remove bool) (Outcome, error) {
	current, extra, settings := claudeEntry(env.Home)
	if !remove && current == command {
		return Unchanged, nil
	}
	removeErr := env.Run(ctx, "claude", "mcp", "remove", "-s", "user", Name)
	if remove {
		if removeErr != nil {
			return Absent, nil
		}
		return Removed, nil
	}
	args := []string{"mcp", "add", "-s", "user"}
	for _, setting := range settings {
		args = append(args, "-e", setting)
	}
	if err := env.Run(ctx, "claude", append(append(args, Name, "--", command), extra...)...); err != nil {
		return Failed, err
	}
	if removeErr == nil || current != "" {
		return Updated, nil
	}
	return Added, nil
}

func claudeEntry(home string) (string, []string, []string) {
	raw, err := os.ReadFile(filepath.Join(home, ".claude.json"))
	if err != nil {
		return "", nil, nil
	}
	var config struct {
		Servers map[string]struct {
			Command string            `json:"command"`
			Args    []string          `json:"args"`
			Env     map[string]string `json:"env"`
		} `json:"mcpServers"`
	}
	if json.Unmarshal(raw, &config) != nil {
		return "", nil, nil
	}
	entry := config.Servers[Name]
	settings := make([]string, 0, len(entry.Env))
	for key, value := range entry.Env {
		settings = append(settings, key+"="+value)
	}
	slices.Sort(settings)
	return entry.Command, entry.Args, settings
}

func editFile(path string, edit func([]byte) ([]byte, Outcome, error)) (Outcome, error) {
	raw, err := os.ReadFile(path)
	missing := errors.Is(err, os.ErrNotExist)
	if err != nil && !missing {
		return Failed, err
	}
	next, outcome, err := edit(raw)
	if err != nil || outcome == Unchanged || outcome == Absent {
		return outcome, err
	}
	if !missing {
		if err := backup(path, raw); err != nil {
			return Failed, err
		}
	}
	return outcome, writeAtomic(path, next)
}

func backup(path string, raw []byte) error {
	target := path + backupSuffix
	if _, err := os.Stat(target); err == nil {
		return nil
	}
	if err := os.WriteFile(target, raw, fileMode); err != nil {
		return fmt.Errorf("setup: back up %s: %w", path, err)
	}
	return nil
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), dirMode); err != nil {
		return fmt.Errorf("setup: %w", err)
	}
	mode := os.FileMode(fileMode)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("setup: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("setup: write %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("setup: write %s: %w", path, err)
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return fmt.Errorf("setup: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("setup: replace %s: %w", path, err)
	}
	return nil
}

type member struct {
	name  string
	value jsontext.Value
}

func hasComments(raw []byte) bool {
	inString, escaped := false, false
	for i, c := range raw {
		switch {
		case inString && escaped:
			escaped = false
		case inString && c == '\\':
			escaped = true
		case inString && c == '"':
			inString = false
		case inString:
		case c == '"':
			inString = true
		case c == '/' && i+1 < len(raw) && (raw[i+1] == '/' || raw[i+1] == '*'):
			return true
		}
	}
	return false
}

func editJSON(raw []byte, key string, entry any) ([]byte, Outcome, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		if entry == nil {
			return raw, Absent, nil
		}
		raw = []byte("{}")
	}
	if hasComments(raw) {
		return nil, Failed, ErrComments
	}
	top, err := members(raw)
	if err != nil {
		return nil, Failed, err
	}
	at := index(top, key)
	var servers []member
	if at >= 0 {
		if servers, err = members(top[at].value); err != nil {
			return nil, Failed, fmt.Errorf("setup: %q is not an object: %w", key, err)
		}
	}
	outcome, servers, err := place(servers, entry)
	if err != nil || outcome == Unchanged || outcome == Absent {
		return raw, outcome, err
	}
	object, err := encode(servers, "")
	if err != nil {
		return nil, Failed, err
	}
	switch {
	case at < 0:
		top = append(top, member{name: key, value: object})
	case len(servers) == 0:
		top = append(top[:at], top[at+1:]...)
	default:
		top[at].value = object
	}
	out, err := encode(top, indentOf(raw))
	if err != nil {
		return nil, Failed, err
	}
	return append(out, '\n'), outcome, nil
}

func place(servers []member, entry any) (Outcome, []member, error) {
	at := index(servers, Name)
	if entry == nil {
		if at < 0 {
			return Absent, servers, nil
		}
		return Removed, append(servers[:at], servers[at+1:]...), nil
	}
	value, err := json.Marshal(entry)
	if err != nil {
		return Failed, nil, err
	}
	if at < 0 {
		return Added, append(servers, member{name: Name, value: value}), nil
	}
	merged, err := keepOthers(servers[at].value, value)
	if err != nil {
		return Failed, nil, err
	}
	if same(servers[at].value, merged) {
		return Unchanged, servers, nil
	}
	servers[at].value = merged
	return Updated, servers, nil
}

func keepOthers(current, fresh jsontext.Value) (jsontext.Value, error) {
	kept, err := members(current)
	if err != nil {
		return fresh, nil
	}
	updates, err := members(fresh)
	if err != nil {
		return nil, err
	}
	for _, u := range updates {
		i := index(kept, u.name)
		switch {
		case i < 0:
			kept = append(kept, u)
		case u.name == "command":
			kept[i].value = newCommand(kept[i].value, u.value)
		}
	}
	return encode(kept, "")
}

func newCommand(current, fresh jsontext.Value) jsontext.Value {
	var have, want []jsontext.Value
	if json.Unmarshal(current, &have) != nil || json.Unmarshal(fresh, &want) != nil || len(have) == 0 || len(want) == 0 {
		return fresh
	}
	have[0] = want[0]
	joined, err := json.Marshal(have)
	if err != nil {
		return fresh
	}
	return joined
}

func same(a, b jsontext.Value) bool {
	x, y := a.Clone(), b.Clone()
	if x.Canonicalize() != nil || y.Canonicalize() != nil {
		return false
	}
	return bytes.Equal(x, y)
}

func index(ms []member, name string) int {
	for i, m := range ms {
		if m.name == name {
			return i
		}
	}
	return -1
}

func members(raw []byte) ([]member, error) {
	dec := jsontext.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.ReadToken()
	if err != nil {
		return nil, fmt.Errorf("setup: not JSON: %w", err)
	}
	if tok.Kind() != '{' {
		return nil, errors.New("setup: not a JSON object")
	}
	var out []member
	for dec.PeekKind() != '}' {
		token, err := dec.ReadToken()
		if err != nil {
			return nil, fmt.Errorf("setup: not JSON: %w", err)
		}
		name := token.String()
		value, err := dec.ReadValue()
		if err != nil {
			return nil, fmt.Errorf("setup: not JSON: %w", err)
		}
		out = append(out, member{name: name, value: value.Clone()})
	}
	if _, err := dec.ReadToken(); err != nil {
		return nil, fmt.Errorf("setup: not JSON: %w", err)
	}
	return out, nil
}

func encode(ms []member, indent string) (jsontext.Value, error) {
	var buf bytes.Buffer
	opts := []jsontext.Options{}
	if indent != "" {
		opts = append(opts, jsontext.WithIndent(indent))
	}
	enc := jsontext.NewEncoder(&buf, opts...)
	if err := enc.WriteToken(jsontext.BeginObject); err != nil {
		return nil, err
	}
	for _, m := range ms {
		if err := enc.WriteToken(jsontext.String(m.name)); err != nil {
			return nil, err
		}
		if err := enc.WriteValue(m.value); err != nil {
			return nil, err
		}
	}
	if err := enc.WriteToken(jsontext.EndObject); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func indentOf(raw []byte) string {
	for _, line := range strings.Split(string(raw), "\n") {
		if trimmed := strings.TrimLeft(line, " \t"); trimmed != line && trimmed != "" {
			return line[:len(line)-len(trimmed)]
		}
	}
	return "  "
}

func editTOML(raw []byte, command string, remove bool) ([]byte, Outcome, error) {
	lines := strings.Split(string(raw), "\n")
	header := "[mcp_servers." + Name + "]"
	start, end := -1, len(lines)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case start < 0 && trimmed == header:
			start = i
		case start >= 0 && strings.HasPrefix(trimmed, "[") && !strings.HasPrefix(trimmed, "[mcp_servers."+Name+"."):
			end = i
		}
		if start >= 0 && end < len(lines) {
			break
		}
	}
	section := []string{header, "command = " + tomlString(command), "args = []"}
	switch {
	case remove && start < 0:
		return raw, Absent, nil
	case remove:
		kept := append(append([]string{}, lines[:start]...), lines[end:]...)
		return []byte(strings.Join(kept, "\n")), Removed, nil
	case start < 0:
		text := strings.TrimRight(string(raw), "\n")
		if text != "" {
			text += "\n\n"
		}
		return []byte(text + strings.Join(section, "\n") + "\n"), Added, nil
	}
	var rest []string
	inTable, hasCommand, hasArgs := false, false, false
	for _, line := range lines[start+1 : end] {
		trimmed := strings.TrimSpace(line)
		inTable = inTable || strings.HasPrefix(trimmed, "[")
		key, _, found := strings.Cut(trimmed, "=")
		key = strings.Trim(strings.TrimSpace(key), `"'`)
		switch {
		case inTable || !found:
		case key == "command":
			line, hasCommand = "command = "+tomlString(command), true
		case key == "args":
			hasArgs = true
		}
		rest = append(rest, line)
	}
	for len(rest) > 0 && strings.TrimSpace(rest[len(rest)-1]) == "" {
		rest = rest[:len(rest)-1]
	}
	section = section[:1]
	if !hasCommand {
		section = append(section, "command = "+tomlString(command))
	}
	if !hasArgs {
		section = append(section, "args = []")
	}
	section = append(section, rest...)
	current := strings.TrimSpace(strings.Join(lines[start:end], "\n"))
	if current == strings.Join(section, "\n") {
		return raw, Unchanged, nil
	}
	tail := lines[end:]
	if end < len(lines) {
		section = append(section, "")
	}
	updated := strings.Join(append(append(append([]string{}, lines[:start]...), section...), tail...), "\n")
	if strings.HasSuffix(string(raw), "\n") && !strings.HasSuffix(updated, "\n") {
		updated += "\n"
	}
	return []byte(updated), Updated, nil
}

func tomlString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}
