//go:build walloptprototype

package coarsetime

import (
	"sync/atomic"
	"time"
)

// Intel Mach absolute/approximate ticks are nanoseconds. These candidates
// remove generic timebase conversion but retain the actual production reader,
// its support check and libc fallback, and the existing atomic correction.
func wallOptUnitUnixNano() int64 {
	correction := wallCorrection.Load()
	return int64(readTicks() + uint64(correction))
}

func wallOptUnitNow() time.Time         { return time.Unix(0, wallOptUnitUnixNano()) }
func wallOptUnitUnsignedNow() time.Time { return wallOptUnsignedTime(wallOptUnitUnixNano()) }

// Flattening tests whether avoiding helper boundaries gives the compiler a
// better result without changing the arithmetic or private OS dependencies.
func wallOptFlatNow() time.Time {
	correction := wallCorrection.Load()
	ns := int64(readTicks() + uint64(correction))
	return time.Unix(0, ns)
}

func wallOptFlatUnsignedNow() time.Time {
	correction := wallCorrection.Load()
	ns := int64(readTicks() + uint64(correction))
	if ns < 0 {
		return time.Unix(0, ns)
	}
	u := uint64(ns)
	return time.Unix(int64(u/1e9), int64(u%1e9))
}

func wallOptUnitInstantTime(i Instant) time.Time {
	return wallOptUnsignedTime(int64(i.ticks + uint64(wallCorrection.Load())))
}

// Fuse clock and correction reads so the UnixNano function fits Go's inlining
// budget. Keep the fallback out of line, just as the adopted tick reader does.
func wallOptInlineUnixNano() int64 {
	correction := wallCorrection.Load()
	var ticks uint64
	if approximateTimeSupported {
		ticks = atomic.LoadUint64(approximateTimePointer)
	} else {
		ticks = readTicksFallback()
	}
	return int64(ticks) + correction
}

func wallOptInlineNow() time.Time { return time.Unix(0, wallOptInlineUnixNano()) }

// Move the entire unsupported branch, including correction acquisition, out
// of line. Each branch still acquires correction BEFORE its timestamp sample.
func wallOptColdUnixNano() int64 {
	if approximateTimeSupported {
		correction := wallCorrection.Load()
		return int64(atomic.LoadUint64(approximateTimePointer)) + correction
	}
	return wallOptColdFallback()
}

//go:noinline
func wallOptColdFallback() int64 {
	correction := wallCorrection.Load()
	return int64(readTicksFallback()) + correction
}

func wallOptColdNow() time.Time { return time.Unix(0, wallOptColdUnixNano()) }
func wallOptColdUnsignedNow() time.Time {
	ns := wallOptColdUnixNano()
	if ns < 0 {
		return time.Unix(0, ns)
	}
	u := uint64(ns)
	return time.Unix(int64(u/1e9), int64(u%1e9))
}
