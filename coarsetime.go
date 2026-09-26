// Package coarsetime provides current wall-clock timestamps with a low-overhead
// coarse fast path on Windows/amd64.
//
// Nanosecond units do not imply nanosecond resolution. Wall time may jump
// backwards or forwards after a clock adjustment. Use time.Since with a value
// returned by Now for elapsed time; do not subtract UnixNano timestamps.
package coarsetime

import "time"

// Now returns the current local time, including Go's monotonic clock reading.
// It is equivalent to time.Now.
func Now() time.Time { return time.Now() }

// UnixNano returns the current wall time as nanoseconds since the Unix epoch.
// On Windows/amd64 it reads the OS's coarse shared clock directly. Elsewhere,
// or with the purego build tag, it uses time.Now().UnixNano().
//
// Consecutive calls may return the same value, and clock adjustments may cause
// the value to decrease. Resolution depends on the OS and its timer settings;
// no maximum staleness is guaranteed. Like time.Time.UnixNano, the result is
// undefined outside the range representable by signed 64-bit Unix nanoseconds
// (approximately years 1678 through 2262).
func UnixNano() int64 { return unixNano() }
