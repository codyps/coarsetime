// Package coarsetime provides inexpensive functions for measuring elapsed time
// and obtaining approximate wall time. Readings may repeat and have
// platform-dependent resolution and staleness; no maximum error is promised.
// Instant readings are not calendar timestamps; Now reads approximate OS-maintained wall time
// independently. No calibration, polling goroutine, or manual refresh is needed.
//
// Default Darwin builds use the Mach approximate clock; Linux uses
// CLOCK_MONOTONIC_COARSE. Both exclude system sleep. Windows/amd64 reads shared
// interrupt time, which includes system sleep. Other platforms use Go's monotonic
// clock, whose sleep behavior depends on the platform.
// With the purego build tag, all platforms use Go's monotonic clock for Instants
// and time.Now for wall readings, without this package's native clock access.
// Instants are meaningful only within the process that obtained them.
package coarsetime
