package coarsetime_test

import (
	"sync"
	"testing"
	"time"

	"github.com/codyps/coarsetime"
)

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

