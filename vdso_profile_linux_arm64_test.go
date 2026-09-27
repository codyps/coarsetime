//go:build !purego

package coarsetime

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime/pprof"
	"strings"
	"testing"
	"time"
)

//go:noinline
func profileVDSOReads() {
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		for i := 0; i < 4096; i++ {
			instantSink = NowInstant()
			timeSink = Now()
			durationSink = time.Duration(UnixNano())
		}
	}
}

func TestVDSOProfileAttribution(t *testing.T) {
	if coarseVDSO == 0 {
		t.Skip("vDSO unavailable")
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("Go toolchain required to inspect CPU profile")
	}
	path := filepath.Join(t.TempDir(), "cpu.pprof")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := pprof.StartCPUProfile(f); err != nil {
		t.Skipf("CPU profiler already active: %v", err)
	}
	profileVDSOReads()
	pprof.StopCPUProfile()
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(goTool, "tool", "pprof", "-raw", path).CombinedOutput()
	if err != nil {
		t.Fatalf("decode profile: %v\n%s", err, out)
	}
	profile := string(out)
	if !strings.Contains(profile, "profileVDSOReads") {
		t.Fatalf("no reader samples in profile:\n%s", out)
	}
	if strings.Contains(profile, "runtime._VDSO") {
		t.Fatalf("vDSO samples lost their Go caller:\n%s", out)
	}
}
