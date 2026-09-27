//go:build !purego

package coarsetime

import (
	"syscall"
	"testing"
	"unsafe"
)

func TestLinuxRealtimeClock(t *testing.T) {
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
