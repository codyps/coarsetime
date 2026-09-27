//go:build !purego && darwin && (amd64 || arm64)

package coarsetime

import (
	"syscall"
	"unsafe"
)

// These symbols are resolved by the Go linker; cgo is not required.
//go:cgo_import_dynamic libc_mach_approximate_time mach_approximate_time "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic libc_mach_timebase_info mach_timebase_info "/usr/lib/libSystem.B.dylib"

// rawSyscall is the internal libc function-pointer bridge used by x/sys/unix.
// Unlike syscall.RawSyscall (a kernel trap), it switches to the system stack
// through runtime.libcCall, without entersyscall/exitsyscall.
//
//go:linkname rawSyscall syscall.rawSyscall
func rawSyscall(fn, a1, a2, a3 uintptr) (r1, r2 uintptr, err syscall.Errno)

func approximateTimeAddress() uintptr
func timebaseInfoAddress() uintptr

var approximateTimePC = approximateTimeAddress()

// Global storage keeps the native output pointer valid across the bridge.
var timebase struct{ numer, denom uint32 }
var clockNumer, clockDenom = initTimebase()

func initTimebase() (uint64, uint64) {
	r, _, _ := rawSyscall(timebaseInfoAddress(), uintptr(unsafe.Pointer(&timebase)), 0, 0)
	if r != 0 || timebase.numer == 0 || timebase.denom == 0 {
		panic("coarsetime: mach_timebase_info failed")
	}
	return uint64(timebase.numer), uint64(timebase.denom)
}

func readLibcTicks() uint64 {
	r, _, _ := rawSyscall(approximateTimePC, 0, 0, 0)
	// Mach returns an unconditional uint64, not an errno-bearing result.
	// The bridge may interpret low bits of -1 as an error; ignore that errno.
	return uint64(r)
}
