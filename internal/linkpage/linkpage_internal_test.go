package linkpage

import (
	"testing"
	"time"
)

func TestAPageLeftAloneStopsCounting(t *testing.T) {
	t.Parallel()
	p := &Page{}
	p.polled, p.seen = time.Now().Add(-watchedFor+time.Second), time.Now().Add(-watchedFor+time.Second)
	if !p.Open() || !p.Seen() {
		t.Fatal("a page polled a moment ago stopped counting")
	}
	p.polled, p.seen = time.Now().Add(-watchedFor), time.Now().Add(-watchedFor)
	if p.Open() || p.Seen() {
		t.Fatal("a page nobody polled for a while still counts")
	}
}
