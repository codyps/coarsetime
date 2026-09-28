//go:build !purego

#include "textflag.h"

// Read SystemTime LowPart and High1Time together using the same 64-bit load
// as https://go.dev/src/runtime/time_windows_arm64.s. The field is only
// 4-byte aligned, but lies within one cache line; this follows Windows/arm64's
// shared-clock contract, not a general assumption about unaligned ARM loads.
// No dependent data is read, so no additional memory barrier is needed.
TEXT ·readWindowsFiletime(SB),NOSPLIT,$0-8
	MOVD $0x7ffe0014, R0
	MOVD (R0), R0
	MOVD R0, ret+0(FP)
	RET
