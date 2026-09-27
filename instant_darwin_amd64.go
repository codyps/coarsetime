//go:build !purego

package coarsetime

import "sync/atomic"

// XNU's approximate-time support is a boot-time capability. The commpage is
// mapped read-only in userspace; the kernel publishes aligned uint64 samples.
// These private ABI offsets are isolated in the architecture assembly file.
func commpageApproximateTimeSupported() bool
func commpageApproximateTimeAddress() *uint64

var approximateTimeSupported = commpageApproximateTimeSupported()
var approximateTimePointer = commpageApproximateTimeAddress()

func readTicks() uint64 {
	return readCommpageTicks(approximateTimeSupported)
}

func readCommpageTicks(supported bool) uint64 {
	if supported {
		// Atomic forces a fresh load on each call. Assembly supplies the
		// foreign pointer without an integer-to-pointer conversion in Go.
		return atomic.LoadUint64(approximateTimePointer)
	}
	return readTicksFallback()
}

// Outline the uncommon bridge so readTicks and NowInstant remain inlineable.
//
//go:noinline
func readTicksFallback() uint64 { return readLibcTicks() }
