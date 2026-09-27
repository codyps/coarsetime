//go:build !purego

package coarsetime

// Windows interrupt time counts 100 ns intervals and includes suspend time.
// Keep native ticks in Instant; Sub and Since perform the conversion.
const clockNumer, clockDenom = uint64(100), uint64(1)

// readTicks reads KUSER_SHARED_DATA.InterruptTime without a Windows API call.
// This uses the same shared-page layout and atomic 64-bit read as Go's runtime:
// https://go.dev/src/runtime/time_windows.h
// https://go.dev/src/runtime/sys_windows_amd64.s
//
// Clock semantics (100 ns units, not resolution; unaffected by wall adjustments):
// https://learn.microsoft.com/en-us/windows/win32/api/realtimeapiset/nf-realtimeapiset-queryinterrupttime
// https://learn.microsoft.com/en-us/windows/win32/sysinfo/interrupt-time
//
// The layout dependency is limited to amd64. Other architectures need separate
// alignment, atomicity, and memory-ordering review. The purego tag avoids it.
func readTicks() uint64
