//go:build walloptprototype

package coarsetime

import (
	"os"
	"testing"
	"time"
)

func TestWallOptLinux(t *testing.T) {
	if os.Getenv("COARSETIME_REQUIRE_VDSO") == "1" && coarseVDSO == 0 {
		t.Fatal("vDSO unavailable")
	}
	saved := coarseVDSO
	defer func() { coarseVDSO = saved }()
	for _, fallback := range []bool{false, true} {
		if fallback {
			coarseVDSO = 0
		}
		for _, read := range []func() time.Time{wallOptTimespecNow, wallOptFusedNow} {
			for n := 0; n < 10000; n++ {
				before := readRealtimeSyscall()
				got := read()
				after := readRealtimeSyscall()
				if got.Before(time.Unix(0, before)) || got.After(time.Unix(0, after)) {
					t.Fatalf("fallback=%v: %v outside [%d,%d]", fallback, got, before, after)
				}
			}
		}
		for n := 0; n < 10000; n++ {
			before := readRealtimeSyscall()
			got := wallOptFusedUnixNano()
			after := readRealtimeSyscall()
			if got < before || got > after {
				t.Fatal("UnixNano outside syscall bracket")
			}
		}
	}
}

func BenchmarkWallOptTimespecNow(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		timeSink = wallOptTimespecNow()
	}
}

func BenchmarkWallOptFusedNow(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		timeSink = wallOptFusedNow()
	}
}

func BenchmarkWallOptFusedUnixNano(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		unixNanoSink = wallOptFusedUnixNano()
	}
}
