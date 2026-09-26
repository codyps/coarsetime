//go:build linuxwallprobe

package coarsetime

import (
	"sync/atomic"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func wallProbeNoopAddress() unsafe.Pointer

var wallProbeNoop = wallProbeNoopAddress()

func wallProbeBridgeNow() time.Time {
	args := struct {
		fn uintptr
		ts syscall.Timespec
	}{}
	if asmcgocall(wallProbeNoop, unsafe.Pointer(&args)) != 0 {
		panic("noop failed")
	}
	return time.Unix(args.ts.Sec, args.ts.Nsec)
}

// Put normalization inside the C-ABI trampoline? This simpler candidate only
// narrows the known nonnegative nsec field, keeping time.Time construction safe.
func wallProbeNarrowNow() time.Time {
	fn := coarseVDSO
	if fn != 0 {
		args := struct {
			fn uintptr
			ts syscall.Timespec
		}{fn: fn}
		if asmcgocall(realtimeTrampoline, unsafe.Pointer(&args)) == 0 {
			return time.Unix(args.ts.Sec, int64(uint32(args.ts.Nsec)))
		}
	}
	return time.Unix(0, readRealtimeSyscall())
}

func BenchmarkLinuxWallBridgeOnly(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		timeSink = wallProbeBridgeNow()
	}
}
func BenchmarkLinuxWallNarrowNow(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		timeSink = wallProbeNarrowNow()
	}
}
func BenchmarkLinuxWallCurrentNow(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		timeSink = Now()
	}
}

// Explicitly different semantics: a timer publishes immutable snapshots.
// This measures read cost with a 1 ms updater active, excluding its start/stop.
func BenchmarkLinuxWallCachedNow(b *testing.B) {
	var cached atomic.Pointer[time.Time]
	publish := func() { now := Now(); cached.Store(&now) }
	publish()
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				publish()
			case <-stop:
				return
			}
		}
	}()
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		timeSink = *cached.Load()
	}
	b.StopTimer()
	close(stop)
	<-done
}
func TestLinuxWallProbe(t *testing.T) {
	if coarseVDSO == 0 {
		t.Fatal("probe requires vDSO")
	}
	if wallProbeBridgeNow() != time.Unix(1700000000, 123456789) {
		t.Fatal("bridge output")
	}
	for n := 0; n < 10000; n++ {
		before := readRealtimeSyscall()
		got := wallProbeNarrowNow()
		after := readRealtimeSyscall()
		if after >= before && (got.Before(time.Unix(0, before)) || got.After(time.Unix(0, after))) {
			t.Fatal("narrow outside bracket")
		}
	}
}
