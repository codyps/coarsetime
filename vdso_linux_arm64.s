//go:build !purego

#include "textflag.h"
#include "funcdata.h"

// Publish the original goroutine and its caller before asmcgocall switches g
// and SP to g0. This gives SIGPROF the user stack even inside the vDSO.
TEXT ·asmcgocall(SB),NOSPLIT,$80-20
	NO_LOCAL_POINTERS
	MOVD fn+0(FP), R0
	MOVD arg+8(FP), R1
	MOVD R0, 8(RSP)
	MOVD R1, 16(RSP)
	MOVD ·runtimeGMOffset(SB), R9
	MOVD (g)(R9), R8
	MOVD R8, 64(RSP)
	MOVD ·runtimeVDSOPCOffset(SB), R9
	MOVD ·runtimeVDSOSPOffset(SB), R10
	MOVD R9, 72(RSP)
	MOVD R10, 80(RSP)
	MOVD (R8)(R9), R2
	MOVD (R8)(R10), R3
	MOVD R2, 32(RSP)
	MOVD R3, 40(RSP)
	MOVD $fn-8(FP), R2 // caller's SP
	MOVD LR, (R8)(R9)
	MOVD R2, (R8)(R10)
	MOVD ZR, 48(RSP)
	MOVD ·runtimeMSignalOffset(SB), R9
	MOVD (R8)(R9), R11
	CBZ R11, call
	CMP g, R11
	BEQ call
	// g.stack.lo is the first word of g (also part of runtime/cgo's ABI).
	MOVD (R11), R11
	MOVD (R11), R12
	MOVD R11, 48(RSP)
	MOVD R12, 56(RSP)
	MOVD g, (R11)
call:
	MOVD ·runtimeAsmcgocall(SB), R16
	CALL (R16)
	MOVD 48(RSP), R11
	CBZ R11, restore
	MOVD 56(RSP), R12
	MOVD R12, (R11)
restore:
	MOVD 64(RSP), R8
	MOVD 72(RSP), R9
	MOVD 80(RSP), R10
	MOVD 40(RSP), R3
	MOVD R3, (R8)(R10)
	MOVD 32(RSP), R2
	MOVD R2, (R8)(R9)
	MOVW 24(RSP), R0
	MOVW R0, ret+16(FP)
	RET

TEXT ·coarseTrampolineAddress(SB),NOSPLIT,$0-8
	MOVD $coarseClock<>(SB), R0
	MOVD R0, ret+0(FP)
	RET

TEXT ·realtimeTrampolineAddress(SB),NOSPLIT,$0-8
	MOVD $realtimeClock<>(SB), R0
	MOVD R0, ret+0(FP)
	RET

// C ABI: args = {fn uintptr; ts {sec, nsec int64}}. Tail calls preserve
// all C callee-save registers and the system stack selected by the runtime.
TEXT coarseClock<>(SB),NOSPLIT|NOFRAME,$0
	MOVD (R0), R9
	ADD $8, R0, R1
	MOVD $6, R0 // CLOCK_MONOTONIC_COARSE
	B (R9)

TEXT realtimeClock<>(SB),NOSPLIT|NOFRAME,$0
	MOVD (R0), R9
	ADD $8, R0, R1
	MOVD $5, R0 // CLOCK_REALTIME_COARSE
	B (R9)
