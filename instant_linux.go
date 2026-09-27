//go:build !purego

package coarsetime

import (
	"syscall"
	"time"
)

const clockMonotonicCoarse = 6
const clockNumer, clockDenom = uint64(1), uint64(1)

var linuxOrigin = time.Now()
var linuxCoarseClock = hasCoarseClock()

func hasCoarseClock() bool {
	var ts syscall.Timespec
	return readCoarseVDSO(&ts)
}

func readTicks() uint64 {
	// Choose one epoch at startup. Switching between a boot-relative coarse
	// clock and a process-relative Go clock would invalidate existing Instants.
	if !linuxCoarseClock {
		return uint64(time.Since(linuxOrigin))
	}
	var ts syscall.Timespec
	if !readCoarseVDSO(&ts) {
		// A clock that worked at startup must remain available. Do not silently
		// change epochs after callers have obtained Instants.
		panic("coarsetime: CLOCK_MONOTONIC_COARSE vDSO failed after initialization")
	}
	return uint64(ts.Sec)*1_000_000_000 + uint64(ts.Nsec)
}
