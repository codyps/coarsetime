//go:build wallsplitprototype

package coarsetime

import (
	"sync/atomic"
	"time"
)

// Experimental amd64-only mapping: Mach ticks are nanoseconds on this target.
// Publish immutable snapshots, so readers cannot mix fields across refreshes.
// This does not replace or change RefreshWallClock. Experiments explicitly
// publish its resulting correction with refreshSplitPrototype afterwards.
type splitPrototypeBase struct {
	start      uint64 // Mach tick corresponding to start of this calendar second
	sec        int64
	correction int64
	limit      uint64    // normally 1e9; clipped at the int64 UnixNano upper boundary
	second     time.Time // comparison variant using the public Time.Add method
}

var splitPrototypeMapping atomic.Pointer[splitPrototypeBase]

func makeSplitPrototypeBase(ticks uint64, correction int64) *splitPrototypeBase {
	ns := int64(ticks + uint64(correction))
	sec, frac := ns/1e9, ns%1e9
	if frac < 0 {
		sec--
		frac += 1e9
	}
	limit := uint64(1e9)
	// Match current UnixNano wraparound if this experiment reaches its limit.
	if sec == int64(9223372036) {
		limit = 854775808
	}
	return &splitPrototypeBase{
		start: ticks - uint64(frac), sec: sec, correction: correction,
		limit: limit, second: time.Unix(sec, 0),
	}
}

func refreshSplitPrototype() {
	correction := wallCorrection.Load()
	splitPrototypeMapping.Store(makeSplitPrototypeBase(readTicks(), correction))
}

func splitPrototypeNow() time.Time {
	base := splitPrototypeMapping.Load()
	ticks := readTicks()
	delta := ticks - base.start
	if delta < base.limit {
		return time.Unix(base.sec, int64(delta))
	}
	return splitPrototypeFallback(base, ticks)
}

func splitPrototypeRebaseNow() time.Time {
	base := splitPrototypeMapping.Load()
	ticks := readTicks()
	delta := ticks - base.start
	if delta < base.limit {
		return time.Unix(base.sec, int64(delta))
	}
	return splitPrototypeRebase(base, ticks)
}

func splitPrototypeAddNow() time.Time {
	base := splitPrototypeMapping.Load()
	ticks := readTicks()
	delta := ticks - base.start
	if delta < base.limit {
		return base.second.Add(time.Duration(delta))
	}
	return splitPrototypeFallback(base, ticks)
}

//go:noinline
func splitPrototypeFallback(base *splitPrototypeBase, ticks uint64) time.Time {
	return time.Unix(0, int64(ticks+uint64(base.correction)))
}

// Rebase within the SAME wall correction; this does not resynchronize wall time.
// CAS prevents an old slow reader from overwriting a concurrently refreshed
// mapping. A losing reader can still return its own consistent snapshot, just
// as a current reader may finish using a correction loaded before a refresh.
//
//go:noinline
func splitPrototypeRebase(base *splitPrototypeBase, ticks uint64) time.Time {
	next := makeSplitPrototypeBase(ticks, base.correction)
	splitPrototypeMapping.CompareAndSwap(base, next)
	return time.Unix(next.sec, int64(ticks-next.start))
}
