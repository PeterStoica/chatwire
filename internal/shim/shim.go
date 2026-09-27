package shim

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/PeterStoica/chatwire/internal/daemon"
	"github.com/PeterStoica/chatwire/internal/spawn"
)

const (
	startTimeout = 10 * time.Second
	pollEvery    = 20 * time.Millisecond
)

var ErrNoDaemon = errors.New("shim: daemon did not come up")

type Launcher struct {
	Socket     string
	Log        string
	Build      string
	Executable string
	DaemonArgs []string
}

func (l Launcher) Connect(ctx context.Context) (net.Conn, error) {
	path := l.Socket
	if err := daemon.CheckPath(path); err != nil {
		return nil, err
	}
	conn, err := l.greet(ctx, path)
	if err == nil {
		return conn, nil
	}
	if errors.Is(err, daemon.ErrRestart) {
		if err := l.awaitExit(ctx, path); err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(l.Log), 0o700); err != nil {
		return nil, fmt.Errorf("shim: log dir: %w", err)
	}
	if _, err := spawn.Detached(ctx, l.Executable, l.DaemonArgs, l.Log); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, startTimeout)
	defer cancel()
	ticker := time.NewTicker(pollEvery)
	defer ticker.Stop()
	for {
		if conn, err := l.greet(ctx, path); err == nil {
			return conn, nil
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("%w within %s", ErrNoDaemon, startTimeout)
		case <-ticker.C:
		}
	}
}

func (l Launcher) greet(ctx context.Context, path string) (net.Conn, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", path)
	if err != nil {
		return nil, fmt.Errorf("shim: dial: %w", err)
	}
	if err := daemon.Hello(conn, l.Build); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

func (l Launcher) awaitExit(ctx context.Context, path string) error {
	ctx, cancel := context.WithTimeout(ctx, startTimeout)
	defer cancel()
	ticker := time.NewTicker(pollEvery)
	defer ticker.Stop()
	for {
		conn, err := (&net.Dialer{}).DialContext(ctx, "unix", path)
		if err != nil {
			return nil
		}
		_ = conn.Close()
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w: the old daemon did not stop within %s", ErrNoDaemon, startTimeout)
		case <-ticker.C:
		}
	}
}

func Pipe(ctx context.Context, conn net.Conn, in io.Reader, out io.Writer) error {
	done := make(chan error, 2)
	go func() {
		_, err := io.Copy(conn, in)
		if closer, ok := conn.(interface{ CloseWrite() error }); ok {
			_ = closer.CloseWrite()
		}
		done <- err
	}()
	go func() {
		_, err := io.Copy(out, conn)
		done <- err
	}()
	select {
	case <-ctx.Done():
	case err := <-done:
		if err != nil {
			_ = conn.Close()
			return err
		}
		<-done
	}
	return conn.Close()
}
