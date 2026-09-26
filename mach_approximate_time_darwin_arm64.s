//go:build !purego

#include "textflag.h"

TEXT ·approximateTimeAddress(SB),NOSPLIT,$0-8
 MOVD $approximateTime<>(SB), R0
 MOVD R0, ret+0(FP)
 RET

// C ABI: entered on the system stack by the runtime's libc bridge.
TEXT approximateTime<>(SB),NOSPLIT|NOFRAME,$0
 JMP libc_mach_approximate_time(SB)

TEXT ·timebaseInfoAddress(SB),NOSPLIT,$0-8
 MOVD $timebaseInfo<>(SB), R0
 MOVD R0, ret+0(FP)
 RET

TEXT timebaseInfo<>(SB),NOSPLIT|NOFRAME,$0
 JMP libc_mach_timebase_info(SB)
