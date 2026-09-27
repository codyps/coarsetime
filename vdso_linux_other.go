//go:build !purego && linux && !amd64 && !arm64

package coarsetime

import "syscall"

// Other architectures use Go clocks because our vDSO bridge is unavailable.
func readCoarseVDSO(ts *syscall.Timespec) bool { return false }

func readRealtimeVDSO(ts *syscall.Timespec) bool { return false }
