//go:build !purego && linux && amd64

package coarsetime

import (
	"syscall"
	"unsafe"
)

// The local ABI0 trampoline tail-jumps to the runtime bridge resolved at startup.
// The runtime switches to an ABI-aligned system stack and restores Go's stack
// and g afterwards. vDSO functions cannot call back into Go. No private runtime
// struct offsets or static references to runtime.asmcgocall are used.
//
//go:noescape
func asmcgocall(fn, arg unsafe.Pointer) int32

func coarseTrampolineAddress() unsafe.Pointer
func realtimeTrampolineAddress() unsafe.Pointer

var coarseTrampoline = coarseTrampolineAddress()
var realtimeTrampoline = realtimeTrampolineAddress()

// The runtime resolves this during startup, before package initialization.
// Go permits this linkname for compatibility, but it remains a private API.
//
//go:linkname runtimeVDSOClockgettime runtime.vdsoClockgettimeSym
var runtimeVDSOClockgettime uintptr

// Keep a package-local copy so fallback tests never change the runtime's clock.
var coarseVDSO = runtimeVDSOClockgettime

func readCoarseVDSO(ts *syscall.Timespec) bool   { return readClockVDSO(ts, coarseTrampoline) }
func readRealtimeVDSO(ts *syscall.Timespec) bool { return readClockVDSO(ts, realtimeTrampoline) }

func readClockVDSO(ts *syscall.Timespec, trampoline unsafe.Pointer) bool {
	if coarseVDSO == 0 {
		return false
	}
	args := struct {
		fn uintptr
		ts syscall.Timespec
	}{fn: coarseVDSO}
	if asmcgocall(trampoline, unsafe.Pointer(&args)) != 0 {
		return false
	}
	*ts = args.ts
	return true
}
