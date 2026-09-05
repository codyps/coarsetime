//go:build walloptprototype

package coarsetime

import "time"

// Both normalization candidates preserve time.Unix's negative-epoch behavior.
// They avoid depending on time.Time's private memory layout.
func wallOptUnsignedTime(ns int64) time.Time {
	if ns < 0 {
		return time.Unix(0, ns)
	}
	u := uint64(ns)
	return time.Unix(int64(u/1e9), int64(u%1e9))
}

func wallOptSignedTime(ns int64) time.Time {
	sec, frac := ns/int64(1e9), ns%int64(1e9)
	if frac < 0 {
		sec--
		frac += 1e9
	}
	return time.Unix(sec, frac)
}

func wallOptUnsignedNow() time.Time { return wallOptUnsignedTime(UnixNano()) }
func wallOptSignedNow() time.Time   { return wallOptSignedTime(UnixNano()) }
