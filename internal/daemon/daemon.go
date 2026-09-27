package daemon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	maxSocketPath = 100
	maxLine       = 255
	helloTimeout  = 5 * time.Second
	greeting      = "chatwire"
	replyOK       = "ok"
	replyRestart  = "restart"
)

var (
	ErrRunning      = errors.New("daemon: another daemon is already serving")
	ErrPathTooLong  = errors.New("daemon: socket path too long for this platform")
	ErrRestart      = errors.New("daemon: the running daemon is another build and is restarting")
	ErrHello        = errors.New("daemon: bad hello")
	errStaleRemoved = errors.New("daemon: stale socket removed")
)

func CheckPath(path string) error {
	if len(path) > maxSocketPath {
		return fmt.Errorf("%w: %d bytes: %s", ErrPathTooLong, len(path), path)
	}
	return nil
}

func Listen(ctx context.Context, path string) (net.Listener, error) {
	if err := CheckPath(path); err != nil {
		return nil, err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("daemon: socket dir: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, fmt.Errorf("daemon: socket dir permissions: %w", err)
	}
	for range 2 {
		listener, err := (&net.ListenConfig{}).Listen(ctx, "unix", path)
		if err == nil {
			if err := os.Chmod(path, 0o600); err != nil {
				_ = listener.Close()
				return nil, fmt.Errorf("daemon: socket permissions: %w", err)
			}
			return listener, nil
		}
		if conn, dialErr := (&net.Dialer{Timeout: time.Second}).DialContext(ctx, "unix", path); dialErr == nil {
			_ = conn.Close()
			return nil, ErrRunning
		}
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return nil, fmt.Errorf("daemon: remove stale socket: %w", removeErr)
		}
	}
	return nil, fmt.Errorf("daemon: listen %s: %w", path, errStaleRemoved)
}

func Hello(conn net.Conn, build string) error {
	if err := conn.SetDeadline(time.Now().Add(helloTimeout)); err != nil {
		return fmt.Errorf("%w: %w", ErrHello, err)
	}
	if _, err := io.WriteString(conn, greeting+" "+build+"\n"); err != nil {
		return fmt.Errorf("%w: %w", ErrHello, err)
	}
	reply, err := readLine(conn)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrHello, err)
	}
	switch reply {
	case replyOK:
		return conn.SetDeadline(time.Time{})
	case replyRestart:
		return ErrRestart
	}
	return fmt.Errorf("%w: reply %q", ErrHello, reply)
}

func readLine(conn net.Conn) (string, error) {
	var line strings.Builder
	one := make([]byte, 1)
	for {
		if _, err := conn.Read(one); err != nil {
			return "", err
		}
		switch {
		case one[0] == '\n':
			return line.String(), nil
		case line.Len() >= maxLine:
			return "", fmt.Errorf("%w: line too long", ErrHello)
		}
		line.WriteByte(one[0])
	}
}

type Session func(ctx context.Context, conn io.ReadWriteCloser)

type Server struct {
	listener net.Listener
	build    string
	linger   time.Duration
	session  Session
	mu       sync.Mutex
	sessions int
	idle     *time.Timer
	stop     context.CancelFunc
}

func New(listener net.Listener, build string, linger time.Duration, session Session) *Server {
	return &Server{listener: listener, build: build, linger: linger, session: session}
}

func (s *Server) Serve(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	s.mu.Lock()
	s.stop = cancel
	s.idle = time.AfterFunc(s.linger, cancel)
	s.mu.Unlock()
	go func() {
		<-ctx.Done()
		_ = s.listener.Close()
	}()
	var sessions sync.WaitGroup
	defer sessions.Wait()
	for {
		conn, err := s.listener.Accept()
		switch {
		case err == nil:
			s.begin()
			sessions.Go(func() {
				defer s.end()
				defer conn.Close()
				if s.greet(conn) {
					s.session(ctx, conn)
				}
			})
		case ctx.Err() != nil:
			return nil
		default:
			return fmt.Errorf("daemon: accept: %w", err)
		}
	}
}

func (s *Server) greet(conn net.Conn) bool {
	if err := conn.SetDeadline(time.Now().Add(helloTimeout)); err != nil {
		return false
	}
	line, err := readLine(conn)
	name, build, _ := strings.Cut(line, " ")
	switch {
	case err != nil || name != greeting || build == "":
		return false
	case build != s.build:
		_, _ = io.WriteString(conn, replyRestart+"\n")
		s.stop()
		return false
	}
	if _, err := io.WriteString(conn, replyOK+"\n"); err != nil {
		return false
	}
	return conn.SetDeadline(time.Time{}) == nil
}

func (s *Server) begin() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions++
	s.idle.Stop()
}

func (s *Server) end() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions--
	if s.sessions == 0 {
		s.idle.Reset(s.linger)
	}
}
