package darwinwall

import (
	"math"
	"math/big"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"unsafe"
)

func TestPageLayout(t *testing.T) {
	var p Page
	if unsafe.Sizeof(p) != 40 || unsafe.Offsetof(p.Tick) != 0 || unsafe.Offsetof(p.Seconds) != 8 ||
		unsafe.Offsetof(p.Fraction) != 16 || unsafe.Offsetof(p.Scale) != 24 || unsafe.Offsetof(p.TicksPerSec) != 32 {
		t.Fatal("Page no longer matches XNU's five-uint64 layout")
	}
}

func TestSnapshot(t *testing.T) {
	base := snapshot{tick: 100, seconds: 1, scale: 1 << 62, ticksPerSec: 4}
	for _, tc := range []struct {
		now  uint64
		want int64
	}{{100, 1e9}, {101, 1250000000}, {103, 1750000000}} {
		if got, ok := base.unixNano(tc.now, 100); !ok || got != tc.want {
			t.Fatalf("tick %d: %d, %v", tc.now, got, ok)
		}
	}
	carry := base
	carry.fraction = 3 << 62
	if got, ok := carry.unixNano(101, 100); !ok || got != 2e9 {
		t.Fatalf("fraction carry: %d %v", got, ok)
	}
	for _, tc := range []struct {
		name         string
		s            snapshot
		now, confirm uint64
	}{
		{"invalid", snapshot{}, 100, 0},
		{"changed", base, 101, 102},
		{"invalidated", base, 101, 0},
		{"sample-before-anchor", base, 99, 100},
		{"expired", base, 104, 100},
		{"very-old", base, math.MaxUint64, 100},
		{"zero-rate", snapshot{tick: 100, scale: 1}, 100, 100},
		{"zero-scale", snapshot{tick: 100, ticksPerSec: 4}, 100, 100},
		{"negative-calendar", snapshot{tick: 100, seconds: math.MaxUint64, scale: 1, ticksPerSec: 4}, 100, 100},
		{"seconds-overflow", snapshot{tick: 100, seconds: math.MaxUint64, fraction: math.MaxUint64, scale: math.MaxUint64, ticksPerSec: 4}, 103, 100},
		{"nanosecond-overflow", snapshot{tick: 100, seconds: math.MaxInt64 / 1_000_000_000, fraction: math.MaxUint64, scale: 1, ticksPerSec: 4}, 100, 100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, ok := tc.s.unixNano(tc.now, tc.confirm); ok {
				t.Fatalf("accepted invalid snapshot: %d", got)
			}
		})
	}
}

func TestArithmeticAgainstBigInt(t *testing.T) {
	rng := rand.New(rand.NewSource(81))
	limit := big.NewInt(math.MaxInt64)
	for i := 0; i < 10000; i++ {
		s := snapshot{tick: 1, seconds: rng.Uint64() % (math.MaxInt64/1_000_000_000 + 1), fraction: rng.Uint64(), scale: rng.Uint64() | 1, ticksPerSec: math.MaxUint64}
		delta := uint64(rng.Uint32())
		// Independent rational calculation: (seconds*2^64 + fraction + delta*scale) * 1e9 / 2^64.
		want := new(big.Int).Lsh(new(big.Int).SetUint64(s.seconds), 64)
		want.Add(want, new(big.Int).SetUint64(s.fraction))
		product := new(big.Int).Mul(new(big.Int).SetUint64(delta), new(big.Int).SetUint64(s.scale))
		want.Add(want, product).Mul(want, big.NewInt(1e9)).Rsh(want, 64)
		got, ok := s.unixNano(1+delta, 1)
		if want.Cmp(limit) > 0 {
			if ok {
				t.Fatal("accepted unrepresentable result")
			}
			continue
		}
		if !ok || got != want.Int64() {
			t.Fatalf("got %d %v, want %s", got, ok, want)
		}
	}
}

func TestKernelUpdates(t *testing.T) {
	var p Page
	var tick atomic.Uint64
	p.Seconds.Store(10)
	p.Scale.Store(1 << 62)
	p.TicksPerSec.Store(4)
	tick.Store(100)
	p.Tick.Store(100)
	if got, ok := Read(&p, &tick); !ok || got != 10e9 {
		t.Fatal(got, ok)
	}
	// Model invalidation and a wall step/resume. The old approximate sample
	// must not be interpolated backwards from the new calendar anchor.
	p.Tick.Store(0)
	if _, ok := Read(&p, &tick); ok {
		t.Fatal("read during invalidation")
	}
	p.Seconds.Store(20)
	p.Tick.Store(101)
	if _, ok := Read(&p, &tick); ok {
		t.Fatal("accepted sample before refreshed anchor")
	}
	tick.Store(101)
	if got, ok := Read(&p, &tick); !ok || got != 20e9 {
		t.Fatal(got, ok)
	}
	// Kernel slewing changes the scale, not just a constant epoch offset.
	p.Tick.Store(0)
	p.Scale.Store(1 << 61)
	p.Tick.Store(102)
	tick.Store(103)
	if got, ok := Read(&p, &tick); !ok || got != 20125000000 {
		t.Fatal(got, ok)
	}
	tick.Store(106)
	if _, ok := Read(&p, &tick); ok {
		t.Fatal("accepted expired sample")
	}
}

func TestConcurrentPublication(t *testing.T) {
	var p Page
	var tick atomic.Uint64
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for version := uint64(1); version <= 20000; version++ {
			p.Tick.Store(0)
			p.Seconds.Store(version * 2)
			p.Fraction.Store((version % 2) << 63)
			p.Scale.Store(1)
			p.TicksPerSec.Store(1)
			tick.Store(version)
			p.Tick.Store(version)
		}
	}()
	for reader := 0; reader < 4; reader++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 20000; i++ {
				if ns, ok := Read(&p, &tick); ok {
					sec, frac := ns/1e9, ns%1e9
					if sec%2 != 0 || frac != (sec/2%2)*500000000 {
						t.Errorf("mixed snapshot: %d", ns)
						return
					}
				}
			}
		}()
	}
	wg.Wait()
	if got, ok := Read(&p, &tick); !ok || got != 40000e9 {
		t.Fatal(got, ok)
	}
}

var nanoSink int64
var okSink bool

// This measures the algorithm with Go-owned memory, not a real Darwin commpage.
func BenchmarkSyntheticRead(b *testing.B) {
	var p Page
	var tick atomic.Uint64
	p.Tick.Store(100)
	p.Seconds.Store(1700000000)
	p.Fraction.Store(1 << 63)
	p.Scale.Store(18446744072)
	p.TicksPerSec.Store(1e9)
	tick.Store(100000100)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		nanoSink, okSink = Read(&p, &tick)
	}
}
