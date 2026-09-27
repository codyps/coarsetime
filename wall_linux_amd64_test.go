//go:build !purego

package coarsetime

import "testing"

func TestLinuxWallGoFallback(t *testing.T) {
	saved := coarseVDSO
	defer func() { coarseVDSO = saved }()
	coarseVDSO = 0
	testLinuxGoWallClock(t)
}
