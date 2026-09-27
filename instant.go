package coarsetime

import (
	"math"
	"math/bits"
	"time"
)

// Instant is an opaque clock reading. Obtain it with NowInstant; its zero value is not
// an initialized reading. Instant is comparable and safe to share between
// goroutines using normal synchronization. Repeated readings may compare equal.
type Instant struct{ ticks uint64 }

// NowInstant reads the platform clock without converting its native ticks.
func NowInstant() Instant { return Instant{ticks: readTicks()} }

// Before reports whether i precedes j.
func (i Instant) Before(j Instant) bool { return i.ticks < j.ticks }

// After reports whether i follows j.
func (i Instant) After(j Instant) bool { return i.ticks > j.ticks }

// Sub returns i-j, truncating fractional nanoseconds toward zero. Results outside
// time.Duration's range are saturated to its minimum or maximum value.
func (i Instant) Sub(j Instant) time.Duration {
	if i.ticks >= j.ticks {
		return time.Duration(scaleTicks(i.ticks-j.ticks, clockNumer, clockDenom, math.MaxInt64))
	}
	n := scaleTicks(j.ticks-i.ticks, clockNumer, clockDenom, uint64(1)<<63)
	return time.Duration(-n)
}

// Since returns the elapsed time since i, equivalent to NowInstant().Sub(i).
func Since(i Instant) time.Duration { return NowInstant().Sub(i) }

// scaleTicks avoids overflowing the intermediate product for long uptimes and
// non-unit Mach timebases. denom must be nonzero.
func scaleTicks(ticks, numer, denom, limit uint64) uint64 {
	if numer == denom {
		if ticks > limit {
			return limit
		}
		return ticks
	}
	hi, lo := bits.Mul64(ticks, numer)
	if hi >= denom {
		return limit
	}
	n, _ := bits.Div64(hi, lo, denom)
	if n > limit {
		return limit
	}
	return n
}
