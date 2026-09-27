package mcpapp_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var builds [2]string

func TestMain(m *testing.M) {
	if runtime.GOOS == "windows" {
		os.Exit(m.Run())
	}
	dir, err := os.MkdirTemp("/tmp", "chatwire-bin")
	if err != nil {
		panic(err)
	}
	for i, flags := range []string{"", "-s -w"} {
		builds[i] = filepath.Join(dir, "build"+string(rune('a'+i)), "chatwire")
		build := exec.CommandContext(context.Background(), "go", "build", "-ldflags", flags, "-o", builds[i], "../../cmd/chatwire")
		build.Stderr = os.Stderr
		if err := build.Run(); err != nil {
			panic(err)
		}
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

type world struct {
	home  string
	state string
	env   []string
}

func newWorld(t *testing.T, home string) world {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the process tests need unix sockets and pgrep")
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	w := world{home: home, state: filepath.Join(home, "state", "linked.json")}
	t.Cleanup(func() {
		for _, pid := range w.daemons(t) {
			_ = exec.CommandContext(context.Background(), "kill", "-9", pid).Run()
		}
		_ = os.RemoveAll(home)
	})
	return w
}

func shortHome(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "chatwire-home")
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func (w world) daemons(t *testing.T) []string {
	t.Helper()
	out, _ := exec.CommandContext(context.Background(), "pgrep", "-f", "daemon -state "+w.state).Output()
	return strings.Fields(string(out))
}

func (w world) daemonOf(t *testing.T, binary string) []string {
	t.Helper()
	out, _ := exec.CommandContext(context.Background(), "pgrep", "-f", binary+" daemon -state "+w.state).Output()
	return strings.Fields(string(out))
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

func (l *lockedBuffer) Len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Len()
}

type window struct {
	session *mcp.ClientSession
	stderr  *lockedBuffer
}

func (w world) open(t *testing.T, binary string, linger time.Duration) window {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), binary, "-state", w.state, "-linger", linger.String())
	cmd.Env = append(append(cmd.Environ(), "HOME="+w.home, "XDG_CACHE_HOME=", "XDG_CONFIG_HOME="), w.env...)
	stderr := &lockedBuffer{}
	cmd.Stderr = stderr
	session, err := mcp.NewClient(&mcp.Implementation{Name: "window", Version: "0"}, nil).Connect(t.Context(), &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("connect: %v (%s)", err, stderr)
	}
	return window{session: session, stderr: stderr}
}

func (win window) status(t *testing.T) string {
	t.Helper()
	result, err := win.session.CallTool(t.Context(), &mcp.CallToolParams{Name: "whatsapp_status", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("whatsapp_status: %v (%s)", err, win.stderr)
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("whatsapp_status returned %T", result.Content[0])
	}
	return text.Text
}

func waitFor(t *testing.T, what string, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestWindowsShareOneDaemon(t *testing.T) {
	t.Parallel()
	w := newWorld(t, shortHome(t))
	windows := make([]window, 5)
	var wg sync.WaitGroup
	for i := range windows {
		wg.Go(func() { windows[i] = w.open(t, builds[0], time.Second) })
	}
	wg.Wait()
	for _, win := range windows {
		if got := win.status(t); !strings.Contains(got, "not linked") {
			t.Fatalf("status = %q", got)
		}
		if win.stderr.Len() > 0 {
			t.Fatalf("a window served itself: %s", win.stderr)
		}
	}
	waitFor(t, "one daemon for five windows", func() bool { return len(w.daemons(t)) == 1 })
	if info, err := os.Stat(filepath.Dir(w.state)); err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("state dir: %v, %v", info, err)
	}
	for _, win := range windows {
		_ = win.session.Close()
	}
	waitFor(t, "the daemon to leave after its linger", func() bool { return len(w.daemons(t)) == 0 })
	later := w.open(t, builds[0], time.Second)
	if got := later.status(t); !strings.Contains(got, "not linked") {
		t.Fatalf("a window after the daemon left: %q", got)
	}
	_ = later.session.Close()
}

func TestANewBuildReplacesTheDaemon(t *testing.T) {
	t.Parallel()
	w := newWorld(t, shortHome(t))
	old := w.open(t, builds[0], time.Minute)
	old.status(t)
	if pids := w.daemonOf(t, builds[0]); len(pids) != 1 {
		t.Fatalf("daemons of the old build: %v", pids)
	}
	fresh := w.open(t, builds[1], time.Minute)
	if got := fresh.status(t); !strings.Contains(got, "not linked") || fresh.stderr.Len() > 0 {
		t.Fatalf("the new build's window: %q (%s)", got, fresh.stderr)
	}
	waitFor(t, "the old daemon to stop", func() bool { return len(w.daemonOf(t, builds[0])) == 0 })
	if pids := w.daemonOf(t, builds[1]); len(pids) != 1 {
		t.Fatalf("daemons of the new build: %v", pids)
	}
	_ = old.session.Close()
	_ = fresh.session.Close()
}

func TestWithoutADaemonAWindowServesItself(t *testing.T) {
	t.Parallel()
	long := filepath.Join(shortHome(t), strings.Repeat("h", 90))
	w := newWorld(t, long)
	w.env = []string{"TMPDIR=" + long, "XDG_RUNTIME_DIR=" + long}
	win := w.open(t, builds[0], time.Minute)
	if got := win.status(t); !strings.Contains(got, "not linked") {
		t.Fatalf("status = %q", got)
	}
	if !strings.Contains(win.stderr.String(), "serving this window on its own") || !strings.Contains(win.stderr.String(), "too long") {
		t.Fatalf("stderr = %q", win.stderr)
	}
	if pids := w.daemons(t); len(pids) != 0 {
		t.Fatalf("daemons: %v", pids)
	}
	_ = win.session.Close()
}
