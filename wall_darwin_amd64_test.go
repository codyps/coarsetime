package coarsetime

import (
	"math"
	"math/rand"
	"testing"
	"time"
)

func TestDarwinWallNormalization(t *testing.T) {
	saved := wallCorrection.Load()
	defer wallCorrection.Store(saved)
	rng := rand.New(rand.NewSource(72))
	samples := []int64{math.MinInt64, -1000000001, -1, 0, 1, 999999999, 1000000000, math.MaxInt64}
	for n := 0; n < 100000; n++ {
		samples = append(samples, int64(rng.Uint64()))
	}
	for _, ns := range samples {
		i := Instant{ticks: uint64(ns) - uint64(saved)}
		if got, want := i.Time(), time.Unix(0, ns); got != want {
			t.Fatalf("ns=%d: got %v want %v", ns, got, want)
		}
	}
	// Freeze the read through a local atomic target to exercise Now's own
	// normalization, including negative dates, without modifying the OS clock.
	pointer, fast := approximateTimePointer, wallCommpageSupported
	defer func() { approximateTimePointer = pointer; wallCommpageSupported = fast }()
	var ticks uint64
	approximateTimePointer = &ticks
	wallCommpageSupported = true
	for _, ns := range samples[:8] {
		wallCorrection.Store(ns)
		if got, want := Now(), time.Unix(0, ns); got != want {
			t.Fatalf("Now: got %v want %v", got, want)
		}
	}
}

func TestDarwinWallFallbacks(t *testing.T) {
	fast, unit, supported, pointer := wallCommpageSupported, wallUnitTimebase, approximateTimeSupported, approximateTimePointer
	numer, denom := clockNumer, clockDenom
	defer func() {
		wallCommpageSupported = fast
		wallUnitTimebase = unit
		approximateTimeSupported = supported
		approximateTimePointer = pointer
		clockNumer = numer
		clockDenom = denom
	}()
	wallCommpageSupported = false
	approximateTimeSupported = false
	approximateTimePointer = nil
	for _, ratio := range [][2]uint64{{1, 1}, {2, 1}, {1, 3}} {
		clockNumer, clockDenom = ratio[0], ratio[1]
		wallUnitTimebase = clockNumer == clockDenom
		for n := 0; n < 1000; n++ {
			correction := wallCorrection.Load()
			before := correctedUnixNano(NowInstant(), correction)
			got := UnixNano()
			now := Now()
			after := correctedUnixNano(NowInstant(), correction)
			if got < before || got > after || now.Before(time.Unix(0, before)) || now.After(time.Unix(0, after)) {
				t.Fatal("fallback outside clock bracket")
			}
			i := Instant{ticks: uint64(n) * 123456789}
			if i.Time() != time.Unix(0, correctedUnixNano(i, correction)) {
				t.Fatal("fallback scaling mismatch")
			}
		}
	}
}
