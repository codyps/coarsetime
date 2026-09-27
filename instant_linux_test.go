//go:build !purego

package coarsetime

import (
	"syscall"
	"testing"
	"time"
)

func TestLinuxCoarseClock(t *testing.T) {
	for n := 0; n < 1000; n++ {
		var before, after syscall.Timespec
		if err := readCoarseSyscall(&before); err != 0 {
			t.Fatal(err)
		}
		current := NowInstant()
		if err := readCoarseSyscall(&after); err != 0 {
			t.Fatal(err)
		}
		lo := uint64(before.Sec)*1_000_000_000 + uint64(before.Nsec)
		hi := uint64(after.Sec)*1_000_000_000 + uint64(after.Nsec)
		if current.ticks < lo || current.ticks > hi {
			t.Fatalf("NowInstant = %d, outside coarse syscall bracket [%d, %d]", current.ticks, lo, hi)
		}
	}
}

func BenchmarkLinuxCoarseSyscall(b *testing.B) {
	var ts syscall.Timespec
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		if err := readCoarseSyscall(&ts); err != 0 {
			b.Fatal(err)
		}
	}
	durationSink = time.Duration(ts.Nano())
}
