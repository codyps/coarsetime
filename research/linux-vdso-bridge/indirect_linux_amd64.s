#include "textflag.h"

// Match runtime.asmcgocall's ABI0 stack arguments exactly. Tail-jumping preserves
// the original return PC and avoids an extra frame around the runtime switch.
TEXT ·indirectCall(SB),NOSPLIT|NOFRAME,$0-20
    MOVQ ·runtimeBridge(SB), AX
    JMP AX

TEXT ·trampolineAddress(SB),NOSPLIT,$0-8
    MOVQ $clockTrampoline<>(SB), AX
    MOVQ AX, ret+0(FP)
    RET

// C ABI: argument is {fn, clock ID, timespec}. vDSO cannot callback into Go.
TEXT clockTrampoline<>(SB),NOSPLIT|NOFRAME,$0
    MOVQ 0(DI), AX
    LEAQ 16(DI), SI
    MOVL 8(DI), DI
    JMP AX
