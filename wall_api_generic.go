//go:build !(darwin && amd64) && !(linux && amd64)

package coarsetime

import "time"

// Now returns an approximate local wall-clock time. It reads the coarse
// clock: Linux reads CLOCK_REALTIME_COARSE directly; other platforms apply a
// cached correction without reading the system wall clock.
// The result has no Go monotonic component and can move backward.
// See RefreshWallClock for the accuracy and suspend tradeoffs.
func Now() time.Time { return time.Unix(0, UnixNano()) }

// UnixNano returns the approximate current Unix timestamp in nanoseconds. It
// avoids constructing a time.Time and uses the same source as Now.
// Nanosecond units do not imply nanosecond
// accuracy. Like time.Time.UnixNano, the result is undefined outside the range
// representable by int64 Unix nanoseconds (roughly years 1678 through 2262).
func UnixNano() int64 { return readWallUnixNano() }
