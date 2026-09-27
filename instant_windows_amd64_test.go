//go:build !purego

package coarsetime

import (
	"runtime"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// Compare the shared-page read with the Windows API, independently of Go's
// monotonic implementation. KernelBase exports this API on the test host;
// Kernel32 does not on all supported systems.
func TestWindowsInterruptTime(t *testing.T) {
	query := syscall.NewLazyDLL("kernelbase.dll").NewProc("QueryInterruptTime")
	if err := query.Find(); err != nil {
		t.Skipf("QueryInterruptTime unavailable: %v", err)
	}
	addr := query.Addr()
	var before, after uint64
	for n := 0; n < 1000; n++ {
		syscall.SyscallN(addr, uintptr(unsafe.Pointer(&before)))
		got := NowInstant()
		syscall.SyscallN(addr, uintptr(unsafe.Pointer(&after)))
		if got.ticks < before || got.ticks > after {
			t.Fatalf("interrupt ticks outside API bounds: %d <= %d <= %d", before, got.ticks, after)
		}
	}
	runtime.KeepAlive(&before)
	runtime.KeepAlive(&after)
}

func TestWindowsInterruptDuration(t *testing.T) {
	// Pin the public conversion to Windows' documented units. Generic arithmetic
	// tests also cover saturation and large differences for this platform's scale.
	start, end := Instant{123}, Instant{133}
	if got := end.Sub(start); got != time.Microsecond {
		t.Fatalf("10 interrupt ticks = %v, want 1us", got)
	}
	if got := start.Sub(end); got != -time.Microsecond {
		t.Fatalf("-10 interrupt ticks = %v, want -1us", got)
	}
}
