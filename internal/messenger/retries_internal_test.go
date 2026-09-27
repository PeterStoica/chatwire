package messenger

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/PeterStoica/chatwire/internal/linkflow"
)

func TestHowLongToWaitBeforeReconnecting(t *testing.T) {
	t.Parallel()
	dropped := errors.New("connection reset")
	near := func(got, want time.Duration) bool { return got >= want*8/10 && got <= want*12/10 }

	r := newRetries()
	for _, want := range []time.Duration{time.Second, 2 * time.Second, 4 * time.Second} {
		if got, _ := r.next(dropped, time.Second); !near(got, want) {
			t.Fatalf("a quick drop: wait %s, want about %s", got, want)
		}
	}
	if got, _ := r.next(dropped, time.Hour); got != 0 || r.failures != 1 {
		t.Fatalf("after a long session: wait %s with %d failures, want an immediate retry", got, r.failures)
	}
	if got, _ := r.next(dropped, time.Second); !near(got, time.Second) {
		t.Fatalf("the backoff did not start over: %s", got)
	}

	r = newRetries()
	ban := fmt.Errorf("login: %w", linkflow.Ban{Code: 101, For: 3 * time.Hour})
	if got, _ := r.next(ban, 0); got != 3*time.Hour {
		t.Fatalf("a ban: wait %s", got)
	}
	if got, _ := r.next(linkflow.Ban{Code: 101}, 0); got != minBanWait {
		t.Fatalf("a ban with no end: wait %s", got)
	}

	r = newRetries()
	if got, _ := r.next(linkflow.ErrReplaced, time.Minute); got != replacedWait {
		t.Fatalf("first replacement: wait %s", got)
	}
	if got, _ := r.next(linkflow.ErrReplaced, time.Second); got != slowRetry {
		t.Fatalf("second replacement: wait %s", got)
	}

	r = newRetries()
	if got, refresh := r.next(linkflow.ErrOutdated, 0); got != 0 || !refresh {
		t.Fatalf("first outdated: wait %s, refresh %v", got, refresh)
	}
	if got, refresh := r.next(linkflow.ErrOutdated, 0); got != slowRetry || refresh {
		t.Fatalf("outdated again: wait %s, refresh %v", got, refresh)
	}
	if got, refresh := r.next(linkflow.ErrOutdated, 0); got != 0 || !refresh {
		t.Fatalf("an hour later it refreshes again: wait %s, refresh %v", got, refresh)
	}
	if got, _ := newRetriesAt(linkflow.ErrClient); got != slowRetry {
		t.Fatalf("a refused client: wait %s", got)
	}
}

func newRetriesAt(err error) (time.Duration, bool) {
	r := newRetries()
	return r.next(err, 0)
}
