//go:build !purego

#include "textflag.h"

// Read LowPart and High1Time together, as Go's Windows/amd64 runtime does.
// The 8-byte field at offset 0x14 lies within a single cache line.
TEXT ·readWindowsFiletime(SB),NOSPLIT,$0-8
	MOVQ $0x7ffe0014, AX
	MOVQ (AX), AX
	MOVQ AX, ret+0(FP)
	RET
