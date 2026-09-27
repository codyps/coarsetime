//go:build purego || !(linux && amd64)

package coarsetime

import "time"

func readWallTime() time.Time { return time.Unix(0, readWallUnixNano()) }
