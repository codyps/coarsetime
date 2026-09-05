//go:build !linux && !windows

package coarsetime

func readWallUnixNano() int64 {
	// Acquire the mapping before sampling the clock.
	correction := wallCorrection.Load()
	return correctedUnixNano(NowInstant(), correction)
}
