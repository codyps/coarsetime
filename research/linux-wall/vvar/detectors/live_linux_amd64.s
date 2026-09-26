#include "textflag.h"
TEXT ·mappedAddress(SB),NOSPLIT,$0-16
 MOVQ addr+0(FP), AX
 MOVQ AX, ret+8(FP)
 RET
