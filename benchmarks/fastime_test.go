package benchmarks

import (
	"coarsetime"
	"context"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/kpango/fastime"
)

var timeSink time.Time
var nanoSink int64

func TestMain(m *testing.M) {
	// fastime starts a global 5 ms updater on import. Keep reference benchmarks
	// free of that work; start and stop it around each fastime sub-benchmark.
	fastime.Stop()
	os.Exit(m.Run())
}

func startFastime(b *testing.B, interval time.Duration) {
	b.Helper()
	b.StopTimer()
	ctx, cancel := context.WithCancel(context.Background())
	fastime.StartTimerD(ctx, interval)
	b.Cleanup(func() { cancel(); fastime.Stop() })
	if !fastime.IsDaemonRunning() {
		b.Fatal("fastime updater is not running")
	}
	b.ReportAllocs()
	b.ResetTimer()
	b.StartTimer()
}

func BenchmarkFastimeNow(b *testing.B) {
	for _, interval := range []time.Duration{time.Millisecond, 5 * time.Millisecond} {
		b.Run(interval.String(), func(b *testing.B) {
			startFastime(b, interval)
			for n := 0; n < b.N; n++ {
				timeSink = fastime.Now()
			}
		})
	}
}

func BenchmarkFastimeUnixNano(b *testing.B) {
	for _, interval := range []time.Duration{time.Millisecond, 5 * time.Millisecond} {
		b.Run(interval.String(), func(b *testing.B) {
			startFastime(b, interval)
			for n := 0; n < b.N; n++ {
				nanoSink = fastime.UnixNanoNow()
			}
		})
	}
}

func BenchmarkFastimeNowParallel(b *testing.B) {
	for _, interval := range []time.Duration{time.Millisecond, 5 * time.Millisecond} {
		b.Run(interval.String(), func(b *testing.B) {
			startFastime(b, interval)
			b.RunParallel(func(pb *testing.PB) {
				var last time.Time
				for pb.Next() {
					last = fastime.Now()
				}
				runtime.KeepAlive(last)
			})
		})
	}
}

func BenchmarkCoarsetimeNow(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		timeSink = coarsetime.Now()
	}
}

func BenchmarkCoarsetimeUnixNano(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		nanoSink = coarsetime.UnixNano()
	}
}

func BenchmarkTimeNow(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		timeSink = time.Now()
	}
}

func BenchmarkCoarsetimeNowParallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var last time.Time
		for pb.Next() {
			last = coarsetime.Now()
		}
		runtime.KeepAlive(last)
	})
}

func BenchmarkTimeNowParallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var last time.Time
		for pb.Next() {
			last = time.Now()
		}
		runtime.KeepAlive(last)
	})
}
