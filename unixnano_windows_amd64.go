//go:build !purego

package coarsetime

// Windows FILETIME counts 100 ns intervals since 1601-01-01 UTC.
const windowsUnixEpoch = 116444736000000000

func unixNano() int64 { return filetimeUnixNano(readWindowsFiletime()) }

func filetimeUnixNano(ticks uint64) int64 {
	// Convert to signed before multiplying to support dates before 1970.
	return int64(ticks-windowsUnixEpoch) * 100
}

// readWindowsFiletime reads KUSER_SHARED_DATA.SystemTime. This OS layout
// dependency follows Go's runtime/time_windows.h and time_windows_amd64.s.
// It must not be reused on another architecture without reviewing alignment,
// atomicity, and memory ordering. Use the purego tag to avoid this dependency.
func readWindowsFiletime() uint64
