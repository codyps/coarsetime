//go:build !purego

package coarsetime

import (
	"syscall"
	"unsafe"
)

const clockMonotonicCoarse = 6
const clockNumer, clockDenom = uint64(1), uint64(1)

func readTicks() uint64 {
	var ts syscall.Timespec
	if !readCoarseVDSO(&ts) {
		if errno := readCoarseSyscall(&ts); errno != 0 {
			// Do not silently change epochs or return a bogus timestamp on failure.
			panic("coarsetime: CLOCK_MONOTONIC_COARSE failed: " + errno.Error())
		}
	}
	return uint64(ts.Sec)*1_000_000_000 + uint64(ts.Nsec)
}

func readCoarseSyscall(ts *syscall.Timespec) syscall.Errno {
	_, _, errno := syscall.RawSyscall(syscall.SYS_CLOCK_GETTIME,
		clockMonotonicCoarse, uintptr(unsafe.Pointer(ts)), 0)
	return errno
}
