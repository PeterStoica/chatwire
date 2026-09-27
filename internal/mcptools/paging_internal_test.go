package mcptools

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/PeterStoica/chatwire/internal/node"
	"github.com/PeterStoica/chatwire/internal/store"
)

type pagingPhone struct {
	Sender
	stored  []store.Message
	batches []int
	asked   int
}

func (p *pagingPhone) Older(context.Context, node.JID) (int, error) {
	if p.asked >= len(p.batches) {
		return 0, nil
	}
	n := p.batches[p.asked]
	p.asked++
	oldest := p.stored[0].Time
	batch := make([]store.Message, n)
	for i := range batch {
		batch[i] = store.Message{ID: fmt.Sprintf("OLD%d-%d", p.asked, i), Time: oldest.Add(-time.Duration(n-i) * time.Minute)}
	}
	p.stored = append(batch, p.stored...)
	return n, nil
}

func (p *pagingPhone) Messages(_ context.Context, q store.Query) ([]store.Message, error) {
	var before []store.Message
	for _, m := range p.stored {
		if m.Time.Before(q.Before) {
			before = append(before, m)
		}
	}
	return before[max(0, len(before)-q.Limit):], nil
}

func TestPagingBackKeepsAskingThePhoneUntilThePageIsFull(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_790_000_000, 0)
	chat := node.JID{User: "40722222222", Server: node.ServerUser}
	for _, tt := range []struct {
		name        string
		batches     []int
		limit       int
		wantAsked   int
		wantFetched int
		wantFound   int
	}{
		{name: "three rounds fill a page of 120", batches: []int{50, 50, 50, 50}, limit: 120, wantAsked: 3, wantFetched: 150, wantFound: 120},
		{name: "the phone runs out", batches: []int{50, 0}, limit: 200, wantAsked: 2, wantFetched: 50, wantFound: 50},
		{name: "rounds are capped", batches: []int{1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, limit: 200, wantAsked: phoneRounds, wantFetched: phoneRounds, wantFound: phoneRounds},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			phone := &pagingPhone{stored: []store.Message{{ID: "NEWEST", Time: now}}, batches: tt.batches}
			q := store.Query{Chat: chat, Before: now, Limit: tt.limit}
			fetched, found := olderFromPhone(t.Context(), phone, q, nil)
			if phone.asked != tt.wantAsked || fetched != tt.wantFetched || len(found) != tt.wantFound {
				t.Fatalf("asked %d times, fetched %d, found %d", phone.asked, fetched, len(found))
			}
		})
	}
}

func TestCatchingUpSaysWhenThereMayBeMore(t *testing.T) {
	t.Parallel()
	full := countDetail(ReadInput{Unread: true}, store.Query{}, make([]ReceivedMessage, maxRead))
	some := countDetail(ReadInput{Unread: true}, store.Query{}, make([]ReceivedMessage, 3))
	if !strings.Contains(full, "may be more") || !strings.Contains(full, "unread=true") || strings.Contains(some, "may be more") {
		t.Fatalf("at the limit: %q; below it: %q", full, some)
	}
}
