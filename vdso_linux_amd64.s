//go:build !purego

#include "textflag.h"

TEXT ·coarseTrampolineAddress(SB),NOSPLIT,$0-8
 MOVQ $coarseClock<>(SB), AX
 MOVQ AX, ret+0(FP)
 RET

// C ABI: args = {fn uintptr; ts {sec, nsec int64}}.
// Tail-call fn(CLOCK_MONOTONIC_COARSE, &args.ts) on the system stack.
TEXT coarseClock<>(SB),NOSPLIT|NOFRAME,$0
 MOVQ 0(DI), AX
 LEAQ 8(DI), SI
 MOVL $6, DI
 JMP AX

TEXT ·realtimeTrampolineAddress(SB),NOSPLIT,$0-8
 MOVQ $realtimeClock<>(SB), AX
 MOVQ AX, ret+0(FP)
 RET

// Same C ABI, with CLOCK_REALTIME_COARSE.
TEXT realtimeClock<>(SB),NOSPLIT|NOFRAME,$0
 MOVQ 0(DI), AX
 LEAQ 8(DI), SI
 MOVL $5, DI
 JMP AX
