//go:build purego

package coarsetime

import "time"

// Wall reads follow the standard-library clock independently of the cached
// correction used to translate Instants. readWallTime strips monotonic data
// by constructing a time.Time from this Unix timestamp on every platform.
func readWallUnixNano() int64 { return time.Now().UnixNano() }
