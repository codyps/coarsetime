//go:build !purego

#include "textflag.h"

// XNU: commpage base 0x7fffffe00000; calendar +0xd0, approximate +0x80,
// approximate support +0x88. Only pointers and the capability are read here.
TEXT ·darwinWallAddresses(SB),NOSPLIT,$0-17
	MOVQ $0x00007fffffe000d0, AX
	MOVQ AX, page+0(FP)
	MOVQ $0x00007fffffe00080, AX
	MOVQ AX, approximate+8(FP)
	CMPB 8(AX), $0
	SETNE supported+16(FP)
	RET
