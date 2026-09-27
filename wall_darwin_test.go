//go:build !purego && darwin && (amd64 || arm64)

package coarsetime

import (
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/codyps/coarsetime/internal/darwinwall"
)

func TestDarwinCalendarSelection(t *testing.T) {
	savedPage, savedTick, savedSupported := darwinCalendarPage, darwinCalendarApproximate, darwinCalendarSupported
	defer func() {
		darwinCalendarPage, darwinCalendarApproximate, darwinCalendarSupported = savedPage, savedTick, savedSupported
	}()
	var p darwinwall.Page
	var tick atomic.Uint64
	darwinCalendarPage, darwinCalendarApproximate, darwinCalendarSupported = &p, &tick, true
	p.Tick.Store(100)
	p.Seconds.Store(1700000000)
	p.Fraction.Store(1 << 63)
	p.Scale.Store(1 << 62)
	p.TicksPerSec.Store(4)
	tick.Store(101)
	const want = 1700000000750000000
	if got := UnixNano(); got != want {
		t.Fatal(got)
	}
	if got := Now(); got != time.Unix(0, want) {
		t.Fatal(got)
	}
	for _, mode := range []string{"unsupported", "invalid", "before-anchor", "expired"} {
		darwinCalendarSupported = mode != "unsupported"
		p.Tick.Store(100)
		tick.Store(101)
		switch mode {
		case "invalid":
			p.Tick.Store(0)
		case "before-anchor":
			tick.Store(99)
		case "expired":
			tick.Store(104)
		}
		before := time.Now().UnixNano()
		got := UnixNano()
		after := time.Now().UnixNano()
		if got < before || got > after {
			t.Fatalf("%s fallback: %d outside [%d,%d]", mode, got, before, after)
		}
	}
}

func TestDarwinCalendarLive(t *testing.T) {
	if !darwinCalendarSupported {
		if os.Getenv("COARSETIME_REQUIRE_DARWIN_WALL") == "1" {
			t.Fatal("approximate commpage clock unsupported")
		}
		t.Skip("approximate commpage clock unsupported")
	}
	hits := 0
	for i := 0; i < 10000; i++ {
		before := time.Now().UnixNano()
		got, ok := darwinwall.Read(darwinCalendarPage, darwinCalendarApproximate)
		after := time.Now().UnixNano()
		if !ok {
			continue
		}
		hits++
		// Approximate ticks can lag; 1 s is a gross sanity check, not a contract.
		// Full binary-fraction conversion can exceed libc's rounded value by <1 us.
		if got < before-int64(time.Second) || got > after+int64(time.Microsecond) {
			t.Fatalf("calendar reading %d outside current wall range [%d,%d]", got, before, after)
		}
	}
	t.Logf("accepted %d/10000 shared-page snapshots", hits)
	if hits == 0 && os.Getenv("COARSETIME_REQUIRE_DARWIN_WALL") == "1" {
		t.Fatal("no usable calendar snapshot")
	}
}

func BenchmarkDarwinCalendarRead(b *testing.B) {
	if !darwinCalendarSupported {
		b.Skip("approximate commpage clock unsupported")
	}
	misses := 0
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ns, ok := darwinwall.Read(darwinCalendarPage, darwinCalendarApproximate)
		if !ok {
			misses++
			ns = readDarwinWallFallback()
		}
		unixNanoSink = ns
	}
	b.ReportMetric(float64(misses)*100/float64(b.N), "fallback-%")
}

func BenchmarkDarwinStandardUnixNano(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		unixNanoSink = time.Now().UnixNano()
	}
}
