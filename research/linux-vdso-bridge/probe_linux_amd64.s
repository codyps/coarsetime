#include "textflag.h"

TEXT ·stackProbeAddress(SB),NOSPLIT,$0-8
    MOVQ $stackProbe<>(SB), AX
    MOVQ AX, ret+0(FP)
    RET

// Synthetic C ABI leaf: confirm the system stack and alignment, touch 16 KiB
// of stack, and return a timespec. No Go callback, scheduler call, or allocation.
// The g.m, m.g0 and g.stack prefix offsets are shared by tested Go versions.
TEXT stackProbe<>(SB),NOSPLIT|NOFRAME,$0
    MOVQ TLS, CX
    MOVQ 0(CX)(TLS*1), AX
    MOVQ 48(AX), DX
    MOVQ 0(DX), DX
    CMPQ SP, 0(DX)
    JLS bad
    CMPQ SP, 8(DX)
    JAE bad
    MOVQ SP, AX
    ANDQ $15, AX
    CMPQ AX, $8
    JNE bad
    SUBQ $16384, SP
    MOVQ $0, 0(SP)
    MOVQ $0, 4096(SP)
    MOVQ $0, 8192(SP)
    MOVQ $0, 12288(SP)
    ADDQ $16384, SP
    MOVQ $123, 0(SI)
    MOVQ $456, 8(SI)
    MOVL $0, AX
    RET
bad:
    MOVL $-999, AX
    RET
