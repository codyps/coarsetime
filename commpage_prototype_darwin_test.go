//go:build commpageprototype && darwin && (amd64 || arm64)

package coarsetime

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
	"unsafe"
)

func TestPrototypeReaders(t *testing.T) {
	t.Logf("direct commpage supported=%v, pointer=%p", prototypeSupported, prototypeApproxPointer)
	if !prototypeSupported {
		t.Skip("direct approximate time unavailable on this host")
	}
	if uintptr(unsafe.Pointer(prototypeApproxPointer))%8 != 0 {
		t.Fatal("unaligned approximate timestamp")
	}
	readers := []struct {
		name string
		read func() uint64
	}{
		{"checked", prototypeCheckedTicks},
		{"asm", prototypeASMTicks},
		{"atomic", prototypeAtomicTicks},
		{"asm-fallback", func() uint64 { return prototypeASMTicksIf(false) }},
		{"atomic-fallback", func() uint64 { return prototypeAtomicTicksIf(false) }},
	}
	for _, reader := range readers {
		t.Run(reader.name, func(t *testing.T) {
			// The library reads the same monotonic kernel slot. Checking a
			// bracket tolerates real scheduler updates between any two reads.
			for i := 0; i < 10000; i++ {
				before := readLibcTicks()
				got := reader.read()
				after := readLibcTicks()
				if got < before || got > after {
					t.Fatalf("sample %d outside libc bracket [%d, %d]", got, before, after)
				}
			}
			initial := reader.read()
			deadline := time.Now().Add(3 * time.Second)
			for reader.read() == initial {
				if time.Now().After(deadline) {
					t.Fatal("clock did not progress")
				}
				runtime.Gosched()
			}
		})
	}

	// Exercise the Go-held foreign pointer and assembly calls during GC,
	// goroutine stack growth, and concurrent kernel publications.
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				before := readLibcTicks()
				got := prototypeReadWithStackGrowth(12)
				after := readLibcTicks()
				if got < before || got > after {
					errs <- fmt.Errorf("concurrent sample %d outside [%d, %d]", got, before, after)
					return
				}
			}
		}()
	}
	for i := 0; i < 8; i++ {
		runtime.GC()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

//go:noinline
func prototypeReadWithStackGrowth(depth int) uint64 {
	var pad [1024]byte
	pad[depth] = byte(depth)
	var ticks uint64
	if depth == 0 {
		ticks = prototypeAtomicTicks()
	} else {
		ticks = prototypeReadWithStackGrowth(depth - 1)
	}
	runtime.KeepAlive(pad)
	return ticks
}

func TestPrototypeWallMapping(t *testing.T) {
	RefreshWallClock()
	for _, read := range []func() uint64{prototypeCheckedTicks, prototypeASMTicks, prototypeAtomicTicks} {
		for i := 0; i < 10000; i++ {
			before := UnixNano()
			correction := wallCorrection.Load()
			got := correctedUnixNano(Instant{ticks: read()}, correction)
			after := UnixNano()
			if got < before || got > after {
				t.Fatalf("wall sample %d outside current implementation [%d, %d]", got, before, after)
			}
		}
	}
}

func prototypeBenchmarkSupported(b *testing.B) {
	if !prototypeSupported {
		b.Skip("direct approximate time unavailable")
	}
	b.ReportAllocs()
	b.ResetTimer()
}

func BenchmarkPrototypeInstantLibc(b *testing.B) {
	prototypeBenchmarkSupported(b)
	for n := 0; n < b.N; n++ {
		instantSink = prototypeLibcInstant()
	}
}

func BenchmarkPrototypeSinceLibc(b *testing.B) {
	start := NowInstant()
	prototypeBenchmarkSupported(b)
	for n := 0; n < b.N; n++ {
		durationSink = prototypeLibcSince(start)
	}
}

func BenchmarkPrototypeUnixNanoLibc(b *testing.B) {
	prototypeBenchmarkSupported(b)
	for n := 0; n < b.N; n++ {
		unixNanoSink = prototypeLibcUnixNano()
	}
}

func BenchmarkPrototypeNowLibc(b *testing.B) {
	prototypeBenchmarkSupported(b)
	for n := 0; n < b.N; n++ {
		timeSink = prototypeLibcNow()
	}
}

func BenchmarkPrototypeInstantCheckedASM(b *testing.B) {
	prototypeBenchmarkSupported(b)
	for n := 0; n < b.N; n++ {
		instantSink = prototypeCheckedASMInstant()
	}
}

func BenchmarkPrototypeSinceCheckedASM(b *testing.B) {
	start := NowInstant()
	prototypeBenchmarkSupported(b)
	for n := 0; n < b.N; n++ {
		durationSink = prototypeCheckedASMSince(start)
	}
}

func BenchmarkPrototypeUnixNanoCheckedASM(b *testing.B) {
	prototypeBenchmarkSupported(b)
	for n := 0; n < b.N; n++ {
		unixNanoSink = prototypeCheckedASMUnixNano()
	}
}

func BenchmarkPrototypeNowCheckedASM(b *testing.B) {
	prototypeBenchmarkSupported(b)
	for n := 0; n < b.N; n++ {
		timeSink = prototypeCheckedASMNow()
	}
}

func BenchmarkPrototypeInstantOnceASM(b *testing.B) {
	prototypeBenchmarkSupported(b)
	for n := 0; n < b.N; n++ {
		instantSink = prototypeOnceASMInstant()
	}
}

func BenchmarkPrototypeSinceOnceASM(b *testing.B) {
	start := NowInstant()
	prototypeBenchmarkSupported(b)
	for n := 0; n < b.N; n++ {
		durationSink = prototypeOnceASMSince(start)
	}
}

func BenchmarkPrototypeUnixNanoOnceASM(b *testing.B) {
	prototypeBenchmarkSupported(b)
	for n := 0; n < b.N; n++ {
		unixNanoSink = prototypeOnceASMUnixNano()
	}
}

func BenchmarkPrototypeNowOnceASM(b *testing.B) {
	prototypeBenchmarkSupported(b)
	for n := 0; n < b.N; n++ {
		timeSink = prototypeOnceASMNow()
	}
}

func BenchmarkPrototypeInstantOnceAtomic(b *testing.B) {
	prototypeBenchmarkSupported(b)
	for n := 0; n < b.N; n++ {
		instantSink = prototypeOnceAtomicInstant()
	}
}

func BenchmarkPrototypeSinceOnceAtomic(b *testing.B) {
	start := NowInstant()
	prototypeBenchmarkSupported(b)
	for n := 0; n < b.N; n++ {
		durationSink = prototypeOnceAtomicSince(start)
	}
}

func BenchmarkPrototypeUnixNanoOnceAtomic(b *testing.B) {
	prototypeBenchmarkSupported(b)
	for n := 0; n < b.N; n++ {
		unixNanoSink = prototypeOnceAtomicUnixNano()
	}
}

func BenchmarkPrototypeNowOnceAtomic(b *testing.B) {
	prototypeBenchmarkSupported(b)
	for n := 0; n < b.N; n++ {
		timeSink = prototypeOnceAtomicNow()
	}
}
