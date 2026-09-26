package coarsetime

import (
	"syscall"
	"time"
	"unsafe"
)

func readWallTime() time.Time {
	// Keep the timespec through construction and invoke the C ABI on the
	// runtime system stack, using the same bridge as the monotonic reader.
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

func readWallUnixNano() int64 {
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
