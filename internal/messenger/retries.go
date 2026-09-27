package messenger

import (
	"errors"
	"math/rand/v2"
	"time"

	"github.com/PeterStoica/chatwire/internal/linkflow"
)

const (
	steadyAfter  = 30 * time.Second
	slowRetry    = time.Hour
	replacedWait = 30 * time.Second
	minBanWait   = time.Minute
)

type retries struct {
	wait      time.Duration
	failures  int
	replaced  bool
	refreshed bool
}

func newRetries() retries {
	return retries{wait: firstRetry}
}

func (r *retries) next(err error, lasted time.Duration) (delay time.Duration, refresh bool) {
	if lasted >= steadyAfter {
		*r = newRetries()
	}
	r.failures++
	var ban linkflow.Ban
	switch {
	case errors.As(err, &ban):
		return max(ban.For, minBanWait), false
	case errors.Is(err, linkflow.ErrReplaced) && !r.replaced:
		r.replaced = true
		return replacedWait, false
	case errors.Is(err, linkflow.ErrReplaced), errors.Is(err, linkflow.ErrClient):
		return slowRetry, false
	case errors.Is(err, linkflow.ErrOutdated) && !r.refreshed:
		r.refreshed = true
		return 0, true
	case errors.Is(err, linkflow.ErrOutdated):
		r.refreshed = false
		return slowRetry, false
	case lasted >= steadyAfter:
		return 0, false
	}
	delay = jittered(r.wait)
	r.wait = min(2*r.wait, maxRetry)
	return delay, false
}

func jittered(d time.Duration) time.Duration {
	return time.Duration(float64(d) * (0.8 + 0.4*rand.Float64()))
}
