//go:build !(darwin && amd64)

package coarsetime

import "time"

func instantTime(i Instant) time.Time {
	return time.Unix(0, correctedUnixNano(i, wallCorrection.Load()))
}
