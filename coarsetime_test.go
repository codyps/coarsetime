package coarsetime_test

import (
	"sync"
	"testing"
	"time"

	"github.com/codyps/coarsetime"
)

func TestNow(t *testing.T) {
	before := time.Now()
	got := coarsetime.Now()
	after := time.Now()
	if got.Before(before) || got.After(after) {
		t.Fatalf("Now outside surrounding time.Now readings: %v <= %v <= %v", before, got, after)
	}
	if got.Location() != time.Local {
		t.Fatal("Now did not preserve local location")
	}
	if got == got.Round(0) {
		t.Fatal("Now did not preserve monotonic reading")
	}
}

func TestUnixNanoConcurrent(t *testing.T) {
	var workers sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for i := 0; i < 1000; i++ {
				before := time.Now().UnixNano()
				got := coarsetime.UnixNano()
				after := time.Now().UnixNano()
				// Allow for clock adjustments and different source resolutions.
				// This is a gross conversion/epoch check, not a staleness contract.
				if before > after {
					before, after = after, before
				}
				if got < before-int64(time.Second) || got > after+int64(time.Second) {
					t.Errorf("UnixNano outside current wall-clock range: %d <= %d <= %d", before, got, after)
					return
				}
			}
		}()
	}
	workers.Wait()
}

var sinkTime time.Time
var sinkNano int64

func BenchmarkNow(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sinkTime = coarsetime.Now()
	}
}

func BenchmarkTimeNow(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sinkTime = time.Now()
	}
}

func BenchmarkUnixNano(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sinkNano = coarsetime.UnixNano()
	}
}

func BenchmarkTimeNowUnixNano(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sinkNano = time.Now().UnixNano()
	}
}
