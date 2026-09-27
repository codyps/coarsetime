//go:build !purego

#include "textflag.h"

// Preserve the ABI0 frame and return PC across the runtime stack switch.
TEXT ·asmcgocall(SB),NOSPLIT|NOFRAME,$0-20
	MOVD ·runtimeAsmcgocall(SB), R16
	B (R16)

TEXT ·coarseTrampolineAddress(SB),NOSPLIT,$0-8
	MOVD $coarseClock<>(SB), R0
	MOVD R0, ret+0(FP)
	RET

TEXT ·realtimeTrampolineAddress(SB),NOSPLIT,$0-8
	MOVD $realtimeClock<>(SB), R0
	MOVD R0, ret+0(FP)
	RET

TEXT coarseClock<>(SB),NOSPLIT|NOFRAME,$0
	MOVD $6, R2 // CLOCK_MONOTONIC_COARSE
	B vdsoClock<>(SB)

TEXT realtimeClock<>(SB),NOSPLIT|NOFRAME,$0
	MOVD $5, R2 // CLOCK_REALTIME_COARSE
	B vdsoClock<>(SB)

// C ABI, already on the runtime system stack. Preserve C callee-save registers,
// including R27 (the Go assembler's scratch register for global loads).
TEXT vdsoClock<>(SB),NOSPLIT,$48
	STP (R19, R20), 8(RSP)
	MOVD R27, 24(RSP)
	MOVD ZR, R19
	MOVD ·runtimeGMOffset(SB), R9
	MOVD (g)(R9), R10
	MOVD ·runtimeMSignalOffset(SB), R9
	MOVD (R10)(R9), R11
	CBZ R11, call
	CMP g, R11
	BEQ call
	// g.stack.lo is the first word of g (also part of runtime/cgo's ABI).
	MOVD (R11), R19
	MOVD (R19), R20
	// sigFetchG recovers this g if a signal interrupts the vDSO without cgo.
	MOVD g, (R19)
call:
	MOVD (R0), R9
	ADD $8, R0, R1
	MOVD R2, R0
	BL (R9)
	CBZ R19, done
	MOVD R20, (R19)
done:
	LDP 8(RSP), (R19, R20)
	MOVD 24(RSP), R27
	RET
