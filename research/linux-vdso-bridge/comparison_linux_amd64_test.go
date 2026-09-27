package bridge

import (
	"os"
	"testing"
)

type comparisonCase struct {
	name string
	read clockReader
}

func comparisonCases() []comparisonCase {
	cases := append(directComparison(), comparisonCase{"resolved", Indirect})
	if os.Getenv("COARSETIME_BENCH_REVERSE") == "1" {
		for i, j := 0, len(cases)-1; i < j; i, j = i+1, j-1 {
			cases[i], cases[j] = cases[j], cases[i]
		}
	}
	return cases
}

func TestComparisonClocks(t *testing.T) {
	readers(t) // Require the actual vDSO and a successfully resolved bridge.
	for _, c := range comparisonCases() {
		t.Run(c.name, func(t *testing.T) {
			for _, id := range []int32{5, 6} {
				for n := 0; n < 10000; n++ {
					lo := syscallClock(id)
					s, ns, result := c.read(id)
					hi := syscallClock(id)
					got := s*1e9 + ns
					if result != 0 || ns < 0 || ns >= 1e9 || (hi >= lo && (got < lo || got > hi)) {
						t.Fatalf("clock %d: %d (%d) outside [%d,%d]", id, got, result, lo, hi)
					}
				}
			}
		})
	}
}

// Same argument record, C-ABI trampoline, clock ID, result conversion, and Go
// function-value call on both sides. Only the runtime bridge binding differs.
func BenchmarkBridgeComparison(b *testing.B) {
	readers(b)
	for _, c := range comparisonCases() {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			for n := 0; n < b.N; n++ {
				s, ns, result := c.read(5)
				if result != 0 {
					b.Fatal(result)
				}
				sink = s*1e9 + ns
			}
		})
	}
}
