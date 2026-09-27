package mcpapp

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/PeterStoica/chatwire/internal/daemon"
)

func TestTheSocketStaysShortWhateverTheHome(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix socket paths are checked on macOS and Linux")
	}
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	usual := "/Users/tester"
	t.Setenv("HOME", usual)
	path, err := socketPath(filepath.Join(usual, "state", "linked.json"))
	if err != nil || !strings.HasPrefix(path, usual) {
		t.Fatalf("an ordinary home: %q, %v; want the cache folder", path, err)
	}
	long := filepath.Join(t.TempDir(), strings.Repeat("sandbox", 20))
	t.Setenv("HOME", long)
	path, err = socketPath(filepath.Join(long, "state", "linked.json"))
	if err != nil || daemon.CheckPath(path) != nil || strings.HasPrefix(path, long) {
		t.Fatalf("a long home: %q, %v", path, err)
	}
	again, _ := socketPath(filepath.Join(long, "state", "linked.json"))
	if again != path {
		t.Fatalf("the shim and the daemon would disagree: %q and %q", path, again)
	}
}
