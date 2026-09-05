package coarsetime

import (
	"syscall"
	"time"
	"unsafe"
)

// Now returns an approximate local wall-clock time. It reads the coarse
// clock: Linux reads CLOCK_REALTIME_COARSE directly; other platforms apply a
// cached correction without reading the system wall clock.
// The result has no Go monotonic component and can move backward.
// See RefreshWallClock for the accuracy and suspend tradeoffs.
func Now() time.Time {
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

// UnixNano returns the approximate current Unix timestamp in nanoseconds. It
// avoids constructing a time.Time and uses the same source as Now.
// Nanosecond units do not imply nanosecond
// accuracy. Like time.Time.UnixNano, the result is undefined outside the range
// representable by int64 Unix nanoseconds (roughly years 1678 through 2262).
func UnixNano() int64 {
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
