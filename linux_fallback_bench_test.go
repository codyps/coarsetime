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

func BenchmarkLinuxInstantFallback(b *testing.B) {
	b.Run("Syscall", BenchmarkLinuxCoarseSyscall)
	b.Run("TimeSince", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			durationSink = time.Since(linuxOrigin)
		}
	})
	b.Run("NowInstant", func(b *testing.B) {
		saved := linuxCoarseClock
		linuxCoarseClock = false
		defer func() { linuxCoarseClock = saved }()
		BenchmarkNowInstant(b)
	})
}
