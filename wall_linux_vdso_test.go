//go:build !purego && linux && (amd64 || arm64)

package coarsetime

import "testing"

func TestLinuxWallGoFallback(t *testing.T) {
	saved := coarseVDSO
	defer func() { coarseVDSO = saved }()
	coarseVDSO = 0
	testLinuxGoWallClock(t)
}
