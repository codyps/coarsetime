//go:build !purego

package coarsetime

import (
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func TestLinuxCoarseClock(t *testing.T) {
	if !linuxCoarseClock {
		t.Skip("Go monotonic clock selected at startup")
	}
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

// Retain the old implementation only as a test oracle and benchmark baseline.
func readCoarseSyscall(ts *syscall.Timespec) syscall.Errno {
	_, _, errno := syscall.RawSyscall(syscall.SYS_CLOCK_GETTIME,
		clockMonotonicCoarse, uintptr(unsafe.Pointer(ts)), 0)
	return errno
}

func TestLinuxGoMonotonicClock(t *testing.T) {
	saved := linuxCoarseClock
	linuxCoarseClock = false
	defer func() { linuxCoarseClock = saved }()
	for n := 0; n < 1000; n++ {
		before := time.Since(linuxOrigin)
		got := NowInstant()
		after := time.Since(linuxOrigin)
		if got.ticks < uint64(before) || got.ticks > uint64(after) {
			t.Fatalf("Go monotonic reading %d outside [%d,%d]", got.ticks, before, after)
		}
	}
	TestClockProgress(t)
	TestConcurrentReads(t)
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
