//go:build linux && amd64

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
	before := NowInstant()
	coarseVDSO = 0
	defer func() { coarseVDSO = saved }()
	TestLinuxCoarseClock(t)
	TestLinuxRealtimeClock(t)
	if NowInstant().Before(before) {
		t.Fatal("fallback changed the clock epoch")
	}
}

func TestVDSOMapping(t *testing.T) {
	tests := []struct {
		maps       string
		base, size uint64
	}{
		{"7ffe0000-7ffe2000 r-xp 00000000 00:00 0 [vdso]\n", 0x7ffe0000, 0x2000},
		{"7ffe0000-7ffe2000 r--p 00000000 00:00 0 [vdso]\n", 0, 0},
		{"7ffe0000-7ffe2000 r-xp 00000000 00:00 0 [vvar]\n", 0, 0},
		{"broken r-xp 00000000 00:00 0 [vdso]\n", 0, 0},
		{"7ffe2000-7ffe0000 r-xp 00000000 00:00 0 [vdso]\n", 0, 0},
		{"0-200000 r-xp 00000000 00:00 0 [vdso]\n", 0, 0},
	}
	for _, tt := range tests {
		base, size := vdsoMapping(tt.maps)
		if base != tt.base || size != tt.size {
			t.Errorf("vdsoMapping(%q) = %#x,%#x", tt.maps, base, size)
		}
	}
}
