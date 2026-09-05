package coarsetime

import (
	"math"
	"math/big"
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestScaleTicks(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	cases := [][3]uint64{
		{math.MaxUint64, 125, 3}, {math.MaxUint64, 3, 125},
		{math.MaxUint64, 1, 1}, {1, 1, 3}, {3, 125, 3},
		{math.MaxInt64, 1, 1}, {1 << 63, 1, 1},
	}
	for n := 0; n < 1000; n++ {
		cases = append(cases, [3]uint64{rng.Uint64(), uint64(rng.Uint32()) + 1, uint64(rng.Uint32()) + 1})
	}
	for _, c := range cases {
		for _, limit := range []uint64{math.MaxInt64, 1 << 63} {
			want := new(big.Int).SetUint64(c[0])
			want.Mul(want, new(big.Int).SetUint64(c[1]))
			want.Div(want, new(big.Int).SetUint64(c[2]))
			cap := new(big.Int).SetUint64(limit)
			if want.Cmp(cap) > 0 {
				want.Set(cap)
			}
			if got := scaleTicks(c[0], c[1], c[2], limit); got != want.Uint64() {
				t.Fatalf("scaleTicks(%v, %d) = %d, want %s", c, limit, got, want)
			}
		}
	}
}

func TestInstant(t *testing.T) {
	a, b := Instant{100}, Instant{200}
	if !a.Before(b) || !b.After(a) || a.After(b) || b.Before(a) || a.Before(a) || a.After(a) {
		t.Fatal("incorrect ordering")
	}
	if a.Sub(a) != 0 || a.Sub(b) != -b.Sub(a) || b.Sub(a) < 0 {
		t.Fatal("incorrect duration arithmetic")
	}
}

func TestClockProgress(t *testing.T) {
	start := NowInstant()
	deadline := time.Now().Add(3 * time.Second)
	for !NowInstant().After(start) {
		if time.Now().After(deadline) {
			t.Fatal("clock did not advance within 3s")
		}
		time.Sleep(time.Millisecond)
	}
	if Since(start) <= 0 {
		t.Fatal("Since did not report progress")
	}
}

// Exercise the native bridge during stack growth, GC, and concurrent reads.
func TestConcurrentReads(t *testing.T) {
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			previous := NowInstant()
			for n := 0; n < 20000; n++ {
				current := NowInstant()
				if current.Before(previous) {
					t.Error("clock moved backwards")
					return
				}
				previous = current
				if n%1000 == 0 {
					growStack(20)
					runtime.Gosched()
				}
			}
		}()
	}
	for n := 0; n < 5; n++ {
		runtime.GC()
	}
	wg.Wait()
}

//go:noinline
func growStack(depth int) byte {
	var pad [1024]byte
	pad[depth] = byte(depth)
	if depth > 0 {
		pad[0] = growStack(depth - 1)
	}
	runtime.KeepAlive(&pad)
	return pad[depth]
}

func TestSubAgainstBigInt(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	pairs := [][2]uint64{{0, math.MaxUint64}, {math.MaxUint64, 0}, {0, 1 << 63}, {1 << 63, 0}, {0, 0}}
	for n := 0; n < 1000; n++ {
		pairs = append(pairs, [2]uint64{rng.Uint64(), rng.Uint64()})
	}
	for _, pair := range pairs {
		want := new(big.Int).Sub(new(big.Int).SetUint64(pair[0]), new(big.Int).SetUint64(pair[1]))
		want.Mul(want, new(big.Int).SetUint64(clockNumer))
		want.Quo(want, new(big.Int).SetUint64(clockDenom))
		min, max := big.NewInt(math.MinInt64), big.NewInt(math.MaxInt64)
		if want.Cmp(min) < 0 {
			want.Set(min)
		}
		if want.Cmp(max) > 0 {
			want.Set(max)
		}
		if got := (Instant{pair[0]}).Sub(Instant{pair[1]}); int64(got) != want.Int64() {
			t.Fatalf("Sub(%v) = %d, want %s", pair, got, want)
		}
	}
}
