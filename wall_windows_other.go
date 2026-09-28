//go:build !purego && windows && !amd64 && !arm64

package coarsetime

import "time"

func readWallUnixNano() int64 { return time.Now().UnixNano() }
