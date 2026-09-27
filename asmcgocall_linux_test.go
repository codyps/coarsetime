//go:build !purego && linux && (amd64 || arm64)

package coarsetime

import (
	"debug/elf"
	"io"
	"reflect"
	"runtime"
	"runtime/pprof"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAsmcgocallResolution(t *testing.T) {
	f, err := elf.Open("/proc/self/exe")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	anchor := reflect.ValueOf(runtime.Gosched).Pointer()
	pc, err := resolveAsmcgocall(f, anchor)
	if err != nil || pc == 0 || pc != runtimeAsmcgocall {
		t.Fatalf("resolve = %#x, %v; initialized entry = %#x", pc, err, runtimeAsmcgocall)
	}
	fn := runtime.FuncForPC(pc)
	file, _ := fn.FileLine(pc)
	if fn.Name() != "runtime.asmcgocall" || !strings.HasSuffix(file, "runtime/asm_"+runtime.GOARCH+".s") {
		t.Fatalf("resolved %s in %s instead of the ABI0 assembly implementation", fn.Name(), file)
	}
	if pc, err := resolveAsmcgocall(f, 0); pc != 0 || err == nil {
		t.Fatalf("invalid anchor accepted: %#x, %v", pc, err)
	}

	// Exercise the alternate section spelling even in a non-PIE test binary.
	copyFile := *f
	copyFile.Sections = append([]*elf.Section(nil), f.Sections...)
	for i, section := range copyFile.Sections {
		if section.Name == ".gopclntab" {
			copySection := *section
			copySection.Name = ".data.rel.ro.gopclntab"
			copyFile.Sections[i] = &copySection
		}
	}
	if pc, err := resolveAsmcgocall(&copyFile, anchor); err != nil || pc != runtimeAsmcgocall {
		t.Fatalf("alternate pclntab section: %#x, %v", pc, err)
	}
}

func TestAsmcgocallMissingMetadata(t *testing.T) {
	for _, f := range []*elf.File{
		{},
		{Sections: []*elf.Section{{SectionHeader: elf.SectionHeader{Name: ".text"}}}},
	} {
		if pc, err := resolveAsmcgocall(f, reflect.ValueOf(runtime.Gosched).Pointer()); pc != 0 || err == nil {
			t.Fatalf("missing metadata accepted: %#x, %v", pc, err)
		}
	}
}

//go:noinline
func readVDSOWithStackGrowth(depth int) uint64 {
	var pad [2048]byte
	pad[depth] = byte(depth)
	var ticks uint64
	if depth > 0 {
		ticks = readVDSOWithStackGrowth(depth - 1)
	} else {
		ticks = NowInstant().ticks
		_ = Now()
		_ = UnixNano()
	}
	runtime.KeepAlive(&pad)
	return ticks + uint64(pad[depth]) - uint64(depth)
}

func TestVDSOConcurrentProfiling(t *testing.T) {
	if coarseVDSO == 0 {
		t.Skip("vDSO unavailable")
	}
	if err := pprof.StartCPUProfile(io.Discard); err != nil {
		t.Skipf("cannot start an additional CPU profile: %v", err)
	}
	defer pprof.StopCPUProfile()
	var stop atomic.Bool
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var previous uint64
			for n := 0; !stop.Load(); n++ {
				current := NowInstant().ticks
				if n%1000 == 0 {
					current = readVDSOWithStackGrowth(32)
				}
				if current < previous {
					t.Error("clock moved backwards")
					return
				}
				previous = current
			}
		}()
	}
	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		runtime.GC()
		runtime.Stack(make([]byte, 65536), true)
		runtime.Gosched()
	}
	stop.Store(true)
	wg.Wait()
}
