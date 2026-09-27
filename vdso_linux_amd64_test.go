//go:build !purego && linux && amd64

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
		t.Skip("vDSO unavailable; Go fallback is active")
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

func TestLinuxGoFallback(t *testing.T) {
	// Tests are deliberately serial: change the initialization-time selection only
	// while no clock readers are running, and restore it before subsequent tests.
	saved := coarseVDSO
	runtimeSaved := runtimeVDSOClockgettime
	coarseVDSO = 0
	defer func() { coarseVDSO = saved }()
	if hasCoarseClock() {
		t.Fatal("missing vDSO must select Go's monotonic clock at startup")
	}
	TestLinuxGoMonotonicClock(t)
	testLinuxGoWallClock(t)
	if runtimeVDSOClockgettime != runtimeSaved {
		t.Fatal("forcing the package fallback changed the runtime's vDSO address")
	}
}

func TestLinuxCoarseClockFailure(t *testing.T) {
	if !linuxCoarseClock {
		t.Skip("coarse clock unavailable")
	}
	saved := coarseVDSO
	coarseVDSO = 0
	defer func() { coarseVDSO = saved }()
	defer func() {
		if recover() == nil {
			t.Fatal("failed coarse read must not silently switch epochs")
		}
	}()
	NowInstant()
}
