//go:build commpageprototype && darwin && (amd64 || arm64)

package coarsetime

import (
	"sync/atomic"
	"time"
)

// Experimental readers only: enable with -tags commpageprototype.
// Offsets come from xnu-12377.1.9 osfmk/{i386,arm}/cpu_capabilities.h.
// Both architectures have an aligned 64-bit APPROX_TIME slot. The kernel
// publishes monotonically newer samples; no multi-field snapshot is needed.
// Support is treated as a boot-time capability by the once-checked variants.
// None of these readers improve the freshness of the cached wall correction.
func prototypeApproxSupported() bool
func prototypeApproxAddress() *uint64
func prototypeApproxLoad() uint64

// Returns zero when unsupported; a zero supported sample is also safe to send
// through the libc fallback. Ordinary post-boot samples are nonzero.
func prototypeApproxCheckedLoad() uint64

var prototypeSupported = prototypeApproxSupported()
var prototypeApproxPointer = prototypeApproxAddress()

func prototypeCheckedTicks() uint64 {
	if ticks := prototypeApproxCheckedLoad(); ticks != 0 {
		return ticks
	}
	return prototypeFallbackTicks()
}

func prototypeASMTicks() uint64 {
	return prototypeASMTicksIf(prototypeSupported)
}

func prototypeASMTicksIf(supported bool) uint64 {
	if supported {
		return prototypeApproxLoad()
	}
	return prototypeFallbackTicks()
}

func prototypeAtomicTicks() uint64 {
	return prototypeAtomicTicksIf(prototypeSupported)
}

func prototypeAtomicTicksIf(supported bool) uint64 {
	if supported {
		// Atomic prevents compiler caching/hoisting and gives an aligned load.
		// The pointer is returned by assembly, avoiding integer-to-pointer
		// conversions in Go and their checkptr/GC ambiguity.
		return atomic.LoadUint64(prototypeApproxPointer)
	}
	return prototypeFallbackTicks()
}

// Keep the uncommon bridge path out of the atomic reader's inlining budget.
//
//go:noinline
func prototypeFallbackTicks() uint64 { return readLibcTicks() }

func prototypeCheckedASMInstant() Instant { return Instant{ticks: prototypeCheckedTicks()} }
func prototypeCheckedASMSince(start Instant) time.Duration {
	return prototypeCheckedASMInstant().Sub(start)
}
func prototypeCheckedASMUnixNano() int64 {
	correction := wallCorrection.Load()
	return correctedUnixNano(prototypeCheckedASMInstant(), correction)
}
func prototypeCheckedASMNow() time.Time { return time.Unix(0, prototypeCheckedASMUnixNano()) }

func prototypeOnceASMInstant() Instant                  { return Instant{ticks: prototypeASMTicks()} }
func prototypeOnceASMSince(start Instant) time.Duration { return prototypeOnceASMInstant().Sub(start) }
func prototypeOnceASMUnixNano() int64 {
	correction := wallCorrection.Load()
	return correctedUnixNano(prototypeOnceASMInstant(), correction)
}
func prototypeOnceASMNow() time.Time { return time.Unix(0, prototypeOnceASMUnixNano()) }

func prototypeOnceAtomicInstant() Instant { return Instant{ticks: prototypeAtomicTicks()} }
func prototypeOnceAtomicSince(start Instant) time.Duration {
	return prototypeOnceAtomicInstant().Sub(start)
}
func prototypeOnceAtomicUnixNano() int64 {
	correction := wallCorrection.Load()
	return correctedUnixNano(prototypeOnceAtomicInstant(), correction)
}
func prototypeOnceAtomicNow() time.Time { return time.Unix(0, prototypeOnceAtomicUnixNano()) }

// Explicit libc controls remain libc controls after production adopts commpage.
func prototypeLibcInstant() Instant                  { return Instant{ticks: readLibcTicks()} }
func prototypeLibcSince(start Instant) time.Duration { return prototypeLibcInstant().Sub(start) }
func prototypeLibcUnixNano() int64 {
	correction := wallCorrection.Load()
	return correctedUnixNano(prototypeLibcInstant(), correction)
}
func prototypeLibcNow() time.Time { return time.Unix(0, prototypeLibcUnixNano()) }
