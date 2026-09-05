//go:build commpageprototype

#include "textflag.h"

// _COMM_PAGE64_BASE_ADDRESS + APPROX_TIME / APPROX_TIME_SUPPORTED.
#define APPROX 0x00007fffffe00080
#define SUPPORTED 0x00007fffffe00088

TEXT ·prototypeApproxSupported(SB),NOSPLIT,$0-1
 MOVQ $SUPPORTED, AX
 CMPB (AX), $0
 SETNE ret+0(FP)
 RET

TEXT ·prototypeApproxAddress(SB),NOSPLIT,$0-8
 MOVQ $APPROX, AX
 MOVQ AX, ret+0(FP)
 RET

TEXT ·prototypeApproxLoad(SB),NOSPLIT,$0-8
 MOVQ $APPROX, AX
 MOVQ (AX), AX
 MOVQ AX, ret+0(FP)
 RET

TEXT ·prototypeApproxCheckedLoad(SB),NOSPLIT,$0-8
 MOVQ $APPROX, CX
 XORQ AX, AX
 CMPB 8(CX), $0
 JE unsupported
 MOVQ (CX), AX
unsupported:
 MOVQ AX, ret+0(FP)
 RET
