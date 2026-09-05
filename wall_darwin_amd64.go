package coarsetime

import (
	"sync/atomic"
	"time"
)

// Evaluate the timebase once; retain generic scaling if a future kernel uses
// different units. Keeping one fast-path branch lets UnixNano inline.
var wallUnitTimebase = clockNumer == clockDenom
var wallCommpageSupported = approximateTimeSupported && wallUnitTimebase

// UnixNano returns the approximate current Unix timestamp in nanoseconds. It
// avoids constructing a time.Time and uses the same source as Now.
// Nanosecond units do not imply nanosecond
// accuracy. Like time.Time.UnixNano, the result is undefined outside the range
// representable by int64 Unix nanoseconds (roughly years 1678 through 2262).
func UnixNano() int64 {
	if wallCommpageSupported {
		// Acquire the mapping before sampling the clock.
		correction := wallCorrection.Load()
		return int64(atomic.LoadUint64(approximateTimePointer)) + correction
	}
	return wallUnixNanoFallback()
}

//go:noinline
func wallUnixNanoFallback() int64 {
	correction := wallCorrection.Load()
	return correctedUnixNano(NowInstant(), correction)
}

// Now returns an approximate local wall-clock time. It reads the coarse
// clock: Linux reads CLOCK_REALTIME_COARSE directly; other platforms apply a
// cached correction without reading the system wall clock.
// The result has no Go monotonic component and can move backward.
// See RefreshWallClock for the accuracy and suspend tradeoffs.
func Now() time.Time {
	ns := UnixNano()
	if ns < 0 {
		return time.Unix(0, ns)
	}
	// Nonnegative division needs less normalization; keep negative epochs exact.
	u := uint64(ns)
	return time.Unix(int64(u/1e9), int64(u%1e9))
}

// Time translates i into approximate local wall time using the current cached
// correction. Its supported date range is the same as UnixNano. It has no
// monotonic component. RefreshWallClock can change the
// result for the same Instant, so convert and retain the time.Time if a stable
// historical timestamp is needed. The zero Instant is not a valid input.
func (i Instant) Time() time.Time {
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
