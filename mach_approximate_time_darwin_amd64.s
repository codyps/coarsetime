

TEXT runtime·mach_approximate_time_trampoline(SB),NOSPLIT,$0
	CALL	libc_mach_approximate_time(SB)
	RET
