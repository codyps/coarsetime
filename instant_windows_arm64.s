//go:build !purego

#include "textflag.h"

// Read the aligned InterruptTime LowPart and High1Time in one load, matching
// https://go.dev/src/runtime/time_windows_arm64.s. No dependent data is read,
// so this clock snapshot needs no additional memory barrier.
TEXT ·readTicks(SB),NOSPLIT,$0-8
	MOVD $0x7ffe0008, R0
	MOVD (R0), R0
	MOVD R0, ret+0(FP)
	RET
