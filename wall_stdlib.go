//go:build purego || (!linux && !windows && (!darwin || (!amd64 && !arm64)))

package coarsetime

import "time"

// Wall reads follow the standard-library clock. readWallTime strips monotonic data
// by constructing a time.Time from this Unix timestamp on every platform.
func readWallUnixNano() int64 { return time.Now().UnixNano() }
