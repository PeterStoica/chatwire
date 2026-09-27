package signal

import (
	"strconv"
	"testing"
)

func BenchmarkMergeSessions(b *testing.B) {
	for _, count := range []int{1, maxArchivedStates + 1} {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			left := &Session{states: make([]*state, count)}
			right := &Session{states: make([]*state, count)}
			b.ReportAllocs()
			for b.Loop() {
				left.Merge(right)
			}
		})
	}
}
