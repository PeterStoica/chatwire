package daemon_test

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/PeterStoica/chatwire/internal/daemon"
)

func socket(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("unix sockets on Windows need a Windows runner")
	}
	dir, err := os.MkdirTemp("/tmp", "chatwire-test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "run", "daemon.sock")
}

func echo(_ context.Context, conn io.ReadWriteCloser) {
	_, _ = io.Copy(conn, conn)
}

func serve(t *testing.T, path, build string, linger time.Duration) chan error {
	t.Helper()
	listener, err := daemon.Listen(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- daemon.New(listener, build, linger, echo).Serve(t.Context()) }()
	return done
}

func dial(t *testing.T, path string) net.Conn {
	t.Helper()
	conn, err := (&net.Dialer{}).DialContext(t.Context(), "unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func TestSessionsStartAfterTheHello(t *testing.T) {
	t.Parallel()
	path := socket(t)
	serve(t, path, "build-a", time.Minute)
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 && info.Mode()&os.ModeSocket == 0 {
		t.Fatalf("socket: %v, %v", info, err)
	}
	if dir, err := os.Stat(filepath.Dir(path)); err != nil || dir.Mode().Perm() != 0o700 {
		t.Fatalf("socket dir: %v, %v", dir.Mode(), err)
	}
	conn := dial(t, path)
	if err := daemon.Hello(conn, "build-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(conn, "ping\n"); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, 5)
	if _, err := io.ReadFull(conn, got); err != nil || string(got) != "ping\n" {
		t.Fatalf("echo = %q, %v", got, err)
	}
}

func TestAnotherBuildRestartsTheDaemon(t *testing.T) {
	t.Parallel()
	path := socket(t)
	done := serve(t, path, "build-a", time.Minute)
	if err := daemon.Hello(dial(t, path), "build-b"); !errors.Is(err, daemon.ErrRestart) {
		t.Fatalf("Hello() = %v, want ErrRestart", err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the daemon kept serving after a newer build asked it to stop")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the socket is still there: %v", err)
	}
}

func TestBadHellosGetNoSession(t *testing.T) {
	t.Parallel()
	path := socket(t)
	serve(t, path, "build-a", time.Minute)
	for _, hello := range []string{"hello build-a\n", "chatwire\n", strings.Repeat("x", 300)} {
		conn := dial(t, path)
		if _, err := io.WriteString(conn, hello); err != nil {
			t.Fatal(err)
		}
		if reply, err := io.ReadAll(conn); err != nil || len(reply) != 0 {
			t.Fatalf("hello %.20q got %q, %v", hello, reply, err)
		}
	}
	conn := dial(t, path)
	if err := daemon.Hello(conn, "build-a"); err != nil {
		t.Fatalf("the daemon stopped serving after bad hellos: %v", err)
	}
}

func TestHelloRefusesStrangeReplies(t *testing.T) {
	t.Parallel()
	for _, reply := range []string{"maybe\n", "", strings.Repeat("y", 300)} {
		client, server := net.Pipe()
		go func() {
			_, _ = io.ReadAll(io.LimitReader(server, int64(len("chatwire build-a\n"))))
			_, _ = io.WriteString(server, reply)
			_ = server.Close()
		}()
		if err := daemon.Hello(client, "build-a"); !errors.Is(err, daemon.ErrHello) {
			t.Errorf("reply %.20q: Hello() = %v, want ErrHello", reply, err)
		}
		_ = client.Close()
	}
}

func TestOneDaemonPerSocket(t *testing.T) {
	t.Parallel()
	path := socket(t)
	serve(t, path, "build-a", time.Minute)
	if _, err := daemon.Listen(t.Context(), path); !errors.Is(err, daemon.ErrRunning) {
		t.Fatalf("second Listen() = %v, want ErrRunning", err)
	}
	stale := socket(t)
	if err := os.MkdirAll(filepath.Dir(stale), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	listener, err := daemon.Listen(t.Context(), stale)
	if err != nil {
		t.Fatalf("a stale socket file blocked the daemon: %v", err)
	}
	_ = listener.Close()
	long := "/tmp/" + strings.Repeat("d", 100) + "/daemon.sock"
	if _, err := daemon.Listen(t.Context(), long); !errors.Is(err, daemon.ErrPathTooLong) {
		t.Fatalf("Listen() on a long path = %v", err)
	}
	if err := daemon.CheckPath(long); !errors.Is(err, daemon.ErrPathTooLong) {
		t.Fatalf("CheckPath() = %v", err)
	}
}

func TestTheDaemonLingersAfterItsLastSession(t *testing.T) {
	t.Parallel()
	path := socket(t)
	linger := 300 * time.Millisecond
	start := time.Now()
	done := serve(t, path, "build-a", linger)
	conn := dial(t, path)
	if err := daemon.Hello(conn, "build-a"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * linger)
	select {
	case <-done:
		t.Fatal("the daemon stopped while a session was open")
	default:
	}
	closed := time.Now()
	_ = conn.Close()
	select {
	case err := <-done:
		if err != nil || time.Since(closed) < linger || time.Since(start) < 3*linger {
			t.Fatalf("stopped after %s (session closed %s ago): %v", time.Since(start), time.Since(closed), err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the daemon never stopped")
	}
}

func TestHelloLinesUpToTheLimit(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		build string
		ok    bool
	}{
		{name: "the longest line", build: strings.Repeat("b", 255-len("chatwire ")), ok: true},
		{name: "one byte more", build: strings.Repeat("b", 256-len("chatwire ")), ok: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path := socket(t)
			serve(t, path, tt.build, time.Minute)
			if err := daemon.Hello(dial(t, path), tt.build); (err == nil) != tt.ok {
				t.Fatalf("Hello() = %v, want ok %v", err, tt.ok)
			}
		})
	}
}

func TestSocketPathLimit(t *testing.T) {
	t.Parallel()
	for _, n := range []int{100, 101} {
		path := "/" + strings.Repeat("s", n-1)
		if err := daemon.CheckPath(path); errors.Is(err, daemon.ErrPathTooLong) != (n > 100) {
			t.Errorf("a %d byte path: %v", n, err)
		}
	}
}
