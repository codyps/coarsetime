package bridge

import (
	"testing"
	"unsafe"
)

func stackProbeAddress() uintptr

func TestSystemStack(t *testing.T) {
	readers(t)
	args := clockArgs{fn: stackProbeAddress(), id: 5}
	result := indirectCall(trampoline, unsafe.Pointer(&args))
	if result != 0 || args.ts.Sec != 123 || args.ts.Nsec != 456 {
		t.Fatalf("indirect stack probe: result=%d timespec=%+v", result, args.ts)
	}
	checkDedicatedStack(t)
}
