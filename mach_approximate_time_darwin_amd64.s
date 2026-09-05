#include "textflag.h"

TEXT ·approximateTimeAddress(SB),NOSPLIT,$0-8
 MOVQ $approximateTime<>(SB), AX
 MOVQ AX, ret+0(FP)
 RET

// C ABI: entered on the system stack by the runtime's libc bridge.
TEXT approximateTime<>(SB),NOSPLIT|NOFRAME,$0
 JMP libc_mach_approximate_time(SB)

TEXT ·timebaseInfoAddress(SB),NOSPLIT,$0-8
 MOVQ $timebaseInfo<>(SB), AX
 MOVQ AX, ret+0(FP)
 RET

TEXT timebaseInfo<>(SB),NOSPLIT|NOFRAME,$0
 JMP libc_mach_timebase_info(SB)

// XNU xnu-12377.1.9 osfmk/i386/cpu_capabilities.h:
// _COMM_PAGE64_BASE_ADDRESS = 0x7fffffe00000
// APPROX_TIME = base + 0x080; APPROX_TIME_SUPPORTED = base + 0x088.
// Support is checked once at initialization. Retain the libc trampolines above
// for unsupported approximate clocks and for timebase initialization.
TEXT ·commpageApproximateTimeSupported(SB),NOSPLIT,$0-1
 MOVQ $0x00007fffffe00088, AX
 CMPB (AX), $0
 SETNE ret+0(FP)
 RET

TEXT ·commpageApproximateTimeAddress(SB),NOSPLIT,$0-8
 MOVQ $0x00007fffffe00080, AX
 MOVQ AX, ret+0(FP)
 RET
