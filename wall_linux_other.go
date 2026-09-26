//go:build !purego && linux && !amd64

package coarsetime

import "syscall"

// CLOCK_REALTIME_COARSE already includes the kernel's wall-clock correction.
// Reading it directly is cheaper and fresher than maintaining our own offset.
func readWallUnixNano() int64 {
	var ts syscall.Timespec
	if readRealtimeVDSO(&ts) {
		return int64(ts.Sec)*1_000_000_000 + int64(ts.Nsec)
	}
	return readRealtimeSyscall()
}
