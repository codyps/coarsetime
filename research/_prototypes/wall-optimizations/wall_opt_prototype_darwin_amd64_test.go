//go:build walloptprototype

package coarsetime

import (
	"math"
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestWallOptDarwin(t *testing.T) {
	if clockNumer != 1 || clockDenom != 1 {
		t.Fatal("Intel Mach timebase is not 1:1")
	}
	RefreshWallClock()
	readers := []func() time.Time{wallOptUnitNow, wallOptUnitUnsignedNow, wallOptFlatNow, wallOptFlatUnsignedNow, wallOptInlineNow, wallOptColdNow, wallOptColdUnsignedNow}
	for _, read := range readers {
		for n := 0; n < 10000; n++ {
			before := Now()
			got := read()
			after := Now()
			if got.Before(before) || got.After(after) {
				t.Fatalf("%v outside [%v,%v]", got, before, after)
			}
		}
	}
	rng := rand.New(rand.NewSource(72))
	for n := 0; n < 10000; n++ {
		i := Instant{ticks: rng.Uint64()}
		if got, want := wallOptUnitInstantTime(i), i.Time(); got != want {
			t.Fatalf("Instant.Time mismatch: %v %v", got, want)
		}
	}
	// Overlap reads with refreshes and GC. The prototypes share the production
	// correction, so all returned dates should remain close to system wall time.
	var wg sync.WaitGroup
	for _, read := range readers {
		wg.Add(1)
		go func(read func() time.Time) {
			defer wg.Done()
			for n := 0; n < 20000; n++ {
				got := read()
				if d := time.Since(got); d < -time.Second || d > time.Second {
					t.Errorf("unexpected wall deviation %v", d)
					return
				}
			}
		}(read)
	}
	for n := 0; n < 100; n++ {
		RefreshWallClock()
		if n%20 == 0 {
			runtime.GC()
		}
	}
	wg.Wait()
	// Compare complete representations including location and no monotonic data.
	for _, ns := range []int64{math.MinInt64, -1, 0, math.MaxInt64} {
		i := Instant{ticks: uint64(ns) - uint64(wallCorrection.Load())}
		if got := wallOptUnitInstantTime(i); got != time.Unix(0, ns) {
			t.Fatalf("wrapped epoch mismatch: %v", got)
		}
	}
}

func BenchmarkWallOptUnitUnixNano(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		unixNanoSink = wallOptUnitUnixNano()
	}
}

func BenchmarkWallOptUnitNow(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		timeSink = wallOptUnitNow()
	}
}

func BenchmarkWallOptUnitUnsignedNow(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		timeSink = wallOptUnitUnsignedNow()
	}
}

func BenchmarkWallOptFlatNow(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		timeSink = wallOptFlatNow()
	}
}

func BenchmarkWallOptFlatUnsignedNow(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		timeSink = wallOptFlatUnsignedNow()
	}
}

func BenchmarkWallOptCurrentInstantTime(b *testing.B) {
	i := NowInstant()
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		timeSink = i.Time()
	}
}

func BenchmarkWallOptUnitInstantTime(b *testing.B) {
	i := NowInstant()
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		timeSink = wallOptUnitInstantTime(i)
	}
}

func TestWallOptInlineFallback(t *testing.T) {
	supported, pointer := approximateTimeSupported, approximateTimePointer
	wallSupported := wallCommpageSupported
	defer func() { wallCommpageSupported = wallSupported }()
	wallCommpageSupported = false
	defer func() { approximateTimeSupported = supported; approximateTimePointer = pointer }()
	approximateTimeSupported = false
	approximateTimePointer = nil // a mistaken direct load would fail
	for n := 0; n < 1000; n++ {
		for _, read := range []func() int64{wallOptInlineUnixNano, wallOptColdUnixNano} {
			before := UnixNano()
			got := read()
			after := UnixNano()
			if got < before || got > after {
				t.Fatal("outlined fallback outside libc bracket")
			}
		}
	}
}
func BenchmarkWallOptInlineUnixNano(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		unixNanoSink = wallOptInlineUnixNano()
	}
}
func BenchmarkWallOptInlineNow(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		timeSink = wallOptInlineNow()
	}
}

func BenchmarkWallOptColdUnixNano(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		unixNanoSink = wallOptColdUnixNano()
	}
}

func BenchmarkWallOptColdNow(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		timeSink = wallOptColdNow()
	}
}

func BenchmarkWallOptColdUnsignedNow(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		timeSink = wallOptColdUnsignedNow()
	}
}
