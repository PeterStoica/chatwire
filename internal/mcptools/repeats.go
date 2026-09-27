package mcptools

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"sync"
	"time"
)

const repeatWindow = 30 * time.Second

type Repeats struct {
	mu    sync.Mutex
	calls map[string]*repeat
}

type repeat struct {
	done   chan struct{}
	report SendReport
	at     time.Time
}

func NewRepeats() *Repeats {
	return &Repeats{calls: map[string]*repeat{}}
}

func repeatKey(tool string, input any) string {
	raw, err := json.Marshal(input)
	if err != nil {
		return ""
	}
	return tool + "\x00" + string(raw)
}

func (r *Repeats) once(ctx context.Context, key string, send func() SendReport) SendReport {
	if key == "" {
		return send()
	}
	r.mu.Lock()
	now := time.Now()
	for k, c := range r.calls {
		if !c.at.IsZero() && now.Sub(c.at) > repeatWindow {
			delete(r.calls, k)
		}
	}
	if earlier, ok := r.calls[key]; ok {
		r.mu.Unlock()
		select {
		case <-earlier.done:
		case <-ctx.Done():
			return SendReport{State: stateFailed, Detail: "The same message is still being sent by an earlier request; check the chat before sending it again."}
		}
		if earlier.report.State != stateSent {
			return r.once(ctx, key, send)
		}
		repeated := earlier.report
		repeated.Detail = fmt.Sprintf("This exact message was already sent %s ago, so it was not sent twice. %s", time.Since(earlier.at).Round(time.Second), earlier.report.Detail)
		return repeated
	}
	c := &repeat{done: make(chan struct{})}
	r.calls[key] = c
	r.mu.Unlock()
	report := send()
	r.mu.Lock()
	c.report, c.at = report, time.Now()
	if report.State != stateSent {
		delete(r.calls, key)
	}
	r.mu.Unlock()
	close(c.done)
	return report
}
