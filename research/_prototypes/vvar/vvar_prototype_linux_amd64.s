//go:build vvarprototype

#include "textflag.h"

TEXT ·vvarPrototypeAddress(SB),NOSPLIT,$0-16
 MOVQ base+0(FP), AX
 MOVQ AX, ret+8(FP)
 RET

// Ordinary amd64 loads preserve load/load order; no C ABI or stack switch.
// Eight attempts bound retries, including the permanently odd TIMENS page.
TEXT ·vvarPrototypeASM(SB),NOSPLIT,$0-25
 MOVQ p+0(FP), DI
 TESTQ DI, DI
 JZ fail
 MOVL $8, CX
retry:
 MOVL 0(DI), AX
 TESTL $1, AX
 JNZ next
 MOVQ 120(DI), R8
 MOVQ 128(DI), R9
 CMPL AX, 0(DI)
 JNE next
 CMPQ R9, $1000000000
 JAE next
 MOVQ R8, sec+8(FP)
 MOVQ R9, nsec+16(FP)
 MOVB $1, ok+24(FP)
 RET
next:
 PAUSE
 DECL CX
 JNZ retry
fail:
 MOVQ $0, sec+8(FP)
 MOVQ $0, nsec+16(FP)
 MOVB $0, ok+24(FP)
 RET
