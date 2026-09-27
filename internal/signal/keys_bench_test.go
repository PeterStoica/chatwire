package signal

import (
	"strconv"
	"testing"
)

func BenchmarkEncryptCBC(b *testing.B) {
	for _, size := range []int{0, 15, 16, 256, 4096} {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			plaintext := make([]byte, size)
			keys := messageKeys{}
			b.ReportAllocs()
			for b.Loop() {
				if _, err := encryptCBC(keys, plaintext); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
