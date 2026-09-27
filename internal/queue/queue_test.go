package queue_test

import (
	"slices"
	"sync"
	"testing"

	"github.com/PeterStoica/chatwire/internal/queue"
)

func TestPushNeverWaitsAndTakeKeepsTheOrder(t *testing.T) {
	q := queue.New[int]()
	for i := range 10000 {
		q.Push(i)
	}
	got := q.Take()
	if len(got) != 10000 || !slices.IsSorted(got) {
		t.Fatalf("took %d items in order %v", len(got), slices.IsSorted(got))
	}
	if rest := q.Take(); rest != nil {
		t.Fatalf("a second Take returned %v", rest)
	}
}

func TestReadyWakesAConsumerForEveryPush(t *testing.T) {
	q := queue.New[int]()
	const producers, each = 8, 500
	var wg sync.WaitGroup
	for p := range producers {
		wg.Go(func() {
			for i := range each {
				q.Push(p*each + i)
			}
		})
	}
	seen := map[int]bool{}
	for len(seen) < producers*each {
		<-q.Ready()
		for _, v := range q.Take() {
			seen[v] = true
		}
	}
	wg.Wait()
	if extra := q.Take(); len(extra) != 0 {
		t.Fatalf("%d items left after all were seen", len(extra))
	}
}
