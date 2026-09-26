package coarsetime

import "time"

// Now returns approximate local wall time, without a Go monotonic component.
// Readings may repeat or move backward; use NowInstant and Since for elapsed time.
// Linux, Windows, and all purego builds read the system wall clock. Other builds
// apply a cached correction; see RefreshWallClock for refresh requirements.
// No maximum staleness or accuracy is promised.
func Now() time.Time { return readWallTime() }

// UnixNano returns approximate current wall time as Unix nanoseconds, using the
// same clock source as Now without constructing a time.Time. Nanosecond units do
// not imply nanosecond accuracy. Like time.Time.UnixNano, results are undefined
// outside the int64 Unix-nanosecond range (roughly years 1678 through 2262).
func UnixNano() int64 { return readWallUnixNano() }

// Time translates i into approximate local wall time using the current cached
// correction on every platform. The result has no monotonic component and the
// same supported date range as UnixNano. RefreshWallClock can change the result
// for the same Instant; retain the time.Time if a stable timestamp is needed.
// The zero Instant is not a valid input.
func (i Instant) Time() time.Time { return instantTime(i) }
