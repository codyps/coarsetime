//go:build !purego

package coarsetime

import (
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func TestLinuxRealtimeClock(t *testing.T) {
	var ts syscall.Timespec
	if !readRealtimeVDSO(&ts) {
		t.Skip("coarse realtime vDSO unavailable")
	}
	read := func() int64 {
		var ts syscall.Timespec
		_, _, err := syscall.RawSyscall(syscall.SYS_CLOCK_GETTIME, 5, uintptr(unsafe.Pointer(&ts)), 0)
		if err != 0 {
			t.Fatal(err)
		}
		return int64(ts.Sec)*1_000_000_000 + int64(ts.Nsec)
	}
	for n := 0; n < 1000; n++ {
		before := read()
		got := UnixNano()
		now := Now().UnixNano()
		after := read()
		if after < before {
			continue
		} // System wall time is allowed to step backward.
		if got < before || got > after || now < before || now > after {
			t.Fatalf("wall reading %d outside [%d,%d]", got, before, after)
		}
	}
}

func testLinuxGoWallClock(t *testing.T) {
	for n := 0; n < 1000; n++ {
		before := time.Now().UnixNano()
		got := UnixNano()
		now := Now()
		after := time.Now().UnixNano()
		if after < before {
			continue
		}
		if got < before || got > after || now.UnixNano() < before || now.UnixNano() > after {
			t.Fatalf("Go wall readings %d, %v outside [%d,%d]", got, now, before, after)
		}
		if now != now.Round(0) || now.Location() != time.Local {
			t.Fatalf("Now must return local time without monotonic data: %v", now)
		}
	}
}
