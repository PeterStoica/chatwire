package mcptools

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
)

func TestConcurrentRepeatsSendOnce(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := NewRepeats()
		release := make(chan struct{})
		var sends atomic.Int32
		send := func() SendReport {
			sends.Add(1)
			<-release
			return SendReport{State: stateSent, ID: "3EB0A"}
		}
		var wg sync.WaitGroup
		reports := make([]SendReport, 3)
		for i := range reports {
			wg.Go(func() { reports[i] = r.once(t.Context(), "k", send) })
		}
		synctest.Wait()
		close(release)
		wg.Wait()
		if sends.Load() != 1 {
			t.Fatalf("sent %d times", sends.Load())
		}
		for _, report := range reports {
			if report.ID != "3EB0A" || report.State != stateSent {
				t.Fatalf("reports = %+v", reports)
			}
		}
	})
}

func TestARepeatAfterAFailureSendsAgain(t *testing.T) {
	r := NewRepeats()
	var sends int
	outcome := []string{stateFailed, stateSent}
	send := func() SendReport {
		sends++
		return SendReport{State: outcome[sends-1]}
	}
	if first := r.once(t.Context(), "k", send); first.State != stateFailed {
		t.Fatalf("first = %+v", first)
	}
	if second := r.once(t.Context(), "k", send); second.State != stateSent || sends != 2 {
		t.Fatalf("second = %+v after %d sends", second, sends)
	}
	if other := r.once(t.Context(), "", func() SendReport { sends++; return SendReport{State: stateSent} }); other.State != stateSent || sends != 3 {
		t.Fatal("a call without a key is never held back")
	}
}

func TestAWaitingRepeatCanGiveUp(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := NewRepeats()
		release := make(chan struct{})
		go r.once(t.Context(), "k", func() SendReport { <-release; return SendReport{State: stateSent} })
		synctest.Wait()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if report := r.once(ctx, "k", func() SendReport { t.Fatal("sent twice"); return SendReport{} }); report.State != stateFailed {
			t.Fatalf("report = %+v", report)
		}
		close(release)
	})
}
