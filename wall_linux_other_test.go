//go:build !purego && linux && !amd64 && !arm64

package coarsetime

import "testing"

func TestLinuxWallGoFallback(t *testing.T) { testLinuxGoWallClock(t) }
