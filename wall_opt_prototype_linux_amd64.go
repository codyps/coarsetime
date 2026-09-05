//go:build walloptprototype

package coarsetime

import (
	"syscall"
	"time"
	"unsafe"
)

func wallOptTimespecNow() time.Time {
	var ts syscall.Timespec
	if readRealtimeVDSO(&ts) {
		return time.Unix(ts.Sec, ts.Nsec)
	}
	return time.Unix(0, readRealtimeSyscall())
}

// Same runtime system-stack bridge and vDSO entry point as production. Fuse
// the helper to avoid an extra call and timespec copy; do not call the C ABI
// directly on a goroutine stack.
func wallOptFusedNow() time.Time {
	fn := coarseVDSO
	if fn != 0 {
		args := struct {
			fn uintptr
			ts syscall.Timespec
		}{fn: fn}
		if asmcgocall(realtimeTrampoline, unsafe.Pointer(&args)) == 0 {
			return time.Unix(args.ts.Sec, args.ts.Nsec)
		}
	}
	return time.Unix(0, readRealtimeSyscall())
}

func wallOptFusedUnixNano() int64 {
	fn := coarseVDSO
	if fn != 0 {
		args := struct {
			fn uintptr
			ts syscall.Timespec
		}{fn: fn}
		if asmcgocall(realtimeTrampoline, unsafe.Pointer(&args)) == 0 {
			return args.ts.Sec*1e9 + args.ts.Nsec
		}
	}
	return readRealtimeSyscall()
}
