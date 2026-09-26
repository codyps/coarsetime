#include "textflag.h"

TEXT ·sharedFiletime(SB),NOSPLIT,$0-8
	MOVQ $0x7ffe0014, AX
	MOVQ (AX), AX
	MOVQ AX, ret+0(FP)
	RET

TEXT ·sharedInterrupt(SB),NOSPLIT,$0-8
	MOVQ $0x7ffe0008, AX
	MOVQ (AX), AX
	MOVQ AX, ret+0(FP)
	RET

TEXT ·sharedUnixNano(SB),NOSPLIT,$0-8
	MOVQ $0x7ffe0014, AX
	MOVQ (AX), AX
	MOVQ $116444736000000000, CX
	SUBQ CX, AX
	IMULQ $100, AX
	MOVQ AX, ret+0(FP)
	RET
