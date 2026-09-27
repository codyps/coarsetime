//go:build !purego && linux && !amd64 && !arm64

package coarsetime

import "syscall"

// Other architectures retain the coarse clock semantics via a kernel syscall.
func readCoarseVDSO(ts *syscall.Timespec) bool { return false }

func readRealtimeVDSO(ts *syscall.Timespec) bool { return false }
