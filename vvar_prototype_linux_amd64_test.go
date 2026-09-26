//go:build vvarprototype

package coarsetime

import (
	"os"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"
)

func TestVvarPrototype(t *testing.T) {
	if unsafe.Offsetof(vvarPrototypeClock{}.sec) != 120 || unsafe.Offsetof(vvarPrototypeClock{}.nsec) != 128 {
		t.Fatal("layout")
	}
	if vvarPrototypePointer == nil {
		if os.Getenv("COARSETIME_VVAR_LAYOUT") != "" {
			t.Fatal("opted-in VVAR probe unavailable")
		}
		t.Skip("VVAR probe not enabled")
	}
	t.Log("direct VVAR reader active; layout selected or recognized")
	for n := 0; n < 100000; n++ {
		before := UnixNano()
		a, b := vvarPrototypeNow(), vvarPrototypeASMNow()
		c, d := vvarPrototypeUnixNano(), vvarPrototypeASMUnixNano()
		after := UnixNano()
		if after < before {
			continue
		}
		for _, got := range []int64{a.UnixNano(), b.UnixNano(), c, d} {
			if got < before || got > after {
				t.Fatalf("%d outside [%d,%d]", got, before, after)
			}
		}
		if a != time.Unix(0, a.UnixNano()) || b != time.Unix(0, b.UnixNano()) {
			t.Fatal("time representation")
		}
		if n%10000 == 0 {
			runtime.GC()
		}
	}
}
func TestVvarPrototypeRetryAndFallback(t *testing.T) {
	readers := []func(*vvarPrototypeClock) (int64, int64, bool){vvarPrototypeRead, vvarPrototypeASM}
	for _, read := range readers {
		for _, p := range []*vvarPrototypeClock{nil, {seq: 1}, {seq: 2, nsec: 1e9}} {
			if _, _, ok := read(p); ok {
				t.Fatal("invalid snapshot accepted")
			}
		}
		p := &vvarPrototypeClock{seq: 2, sec: 1700000000, nsec: 123}
		if s, ns, ok := read(p); !ok || s != 1700000000 || ns != 123 {
			t.Fatal("valid snapshot rejected")
		}
	}
	saved := vvarPrototypePointer
	defer func() { vvarPrototypePointer = saved }()
	for _, p := range []*vvarPrototypeClock{nil, {seq: 1}} {
		vvarPrototypePointer = p
		before := UnixNano()
		a, b := vvarPrototypeNow().UnixNano(), vvarPrototypeASMNow().UnixNano()
		after := UnixNano()
		if after >= before && (a < before || a > after || b < before || b > after) {
			t.Fatal("fallback bracket")
		}
	}
}
func TestVvarPrototypeConcurrentSequence(t *testing.T) {
	for _, read := range []func(*vvarPrototypeClock) (int64, int64, bool){vvarPrototypeRead, vvarPrototypeASM} {
		p := &vvarPrototypeClock{sec: 1700000000}
		done := make(chan struct{})
		go func() {
			defer close(done)
			for n := uint64(1); n <= 100000; n++ {
				atomic.AddUint32(&p.seq, 1)
				atomic.StoreUint64(&p.sec, 1700000000+n)
				atomic.StoreUint64(&p.nsec, n)
				atomic.AddUint32(&p.seq, 1)
			}
		}()
		for n := 0; n < 100000; n++ {
			s, ns, ok := read(p)
			if ok && s-ns != 1700000000 {
				t.Errorf("torn snapshot %d %d", s, ns)
				break
			}
		}
		<-done
	}
}
func BenchmarkVvarCurrentNow(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		timeSink = Now()
	}
}
func BenchmarkVvarGoNow(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		timeSink = vvarPrototypeNow()
	}
}
func BenchmarkVvarASMNow(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		timeSink = vvarPrototypeASMNow()
	}
}
func BenchmarkVvarCurrentUnixNano(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		unixNanoSink = UnixNano()
	}
}
func BenchmarkVvarGoUnixNano(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		unixNanoSink = vvarPrototypeUnixNano()
	}
}
func BenchmarkVvarASMUnixNano(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		unixNanoSink = vvarPrototypeASMUnixNano()
	}
}
