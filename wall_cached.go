//go:build !linux && !windows && !(darwin && amd64)


package coarsetime

func readWallUnixNano() int64 {
	// Acquire the mapping before sampling the clock.
	correction := wallCorrection.Load()
	return correctedUnixNano(NowInstant(), correction)
}
