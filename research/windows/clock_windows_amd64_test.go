package windowsbench

import (
	"runtime"
	"sort"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

var kernel32 = syscall.NewLazyDLL("kernel32.dll")
var kernelbase = syscall.NewLazyDLL("kernelbase.dll")
var sinkTime time.Time
var sinkInt int64
var sinkUint uint64

func proc(tb testing.TB, name string) *syscall.LazyProc {
	tb.Helper()
	p := kernel32.NewProc(name)
	if name == "QueryInterruptTime" || name == "QueryInterruptTimePrecise" {
		p = kernelbase.NewProc(name)
	}
	if err := p.Find(); err != nil {
		tb.Fatal(err)
	}
	return p
}

func BenchmarkClock(b *testing.B) {
	b.Run("TimeNow", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkTime = time.Now()
		}
	})
	b.Run("TimeNowUnixNano", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkInt = time.Now().UnixNano()
		}
	})
	b.Run("TimeSince", func(b *testing.B) {
		start := time.Now()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			sinkInt = int64(time.Since(start))
		}
	})
	b.Run("SharedTime", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkTime = sharedNow()
		}
	})
	b.Run("SharedUnixNano", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkInt = sharedUnixNano()
		}
	})
	b.Run("SharedFiletimeRaw", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkUint = sharedFiletime()
		}
	})
	b.Run("SharedInterruptRaw", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sinkUint = sharedInterrupt()
		}
	})
	for _, name := range []string{"GetSystemTimeAsFileTime", "GetSystemTimePreciseAsFileTime", "QueryPerformanceCounter", "QueryInterruptTime", "QueryInterruptTimePrecise"} {
		b.Run(name+"Raw", func(b *testing.B) {
			addr := proc(b, name).Addr()
			var value uint64
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				syscall.SyscallN(addr, uintptr(unsafe.Pointer(&value)))
				sinkUint = value
			}
			runtime.KeepAlive(&value)
		})
		if name == "GetSystemTimeAsFileTime" || name == "GetSystemTimePreciseAsFileTime" {
			b.Run(name+"Time", func(b *testing.B) {
				addr := proc(b, name).Addr()
				var value uint64
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					syscall.SyscallN(addr, uintptr(unsafe.Pointer(&value)))
					sinkTime = filetimeToTime(value)
				}
				runtime.KeepAlive(&value)
			})
		}
	}
	b.Run("GetTickCount64Raw", func(b *testing.B) {
		addr := proc(b, "GetTickCount64").Addr()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			r, _, _ := syscall.SyscallN(addr)
			sinkUint = uint64(r)
		}
	})
}

func TestSharedClocks(t *testing.T) {
	for i := 0; i < 10000; i++ {
		before := time.Now()
		got := sharedNow()
		after := time.Now()
		if got.Before(before.Round(0)) || got.After(after.Round(0)) {
			t.Fatalf("shared wall clock outside time.Now bounds: %v <= %v <= %v", before, got, after)
		}
		if got != got.Round(0) {
			t.Fatal("unexpected monotonic component")
		}
		a, z := sharedInterrupt(), sharedInterrupt()
		if z < a {
			t.Fatal("interrupt clock regressed")
		}
	}
	if got := filetimeToTime(filetimeEpoch); !got.Equal(time.Unix(0, 0)) {
		t.Fatal(got)
	}
}

// Diagnostic (skipped with -short): observed increments, not a precision guarantee.
// Sampling overhead and descheduling can increase observed increments.
func TestResolution(t *testing.T) {
	if testing.Short() {
		t.Skip("resolution sampling")
	}
	var value uint64
	qpc := proc(t, "QueryPerformanceCounter")
	qpf := proc(t, "QueryPerformanceFrequency")
	var freq uint64
	r, _, _ := qpf.Call(uintptr(unsafe.Pointer(&freq)))
	if r == 0 || freq == 0 {
		t.Fatal("QueryPerformanceFrequency failed")
	}
	t.Logf("QPC frequency: %d Hz", freq)
	cases := []struct {
		name string
		read func() int64
	}{
		{"TimeNow", func() int64 { return time.Now().UnixNano() }},
		{"SharedUnixNano", sharedUnixNano},
		{"SharedInterrupt", func() int64 { return int64(sharedInterrupt()) * 100 }},
		{"QPC", func() int64 {
			qpc.Call(uintptr(unsafe.Pointer(&value)))
			return int64(value/freq)*1e9 + int64(value%freq)*1e9/int64(freq)
		}},
	}
	for _, name := range []string{"GetSystemTimeAsFileTime", "GetSystemTimePreciseAsFileTime", "QueryInterruptTime", "QueryInterruptTimePrecise"} {
		p := proc(t, name)
		cases = append(cases, struct {
			name string
			read func() int64
		}{name, func() int64 {
			p.Call(uintptr(unsafe.Pointer(&value)))
			if name == "GetSystemTimeAsFileTime" || name == "GetSystemTimePreciseAsFileTime" {
				return int64(value-filetimeEpoch) * 100
			}
			return int64(value) * 100
		}})
	}
	for _, c := range cases {
		end := time.Now().Add(250 * time.Millisecond)
		prev := c.read()
		var deltas []int64
		var same, backwards int
		for time.Now().Before(end) {
			now := c.read()
			if now > prev {
				deltas = append(deltas, now-prev)
			} else if now == prev {
				same++
			} else {
				backwards++
			}
			prev = now
		}
		sort.Slice(deltas, func(i, j int) bool { return deltas[i] < deltas[j] })
		if len(deltas) == 0 {
			t.Fatalf("%s never advanced", c.name)
		}
		t.Logf("%s: min=%dns median=%dns changes=%d repeats=%d backwards=%d", c.name, deltas[0], deltas[len(deltas)/2], len(deltas), same, backwards)
	}
	runtime.KeepAlive(&value)
}
