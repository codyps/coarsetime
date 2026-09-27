// Package darwinwall interprets XNU's kernel-published calendar mapping.
// It contains no OS access: Darwin callers supply commpage pointers, and tests
// supply ordinary Go memory. No local calibration or periodic updater is used.
package darwinwall

import (
	"math"
	"math/bits"
	"sync/atomic"
)

// Page matches XNU's new_commpage_timeofday_data_t: five aligned uint64 fields.
// The kernel invalidates Tick, writes the payload, then publishes a new Tick.
// Do not reorder fields. The kernel, not this package, owns production writes.
type Page struct {
	Tick        atomic.Uint64
	Seconds     atomic.Uint64
	Fraction    atomic.Uint64
	Scale       atomic.Uint64
	TicksPerSec atomic.Uint64
}

// Read makes one attempt to translate the shared approximate Mach tick through
// the current calendar mapping. Invalid, changing, expired, or unrepresentable
// data returns false; the caller must read the OS wall clock instead. There is
// no retry loop. Atomic loads preserve reader ordering, including on ARM64.
func Read(page *Page, approximate *atomic.Uint64) (int64, bool) {
	anchor := page.Tick.Load()
	if anchor == 0 {
		return 0, false
	}
	now := approximate.Load()
	s := snapshot{
		tick:        anchor,
		seconds:     page.Seconds.Load(),
		fraction:    page.Fraction.Load(),
		scale:       page.Scale.Load(),
		ticksPerSec: page.TicksPerSec.Load(),
	}
	return s.unixNano(now, page.Tick.Load())
}

type snapshot struct {
	tick, seconds, fraction, scale, ticksPerSec uint64
}

func (s snapshot) unixNano(now, confirmation uint64) (int64, bool) {
	// Approximate time and calendar anchors are published independently. Never
	// extrapolate backwards from a new anchor, or beyond XNU's one-second window.
	if s.tick == 0 || s.tick != confirmation || now < s.tick || now-s.tick >= s.ticksPerSec || s.scale == 0 {
		return 0, false
	}
	whole, fraction := bits.Mul64(now-s.tick, s.scale)
	fraction, carry := bits.Add64(fraction, s.fraction, 0)
	seconds, overflow := bits.Add64(s.seconds, whole, carry)
	if overflow != 0 || seconds > math.MaxInt64/1_000_000_000 {
		return 0, false
	}
	// Convert the binary fraction directly to nanoseconds. These units do not
	// imply nanosecond accuracy; the input sample is deliberately approximate.
	nanos, _ := bits.Mul64(fraction, 1_000_000_000)
	result := seconds*1_000_000_000 + nanos
	if result > math.MaxInt64 {
		return 0, false
	}
	return int64(result), true
}
