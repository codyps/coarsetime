//go:build linux && !purego

package coarsetime

import (
	"testing"
	"time"
)

// Compare the existing raw syscall with the proposed standard-library wall
// fallback in the same process, including the public API's time conversion.
func BenchmarkLinuxWallFallback(b *testing.B) {
	b.Run("SyscallNow", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			timeSink = time.Unix(0, readRealtimeSyscall())
		}
	})
	b.Run("TimeNow", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			timeSink = time.Now().Round(0)
		}
	})
	b.Run("SyscallUnixNano", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			unixNanoSink = readRealtimeSyscall()
		}
	})
	b.Run("TimeNowUnixNano", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			unixNanoSink = time.Now().UnixNano()
		}
	})
}
