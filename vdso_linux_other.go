//go:build !purego && linux && !amd64

package coarsetime

import "syscall"

// Other architectures use Go clocks because our vDSO bridge is unavailable.
// In particular, arm64 vDSO calls need additional runtime signal-stack handling;
// asmcgocall alone is not sufficient when cgo is disabled.
func readCoarseVDSO(ts *syscall.Timespec) bool { return false }

func readRealtimeVDSO(ts *syscall.Timespec) bool { return false }
