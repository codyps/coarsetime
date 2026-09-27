package coarsetime

import (
	"runtime"
	"testing"
	"time"
)

var instantSink Instant
var durationSink time.Duration
var timeSink time.Time

func BenchmarkNowInstant(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		instantSink = NowInstant()
	}
}

func BenchmarkSince(b *testing.B) {
	start := NowInstant()
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		durationSink = Since(start)
	}
}

func BenchmarkTimeSince(b *testing.B) {
	start := time.Now()
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		durationSink = time.Since(start)
	}
}

func BenchmarkTimeNow(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		timeSink = time.Now()
	}
}

func BenchmarkNowInstantParallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var last Instant
		for pb.Next() {
			last = NowInstant()
		}
		runtime.KeepAlive(last)
	})
}

func BenchmarkTimeSinceParallel(b *testing.B) {
	start := time.Now()
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		var last time.Duration
		for pb.Next() {
			last = time.Since(start)
		}
		runtime.KeepAlive(last)
	})
}

// Diagnostic sampling, not a read-latency benchmark. Step size measures observed
// granularity; it does not establish an accuracy or staleness bound.
func BenchmarkClockProgress(b *testing.B) {
	previous := NowInstant()
	var repeated int64
	var maxStep time.Duration
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		next := NowInstant()
		if next == previous {
			repeated++
		}
		if step := next.Sub(previous); step > maxStep {
			maxStep = step
		}
		previous = next
	}
	b.ReportMetric(float64(repeated)*100/float64(b.N), "repeat-%")
	b.ReportMetric(float64(maxStep), "max-step-ns")
}

var unixNanoSink int64

func BenchmarkNow(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		timeSink = Now()
	}
}

func BenchmarkUnixNano(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		unixNanoSink = UnixNano()
	}
}

func BenchmarkNowParallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var last time.Time
		for pb.Next() {
			last = Now()
		}
		runtime.KeepAlive(last)
	})
}

// Mutable function variables model callers that retain the public API as a
// callback. These calls cannot inline the generic API wrappers.
var nowFuncValue = Now
var unixNanoFuncValue = UnixNano

func BenchmarkNowFuncValue(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		timeSink = nowFuncValue()
	}
}

func BenchmarkUnixNanoFuncValue(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		unixNanoSink = unixNanoFuncValue()
	}
}
