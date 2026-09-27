package mcpapp

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestOneChatwirePerLink(t *testing.T) {
	dir := t.TempDir()
	first, err := acquire(t.Context(), dir, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, err = acquire(t.Context(), dir, 300*time.Millisecond)
	if !errors.Is(err, ErrInUse) || !strings.Contains(err.Error(), strconv.Itoa(os.Getpid())) {
		t.Fatalf("a second holder: %v", err)
	}
	if waited := time.Since(start); waited < 300*time.Millisecond {
		t.Fatalf("gave up after %s without waiting for the first to finish", waited)
	}
	released := make(chan struct{})
	go func() {
		time.Sleep(300 * time.Millisecond)
		_ = first.Close()
		close(released)
	}()
	second, err := acquire(t.Context(), dir, 5*time.Second)
	<-released
	if err != nil {
		t.Fatalf("after the first let go: %v", err)
	}
	_ = second.Close()
}
