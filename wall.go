package coarsetime

import (
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// One atomic correction keeps readers lock-free and prevents them from seeing
// a partially refreshed mapping. Only calibration takes the mutex.
var wallCorrection atomic.Int64
var wallCalibrationMu sync.Mutex

func init() { RefreshWallClock() }

// Now returns an approximate local wall-clock time. It reads the coarse
// clock: Linux reads CLOCK_REALTIME_COARSE directly; other platforms apply a
// cached correction without reading the system wall clock.
// The result has no Go monotonic component and can move backward.
// See RefreshWallClock for the accuracy and suspend tradeoffs.
func Now() time.Time { return time.Unix(0, UnixNano()) }

// UnixNano returns the approximate current Unix timestamp in nanoseconds. It
// avoids constructing a time.Time and uses the same source as Now.
// Nanosecond units do not imply nanosecond
// accuracy. Like time.Time.UnixNano, the result is undefined outside the range
// representable by int64 Unix nanoseconds (roughly years 1678 through 2262).
func UnixNano() int64 { return readWallUnixNano() }

// Time translates i into approximate local wall time using the current cached
// correction. Its supported date range is the same as UnixNano. It has no
// monotonic component. RefreshWallClock can change the
// result for the same Instant, so convert and retain the time.Time if a stable
// historical timestamp is needed. The zero Instant is not a valid input.
func (i Instant) Time() time.Time {
	return time.Unix(0, correctedUnixNano(i, wallCorrection.Load()))
}

func correctedUnixNano(i Instant, correction int64) int64 {
	// Use unsigned arithmetic for the native epoch, then wrap into UnixNano's
	// signed representation. The correction can itself wrap for a distant epoch;
	// only the resulting wall timestamp needs to fit int64.
	ns := scaleTicks(i.ticks, clockNumer, clockDenom, math.MaxUint64)
	return int64(ns + uint64(correction))
}

// RefreshWallClock recalibrates the mapping from coarse readings to wall time.
// Calibration runs once at package initialization. Linux Now and UnixNano
// use the kernel wall clock directly and do not need this correction; on Linux
// it is used only by Instant.Time. There is no automatic updater:
// call this periodically or after resume/clock changes if freshness matters.
//
// Between refreshes the mapping ignores wall-clock steps and accumulated drift.
// Clocks that pause during suspend (including Darwin and Linux) leave wall reads
// behind by the suspended duration until refreshed. Coarse-clock staleness and
// calibration sampling add error; no maximum error is promised. A refresh can
// make subsequent wall readings jump in either direction. Instant comparisons
// and duration measurements are unaffected.
//
// Concurrent refreshes are serialized; wall-clock readers never take that lock.
func RefreshWallClock() {
	wallCalibrationMu.Lock()
	defer wallCalibrationMu.Unlock()

	bestSpan := time.Duration(math.MaxInt64)
	var correction int64
	// Prefer the narrowest bracket to reduce scheduler interruption error. Use
	// precise monotonic elapsed time for the bracket, not the coarse clock (which
	// can remain unchanged throughout an interrupted sample).
	for n := 0; n < 8; n++ {
		before := time.Now()
		instant := NowInstant()
		after := time.Now()
		span := after.Sub(before)
		if span >= 0 && span < bestSpan {
			bestSpan = span
			wall := before.Add(span / 2).UnixNano()
			ns := scaleTicks(instant.ticks, clockNumer, clockDenom, math.MaxUint64)
			correction = int64(uint64(wall) - ns)
		}
	}
	wallCorrection.Store(correction)
}
