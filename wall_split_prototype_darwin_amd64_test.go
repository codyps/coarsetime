//go:build wallsplitprototype

package coarsetime

import (
	"math"
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"
)

func splitPrototypeAt(base *splitPrototypeBase, ticks uint64) time.Time {
	delta := ticks - base.start
	if delta < base.limit {
		return time.Unix(base.sec, int64(delta))
	}
	return splitPrototypeFallback(base, ticks)
}

func TestSplitPrototypeArithmetic(t *testing.T) {
	// Include negative epochs, fraction carries, uint64 tick wrap, and the
	// int64 UnixNano upper boundary; compare complete time.Time values.
	random := rand.New(rand.NewSource(20260905))
	anchors := []int64{math.MinInt64, -1000000001, -1, 0, 1, 999999999, 1000000000, 1750000000123456789, math.MaxInt64 - 1}
	for n := 0; n < 1000; n++ {
		anchors = append(anchors, random.Int63()-random.Int63())
	}
	for _, unix := range anchors {
		ticks := random.Uint64()
		correction := int64(uint64(unix) - ticks)
		base := makeSplitPrototypeBase(ticks, correction)
		for _, elapsed := range []uint64{0, 1, 2, 999999999, 1000000000, 1000000001, 86400 * 1e9, 1 << 63} {
			sample := ticks + elapsed
			want := time.Unix(0, int64(sample+uint64(correction)))
			got := splitPrototypeAt(base, sample)
			if got != want {
				t.Fatalf("anchor=%d elapsed=%d: got %v want %v", unix, elapsed, got, want)
			}
			splitPrototypeMapping.Store(base)
			if got := splitPrototypeRebase(base, sample); got != want {
				t.Fatalf("rebase got %v want %v", got, want)
			}
		}
	}
}

func TestSplitPrototypeRefreshRace(t *testing.T) {
	old := makeSplitPrototypeBase(100, 1000000000)
	replacement := makeSplitPrototypeBase(200, 5000000000)
	splitPrototypeMapping.Store(replacement)
	splitPrototypeRebase(old, 2000000000)
	if splitPrototypeMapping.Load() != replacement {
		t.Fatal("stale rebase overwrote new correction")
	}
}

func TestSplitPrototypeLive(t *testing.T) {
	if clockNumer != clockDenom {
		t.Fatal("prototype requires nanosecond Mach ticks")
	}
	RefreshWallClock()
	refreshSplitPrototype()
	for _, read := range []func() time.Time{splitPrototypeNow, splitPrototypeRebaseNow, splitPrototypeAddNow} {
		for n := 0; n < 10000; n++ {
			before := Now()
			got := read()
			after := Now()
			if got.Before(before) || got.After(after) {
				t.Fatalf("%v outside [%v,%v]", got, before, after)
			}
			if got != time.Unix(0, got.UnixNano()) {
				t.Fatal("unexpected location or monotonic component")
			}
		}
	}
	// Refresh mapping snapshots and read/rebase them concurrently with GC.
	// Each publication uses one of two corrections, which must never mix.
	a, b := wallCorrection.Load(), wallCorrection.Load()+1234567890
	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; n < 5000; n++ {
				before := readTicks()
				got := splitPrototypeRebaseNow()
				after := readTicks()
				in := func(c int64) bool {
					return !got.Before(time.Unix(0, int64(before+uint64(c)))) && !got.After(time.Unix(0, int64(after+uint64(c))))
				}
				if !in(a) && !in(b) {
					t.Error("mixed or invalid mapping")
					return
				}
			}
		}()
	}
	for n := 0; n < 1000; n++ {
		correction := a
		if n%2 == 0 {
			correction = b
		}
		splitPrototypeMapping.Store(makeSplitPrototypeBase(readTicks()-2e9, correction))
		if n%100 == 0 {
			runtime.GC()
		}
	}
	wg.Wait()
	refreshSplitPrototype()
}

func setupSplitBenchmark(b *testing.B, aged bool) {
	b.StopTimer()
	// Start at a synthetic calendar-second boundary so the short benchmark
	// measures a full fast window, independently of launch-time phase.
	// This is only benchmark setup; real refreshes preserve the correction.
	ticks := readTicks()
	sec := int64(ticks+uint64(wallCorrection.Load())) / 1e9
	correction := int64(uint64(sec*1e9) - ticks)
	splitPrototypeMapping.Store(makeSplitPrototypeBase(ticks, correction))
	if aged {
		base := splitPrototypeMapping.Load()
		splitPrototypeMapping.Store(makeSplitPrototypeBase(readTicks()-2e9, base.correction))
	}
	b.ReportAllocs()
	b.ResetTimer()
	b.StartTimer()
}

func BenchmarkSplitCurrentNow(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		timeSink = Now()
	}
}

func BenchmarkSplitCurrentUnixNano(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		unixNanoSink = UnixNano()
	}
}

func BenchmarkSplitFixedFresh(b *testing.B) {
	setupSplitBenchmark(b, false)
	for n := 0; n < b.N; n++ {
		timeSink = splitPrototypeNow()
	}
}

func BenchmarkSplitFixedAged(b *testing.B) {
	setupSplitBenchmark(b, true)
	for n := 0; n < b.N; n++ {
		timeSink = splitPrototypeNow()
	}
}

func BenchmarkSplitRebaseFresh(b *testing.B) {
	setupSplitBenchmark(b, false)
	for n := 0; n < b.N; n++ {
		timeSink = splitPrototypeRebaseNow()
	}
}

func BenchmarkSplitRebaseAged(b *testing.B) {
	setupSplitBenchmark(b, true)
	for n := 0; n < b.N; n++ {
		timeSink = splitPrototypeRebaseNow()
	}
}

func BenchmarkSplitAddFresh(b *testing.B) {
	setupSplitBenchmark(b, false)
	for n := 0; n < b.N; n++ {
		timeSink = splitPrototypeAddNow()
	}
}

func BenchmarkSplitAddAged(b *testing.B) {
	setupSplitBenchmark(b, true)
	for n := 0; n < b.N; n++ {
		timeSink = splitPrototypeAddNow()
	}
}

// Forces a real slow path every iteration. Includes the atomic Store used to
// expire the mapping, so this is an upper-bound boundary cost, not steady state.
func BenchmarkSplitRebaseBoundary(b *testing.B) {
	setupSplitBenchmark(b, true)
	expired := splitPrototypeMapping.Load()
	for n := 0; n < b.N; n++ {
		splitPrototypeMapping.Store(expired)
		timeSink = splitPrototypeRebaseNow()
	}
}

func BenchmarkSplitRebaseSteady(b *testing.B) {
	setupSplitBenchmark(b, false)
	for n := 0; n < b.N; n++ {
		timeSink = splitPrototypeRebaseNow()
	}
}
