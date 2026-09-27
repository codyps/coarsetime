package bridge

import (
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

type clockReader func(int32) (int64, int64, int32)

func readers(t testing.TB) map[string]clockReader {
	t.Helper()
	if clockEntry == 0 {
		t.Fatal("vDSO unavailable; this experiment requires actual vDSO calls")
	}
	if runtimeBridgeError != nil {
		t.Fatal(runtimeBridgeError)
	}
	result := map[string]clockReader{"indirect": Indirect}
	if dedicatedAvailable {
		result["dedicated"] = Dedicated
	}
	return result
}

func syscallClock(id int32) int64 {
	var ts syscall.Timespec
	_, _, err := syscall.RawSyscall(syscall.SYS_CLOCK_GETTIME, uintptr(id), uintptr(unsafe.Pointer(&ts)), 0)
	if err != 0 {
		panic(err)
	}
	return ts.Sec*1e9 + ts.Nsec
}

func TestBrackets(t *testing.T) {
	t.Logf("%s: vDSO=%#x runtime.asmcgocall=%#x dedicated=%v", runtime.Version(), clockEntry, runtimeBridge, dedicatedAvailable)
	for name, read := range readers(t) {
		t.Run(name, func(t *testing.T) {
			for _, id := range []int32{5, 6} {
				for n := 0; n < 100000; n++ {
					lo := syscallClock(id)
					sec, nsec, result := read(id)
					hi := syscallClock(id)
					got := sec*1e9 + nsec
					if result != 0 || nsec < 0 || nsec >= 1e9 || (hi >= lo && (got < lo || got > hi)) {
						t.Fatalf("clock %d result %d timestamp %d outside [%d,%d]", id, result, got, lo, hi)
					}
				}
			}
			_, _, result := read(-12345)
			if result != -int32(syscall.EINVAL) {
				t.Fatalf("invalid clock: %d", result)
			}
		})
	}
}

//go:noinline
func readDeep(read clockReader, depth int) int64 {
	var pad [2048]byte
	pad[depth] = byte(depth)
	var value int64
	if depth > 0 {
		value = readDeep(read, depth-1)
	} else {
		s, ns, result := read(6)
		if result != 0 {
			panic("vDSO read failed")
		}
		value = s*1e9 + ns
	}
	runtime.KeepAlive(&pad)
	return value + int64(pad[depth]) - int64(depth)
}

func TestStress(t *testing.T) {
	for name, read := range readers(t) {
		t.Run(name, func(t *testing.T) {
			profile, err := os.CreateTemp(t.TempDir(), "cpu-*.pprof")
			if err != nil {
				t.Fatal(err)
			}
			defer profile.Close()
			if err := pprof.StartCPUProfile(profile); err != nil {
				t.Fatal(err)
			}
			defer pprof.StopCPUProfile()
			var stop atomic.Bool
			var wg sync.WaitGroup
			for g := 0; g < 8; g++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					previous := int64(0)
					for n := 0; !stop.Load(); n++ {
						var now int64
						if n%1000 == 0 {
							now = readDeep(read, 32)
						} else {
							s, ns, result := read(6)
							if result != 0 {
								panic("read failed")
							}
							now = s*1e9 + ns
						}
						if now < previous {
							t.Error("clock went backwards")
							return
						}
						previous = now
					}
				}()
			}
			deadline := time.Now().Add(750 * time.Millisecond)
			for time.Now().Before(deadline) {
				runtime.GC()
				// Force runtime unwinding as well as SIGPROF during reads.
				runtime.Stack(make([]byte, 65536), true)
				runtime.Gosched()
			}
			stop.Store(true)
			wg.Wait()
		})
	}
}

var sink int64

func BenchmarkResolveRuntimeBridge(b *testing.B) {
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		pc, err := resolveRuntimeBridge()
		if err != nil || pc == 0 {
			b.Fatalf("resolve: %x %v", pc, err)
		}
	}
}

func BenchmarkRead(b *testing.B) {
	for name, read := range readers(b) {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for n := 0; n < b.N; n++ {
				s, ns, result := read(5)
				if result != 0 {
					b.Fatal(result)
				}
				sink = s*1e9 + ns
			}
		})
	}
	b.Run("syscall", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			sink = syscallClock(5)
		}
	})
}
