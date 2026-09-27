package shim_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/PeterStoica/chatwire/internal/shim"
)

func TestTheShimStopsWhenItsParentIsGone(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var parent atomic.Int32
		parent.Store(4242)
		ctx, stop := shim.WhileParentLives(t.Context(), func() int { return int(parent.Load()) }, 5*time.Second)
		defer stop()
		synctest.Sleep(time.Minute)
		if ctx.Err() != nil {
			t.Fatal("stopped while the parent was alive")
		}
		start := time.Now()
		parent.Store(1)
		<-ctx.Done()
		if waited := time.Since(start); waited > 5*time.Second || !errors.Is(ctx.Err(), context.Canceled) {
			t.Fatalf("noticed after %s: %v", waited, ctx.Err())
		}
	})
}
