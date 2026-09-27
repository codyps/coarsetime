//go:build !purego && linux && (amd64 || arm64)

package coarsetime

import (
	"os"
	"syscall"
	"testing"
)

func TestVDSO(t *testing.T) {
	if coarseVDSO == 0 {
		if os.Getenv("COARSETIME_REQUIRE_VDSO") == "1" {
			t.Fatal("vDSO fast path unavailable")
		}
		t.Skip("vDSO unavailable; syscall fallback is active")
	}
	var ts syscall.Timespec
	if !readCoarseVDSO(&ts) {
		t.Fatal("resolved vDSO does not support the coarse clock")
	}
	if ts.Sec < 0 || ts.Nsec < 0 || ts.Nsec >= 1_000_000_000 {
		t.Fatalf("invalid timespec: %+v", ts)
	}
	if !readRealtimeVDSO(&ts) {
		t.Fatal("realtime vDSO unavailable")
	}
	t.Log("monotonic and realtime vDSO fast paths active")
}

func TestLinuxSyscallFallback(t *testing.T) {
	// Tests are deliberately serial: change the initialization-time selection only
	// while no clock readers are running, and restore it before subsequent tests.
	saved := coarseVDSO
	runtimeSaved := runtimeVDSOClockgettime
	before := NowInstant()
	coarseVDSO = 0
	defer func() { coarseVDSO = saved }()
	TestLinuxCoarseClock(t)
	TestLinuxRealtimeClock(t)
	if NowInstant().Before(before) {
		t.Fatal("fallback changed the clock epoch")
	}
	if runtimeVDSOClockgettime != runtimeSaved {
		t.Fatal("forcing the package fallback changed the runtime's vDSO address")
	}
}
