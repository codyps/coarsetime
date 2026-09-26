//go:build !purego

package coarsetime

import "testing"

func TestFiletimeUnixNano(t *testing.T) {
	for _, ns := range []int64{
		-9223372036854775800, // First representable 100 ns tick.
		-2208988800000000000, // 1900-01-01.
		-100, 0, 100,
		946684800000000000,  // 2000-01-01.
		9223372036854775800, // Last representable 100 ns tick.
	} {
		ticks := uint64(int64(windowsUnixEpoch) + ns/100)
		if got := filetimeUnixNano(ticks); got != ns {
			t.Errorf("FILETIME %d: got %d, want %d", ticks, got, ns)
		}
	}
}
