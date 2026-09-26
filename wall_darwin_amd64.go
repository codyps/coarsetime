//go:build !purego

package coarsetime

import (
	"sync/atomic"
	"time"
)

// Evaluate the timebase once; retain generic scaling if a future kernel uses
// different units. Keeping one fast-path branch lets UnixNano inline.
var wallUnitTimebase = clockNumer == clockDenom
var wallCommpageSupported = approximateTimeSupported && wallUnitTimebase

func readWallUnixNano() int64 {
	if wallCommpageSupported {
		// Acquire the mapping before sampling the clock.
		return wallCorrection.Load() + int64(atomic.LoadUint64(approximateTimePointer))
	}
	return wallUnixNanoFallback()
}

//go:noinline
func wallUnixNanoFallback() int64 {
	correction := wallCorrection.Load()
	return correctedUnixNano(NowInstant(), correction)
}

func readWallTime() time.Time {
	ns := readWallUnixNano()
	if ns < 0 {
		return time.Unix(0, ns)
	}
	// Nonnegative division needs less normalization; keep negative epochs exact.
	u := uint64(ns)
	return time.Unix(int64(u/1e9), int64(u%1e9))
}

func instantTime(i Instant) time.Time {
	correction := wallCorrection.Load()
	var ns int64
	if wallUnitTimebase {
		ns = int64(i.ticks) + correction
	} else {
		ns = correctedUnixNano(i, correction)
	}
	if ns < 0 {
		return time.Unix(0, ns)
	}
	u := uint64(ns)
	return time.Unix(int64(u/1e9), int64(u%1e9))
}
