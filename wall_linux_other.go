//go:build !purego && linux && !amd64

package coarsetime

import "time"

// Let Go use its optimized clock reader on architectures without our vDSO bridge.
func readWallUnixNano() int64 {
	return time.Now().UnixNano()
}
