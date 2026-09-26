//go:build purego

package coarsetime

import (
	"testing"
	"time"
)

func TestPureGoWallIgnoresCachedCorrection(t *testing.T) {
	saved := wallCorrection.Load()
	defer wallCorrection.Store(saved)
	i := NowInstant()
	initial := i.Time()
	wallCorrection.Store(saved + int64(24*time.Hour))
	if !i.Time().Equal(initial.Add(24 * time.Hour)) {
		t.Fatal("Instant.Time must still use the cached correction")
	}
	for _, got := range []time.Time{Now(), time.Unix(0, UnixNano())} {
		if delta := time.Since(got); delta < -time.Second || delta > time.Second {
			t.Fatalf("purego wall read used cached correction: %v", got)
		}
		if got != got.Round(0) {
			t.Fatal("purego wall read contains a monotonic component")
		}
	}
}
