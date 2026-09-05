package coarsetime

// Retain libSystem's reader until direct commpage access is validated on ARM64.
func readTicks() uint64 { return readLibcTicks() }
