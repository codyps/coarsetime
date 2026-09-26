//go:build !purego && windows && !amd64

package coarsetime

import "time"

func unixNano() int64 { return time.Now().UnixNano() }
