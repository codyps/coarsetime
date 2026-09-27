//go:build !purego

#include "textflag.h"

// XNU: commpage base 0xfffffc000; calendar +0x120, approximate +0xc0,
// approximate support +0xc8. Ordered data reads use Go atomics, not this stub.
TEXT ·darwinWallAddresses(SB),NOSPLIT,$0-17
	MOVD $0x0000000fffffc120, R0
	MOVD R0, page+0(FP)
	MOVD $0x0000000fffffc0c0, R0
	MOVD R0, approximate+8(FP)
	MOVBU 8(R0), R0
	CMP $0, R0
	CSET NE, R0
	MOVB R0, supported+16(FP)
	RET
