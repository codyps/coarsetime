//go:cgo_import_dynamic libc_mach_approximate_time mach_approximate_time "/usr/lib/libSystem.B.dylib"

func Now() time.Time {
	// call `mach_absolute_time` to get the current time.
	machTime := mach_absolute_time_trampoline()

	time.T
}
