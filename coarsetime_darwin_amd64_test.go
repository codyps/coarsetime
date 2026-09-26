//go:build !purego

package coarsetime

import "testing"

func TestDarwinCommpageAgainstLibc(t *testing.T) {
	t.Logf("direct approximate time supported=%v", approximateTimeSupported)
	for _, tc := range []struct {
		name string
		read func() uint64
	}{
		{"selected", readTicks},
		{"forced-fallback", func() uint64 { return readCommpageTicks(false) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// libc reads the same monotonic approximate clock, so scheduler
			// updates between samples are allowed but epoch mismatches are not.
			for n := 0; n < 10000; n++ {
				before := readLibcTicks()
				got := tc.read()
				after := readLibcTicks()
				if got < before || got > after {
					t.Fatalf("sample %d outside libc bracket [%d, %d]", got, before, after)
				}
			}
		})
	}
}
