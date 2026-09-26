//go:build linux && linuxwallprobe

#include "textflag.h"

TEXT ·wallProbeNoopAddress(SB),NOSPLIT,$0-8
 MOVQ $wallProbeNoop<>(SB), AX
 MOVQ AX, ret+0(FP)
 RET

// C ABI. Same output shape as clock_gettime, with no kernel data access.
TEXT wallProbeNoop<>(SB),NOSPLIT|NOFRAME,$0
 MOVQ $1700000000, 8(DI)
 MOVQ $123456789, 16(DI)
 XORL AX, AX
 RET
