//go:build !windows || !amd64 || purego

package coarsetime

import "time"

func unixNano() int64 { return time.Now().UnixNano() }
