//go:build !purego

package coarsetime

import (
	"runtime"
	"syscall"
	"unsafe"
)

func readRealtimeSyscall() int64 {
	var ts syscall.Timespec
	// Linux's legacy 32-bit timespec cannot represent wall dates after 2038.
	// Prefer clock_gettime64 there; older kernels may only provide the legacy ABI.
	// This branch is compiled away on 64-bit targets.
	if unsafe.Sizeof(ts.Sec) == 4 {
		var full struct{ sec, nsec int64 }
		trap := uintptr(403)
		if runtime.GOARCH == "mips" || runtime.GOARCH == "mipsle" {
			trap += 4000
		}
		_, _, errno := syscall.RawSyscall(trap, 5, uintptr(unsafe.Pointer(&full)), 0)
		if errno == 0 {
			return full.sec*1_000_000_000 + full.nsec
		}
		if errno != syscall.ENOSYS {
			panic("coarsetime: CLOCK_REALTIME_COARSE failed: " + errno.Error())
		}
	}
	_, _, errno := syscall.RawSyscall(syscall.SYS_CLOCK_GETTIME, 5, uintptr(unsafe.Pointer(&ts)), 0)
	if errno != 0 {
		panic("coarsetime: CLOCK_REALTIME_COARSE failed: " + errno.Error())
	}
	return int64(ts.Sec)*1_000_000_000 + int64(ts.Nsec)
}
