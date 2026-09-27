package coarsetime

import (
	"sync"
	"testing"
	"time"
)

func TestWallClock(t *testing.T) {
	before := time.Now()
	values := []time.Time{Now(), time.Unix(0, UnixNano())}
	after := time.Now()
	for _, v := range values {
		// Sanity check, not a promised staleness bound.
		if v.Before(before.Add(-time.Second)) || v.After(after.Add(time.Second)) {
			t.Fatalf("wall time %v is far from [%v, %v]", v, before, after)
		}
		if v != v.Round(0) {
			t.Fatal("wall result contains a monotonic component")
		}
	}
}

func TestConcurrentWallReads(t *testing.T) {
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; n < 1000; n++ {
				if Now().IsZero() {
					t.Error("invalid concurrent wall reading")
					return
				}
				_ = UnixNano()
			}
		}()
	}
	wg.Wait()
}
