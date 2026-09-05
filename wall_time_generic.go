//go:build !(darwin && amd64)

package coarsetime

import "time"

// Time translates i into approximate local wall time using the current cached
// correction. Its supported date range is the same as UnixNano. It has no
// monotonic component. RefreshWallClock can change the
// result for the same Instant, so convert and retain the time.Time if a stable
// historical timestamp is needed. The zero Instant is not a valid input.
func (i Instant) Time() time.Time { return time.Unix(0, correctedUnixNano(i, wallCorrection.Load())) }
