package coarsetime

import "time"

// Now returns approximate local wall time, without a Go monotonic component.
// Readings may repeat or move backward; use NowInstant and Since for elapsed time.
// Readings use OS-maintained wall time; no application refresh is required.
// No maximum staleness or accuracy is promised.
func Now() time.Time { return readWallTime() }

// UnixNano returns approximate current wall time as Unix nanoseconds, using the
// same clock source as Now without constructing a time.Time. Nanosecond units do
// not imply nanosecond accuracy. Like time.Time.UnixNano, results are undefined
// outside the int64 Unix-nanosecond range (roughly years 1678 through 2262).
func UnixNano() int64 { return readWallUnixNano() }
