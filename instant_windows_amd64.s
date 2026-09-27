//go:build !purego

#include "textflag.h"

// InterruptTime is 8-byte aligned. Read LowPart and High1Time atomically,
// following https://go.dev/src/runtime/time_windows.h.
TEXT ·readTicks(SB),NOSPLIT,$0-8
	MOVQ $0x7ffe0008, AX
	MOVQ (AX), AX
	MOVQ AX, ret+0(FP)
	RET
