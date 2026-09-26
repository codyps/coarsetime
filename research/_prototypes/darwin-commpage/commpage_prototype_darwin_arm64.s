//go:build commpageprototype

#include "textflag.h"

// _COMM_PAGE64_BASE_ADDRESS + APPROX_TIME / APPROX_TIME_SUPPORTED.
#define APPROX 0x0000000fffffc0c0
#define SUPPORTED 0x0000000fffffc0c8

TEXT ·prototypeApproxSupported(SB),NOSPLIT,$0-1
 MOVD $SUPPORTED, R0
 MOVBU (R0), R0
 CMP $0, R0
 CSET NE, R0
 MOVB R0, ret+0(FP)
 RET

TEXT ·prototypeApproxAddress(SB),NOSPLIT,$0-8
 MOVD $APPROX, R0
 MOVD R0, ret+0(FP)
 RET

TEXT ·prototypeApproxLoad(SB),NOSPLIT,$0-8
 MOVD $APPROX, R0
 MOVD (R0), R0
 MOVD R0, ret+0(FP)
 RET

TEXT ·prototypeApproxCheckedLoad(SB),NOSPLIT,$0-8
 MOVD $APPROX, R1
 MOVBU 8(R1), R0
 CBZ R0, unsupported
 MOVD (R1), R0
unsupported:
 MOVD R0, ret+0(FP)
 RET
