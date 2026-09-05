//go:build !linux && (!darwin || (!amd64 && !arm64))

package coarsetime

import "time"

var origin = time.Now()

const clockNumer, clockDenom = uint64(1), uint64(1)

func readTicks() uint64 { return uint64(time.Since(origin)) }
