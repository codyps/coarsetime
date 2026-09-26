//go:build walloptprototype

package coarsetime

import (
	"math"
	"math/rand"
	"testing"
	"time"
)

func TestWallOptNormalization(t *testing.T) {
	samples := []int64{math.MinInt64, math.MinInt64 + 1, -1000000001, -1000000000, -999999999, -1, 0, 1, 999999999, 1000000000, 1000000001, math.MaxInt64 - 1, math.MaxInt64}
	rng := rand.New(rand.NewSource(20260905))
	for n := 0; n < 100000; n++ {
		samples = append(samples, int64(rng.Uint64()))
	}
	for _, ns := range samples {
		want := time.Unix(0, ns)
		for _, fn := range []func(int64) time.Time{wallOptUnsignedTime, wallOptSignedTime} {
			if got := fn(ns); got != want {
				t.Fatalf("ns=%d: got %#v want %#v", ns, got, want)
			}
		}
	}
}

func BenchmarkWallOptCurrentNow(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		timeSink = Now()
	}
}
func BenchmarkWallOptCurrentUnixNano(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		unixNanoSink = UnixNano()
	}
}
func BenchmarkWallOptUnsignedNow(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		timeSink = wallOptUnsignedNow()
	}
}
func BenchmarkWallOptSignedNow(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		timeSink = wallOptSignedNow()
	}
}
