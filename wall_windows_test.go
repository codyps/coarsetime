//go:build !purego && (amd64 || arm64)

package coarsetime

import (
	"syscall"
	"testing"
)

func TestWindowsSystemTime(t *testing.T) {
	var before, after syscall.Filetime
	for n := 0; n < 1000; n++ {
		syscall.GetSystemTimeAsFileTime(&before)
		got := readWindowsFiletime()
		syscall.GetSystemTimeAsFileTime(&after)
		lo := uint64(before.HighDateTime)<<32 | uint64(before.LowDateTime)
		hi := uint64(after.HighDateTime)<<32 | uint64(after.LowDateTime)
		if hi < lo {
			continue // Wall time can be adjusted backwards.
		}
		if got < lo || got > hi {
			t.Fatalf("system ticks outside API bounds: %d <= %d <= %d", lo, got, hi)
		}
	}
}

func TestFiletimeUnixNano(t *testing.T) {
	for _, ns := range []int64{
		-9223372036854775800, // First representable 100 ns tick.
		-2208988800000000000, // 1900-01-01.
		-100, 0, 100,
		946684800000000000,  // 2000-01-01.
		9223372036854775800, // Last representable 100 ns tick.
	} {
		ticks := uint64(int64(windowsUnixEpoch) + ns/100)
		if got := filetimeUnixNano(ticks); got != ns {
			t.Errorf("FILETIME %d: got %d, want %d", ticks, got, ns)
		}
	}
}
