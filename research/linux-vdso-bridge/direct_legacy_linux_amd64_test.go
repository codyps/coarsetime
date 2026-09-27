//go:build !go1.27

package bridge

import "unsafe"

// Match the production library's original binding and noescape declaration.
//
//go:linkname directAsmcgocall runtime.asmcgocall
//go:noescape
func directAsmcgocall(fn, arg unsafe.Pointer) int32

func directRead(id int32) (int64, int64, int32) {
	args := clockArgs{fn: clockEntry, id: int64(id)}
	result := directAsmcgocall(trampoline, unsafe.Pointer(&args))
	return args.ts.Sec, args.ts.Nsec, result
}

func directComparison() []comparisonCase {
	return []comparisonCase{{"direct", directRead}}
}
