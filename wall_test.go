package coarsetime

import (
	"sync"
	"testing"
	"time"
)

func TestWallClock(t *testing.T) {
	RefreshWallClock()
	before := time.Now()
	values := []time.Time{Now(), time.Unix(0, UnixNano()), NowInstant().Time()}
	after := time.Now()
	for _, v := range values {
		// Sanity check, not a promised staleness bound. Deterministic correction
		// behavior is tested separately below.
		if v.Before(before.Add(-time.Second)) || v.After(after.Add(time.Second)) {
			t.Fatalf("wall time %v is far from [%v, %v]", v, before, after)
		}
		if v != v.Round(0) {
			t.Fatal("wall result contains a monotonic component")
		}
	}
}

func TestWallCorrection(t *testing.T) {
	instant := NowInstant()
	saved := wallCorrection.Load()
	defer wallCorrection.Store(saved)
	initial := instant.Time()
	for _, delta := range []time.Duration{time.Hour, -2 * time.Hour, 0} {
		wallCorrection.Store(saved + int64(delta))
		if got := instant.Time(); !got.Equal(initial.Add(delta)) {
			t.Fatalf("correction %v: got %v, want %v", delta, got, initial.Add(delta))
		}
		if instant.Sub(instant) != 0 {
			t.Fatal("correction affected Instant")
		}
	}
}

func TestWallRefresh(t *testing.T) {
	wallCorrection.Add(int64(24 * time.Hour))
	RefreshWallClock()
	if delta := time.Since(NowInstant().Time()); delta < -time.Second || delta > time.Second {
		t.Fatalf("refresh did not remove synthetic wall error: %v", delta)
	}
}

func TestConcurrentWallRefresh(t *testing.T) {
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; n < 1000; n++ {
				if n%100 == 0 {
					RefreshWallClock()
				}
				if Now().IsZero() || NowInstant().Time().IsZero() {
					t.Error("invalid concurrent wall reading")
					return
				}
				_ = UnixNano()
			}
		}()
	}
	wg.Wait()
}
