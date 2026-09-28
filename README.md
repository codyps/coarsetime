# coarsetime

Inexpensive functions for measuring elapsed time and obtaining approximate wall
time in Go. Uses coarse platform clocks where available, trading resolution and
freshness for speed.
Requires Go 1.23 or newer; cgo is not required.

```sh
go get github.com/codyps/coarsetime
```

## Usage

```go
import "github.com/codyps/coarsetime"

start := coarsetime.NowInstant()
// Do some work.
elapsed := coarsetime.Since(start) // time.Duration

stamp := coarsetime.Now()         // Approximate local time.Time.
unixNS := coarsetime.UnixNano()   // Same wall clock, as int64 nanoseconds.
```

Readings may repeat; short intervals may measure zero. There is no guaranteed
resolution or maximum staleness. Nanosecond units do not imply nanosecond accuracy.

Instants are process-local; their zero value is uninitialized. Use `Before` and
`After` to compare them, and `Sub` or `Since` to measure elapsed time.
`Sub` truncates fractional nanoseconds and saturates at `time.Duration` limits.

## Performance

[Continuous benchmark dashboard](https://codyps.github.io/coarsetime/) ·
[measurement protocol and PR reports](benchmarks/README.md).
CI records native Linux, macOS, and Windows results on amd64 and arm64, with
standard-library comparisons. PR reports measure base and head on the same worker.

The standard-library column provides the comparison for each operation:

| Operation | coarsetime | Standard library | Notes |
| --- | --- | --- | --- |
| Capture an elapsed-time start | `NowInstant()` | `time.Now()` | Only reads elapsed time. |
| Measure elapsed time | `Since(start)` | `time.Since(start)` | Reads elapsed time and subtracts the start. |
| Read a wall timestamp | `UnixNano()` | `time.Now().UnixNano()` | Avoids constructing a `time.Time`. |
| Read calendar time | `Now()` | `time.Now()` | Omits Go's monotonic component. |
| Subtract stored readings | `end.Sub(start)` | `end.Sub(start)` on `time.Time` | Neither reads a clock; not tracked in the continuous dashboard. |

Native CI results captured on September 27, 2026 with Go 1.23.12 and 1.27.1 show:

- Linux amd64/arm64 and macOS amd64/arm64 are faster for all four clock-reading
  operations. macOS amd64 has particularly large elapsed-time gains; the `Since`
  gain on macOS arm64 is more modest.
- Windows amd64 is faster for all four operations, with smaller gains for
  `Since` and `Now` than for `NowInstant` and `UnixNano`.
- These Windows arm64 measurements used Go clock fallbacks: `NowInstant` was modestly faster, but
  `Since`, `Now`, and `UnixNano` were slower than their standard-library comparisons.
  Windows arm64 now reads the shared clocks directly; those measurements predate this change.

See the dashboard for exact measurements and subsequent runs. Results depend on
the runner hardware, OS, toolchain, and workload; differences between separate CI
jobs do not isolate the effect of a code or Go version change. Other targets,
fallbacks, and `purego` builds may offer no speedup or be slower than the standard
library. Benchmark your workload.

## Wall time

`Now()` returns local `time.Time` values **without Go's
monotonic component**. Wall time can jump backwards or forwards; use Instants
for elapsed-time measurements. Wall timestamps have the signed Unix-nanosecond
range, roughly 1678–2262, subject to OS clock limits.

Wall time comes from the OS, with no calibration, background polling, or manual
refresh. Capture `Now()` separately when you need a calendar timestamp;
Instants cannot be converted to wall time.

## Platforms

Default builds use these clock sources:

| Platform | Elapsed time | Wall time (`Now`, `UnixNano`) |
| --- | --- | --- |
| Darwin amd64/arm64 | Mach approximate clock | XNU calendar mapping plus approximate ticks; Go fallback |
| Linux amd64/arm64 | Kernel coarse monotonic clock; Go fallback selected at startup | Kernel coarse realtime clock; Go fallback |
| Windows amd64/arm64 | Shared InterruptTime counter | Shared SystemTime page |
| Other targets | Go monotonic clock | Go wall clock |

Suspend accounting matches Go's current clocks: Linux and macOS exclude system
sleep; Windows includes it. Other targets follow Go's clock.

Build with `-tags=purego` to use standard-library clocks on every platform,
disabling this package's assembly and native clock access.

Windows amd64/arm64 uses the same [shared clocks as Go](https://go.dev/src/runtime/time_windows.h),
keeping [interrupt time](https://learn.microsoft.com/en-us/windows/win32/sysinfo/interrupt-time)
in native ticks until duration conversion.

The macOS wall reader uses XNU's calendar mapping, falling back to Go when a
snapshot is unusable. See the [calendar reader notes](research/darwin/calendar/README.md).

Linux amd64/arm64 uses the kernel vDSO through a verified Go runtime bridge, with
standard-library clock fallbacks. Initialization requires `/proc/self/exe` and panics if bridge
verification fails. Stripped and PIE executables are supported; custom packers,
obfuscation, and shared-library builds are unvalidated. Other Linux architectures
use Go clocks. The monotonic source is selected once at startup: when the coarse
vDSO clock is unavailable, `Instant` uses `time.Since` from a fixed `time.Now()`
origin. A coarse read failure after successful selection panics instead of
changing epochs beneath existing Instants. Wall reads can fall back per call;
`Now` always removes Go's monotonic component. ARM64 also publishes the original
goroutine and caller traceback metadata; its runtime layout operands are verified
at startup. See the [bridge notes](research/linux-vdso-bridge/README.md).

## Tests and benchmarks

```sh
go test ./...
go test -tags=purego ./...
go vet ./...
go test -run '^$' -bench . -benchmem -count=5
```

On Linux amd64/arm64, `COARSETIME_REQUIRE_VDSO=1 go test ./...` requires the vDSO path;
`BenchmarkLinuxCoarseSyscall` measures the former syscall implementation.
`BenchmarkLinuxWallFallback` and `BenchmarkLinuxInstantFallback` compare the old
syscalls with Go clocks on native GitHub Linux amd64 and arm64 runners.

Run benchmarks on an otherwise idle machine. Compare `BenchmarkSince` with
`BenchmarkTimeSince` for elapsed timing; `BenchmarkClockProgress` measures clock
updates rather than read speed.

See [research](research/README.md) for implementation details and experiments.

## Source map

The production library is one package at the repository root. Files are grouped
by clock family, with OS and architecture suffixes selecting native implementations:

- `doc.go`: package documentation.
- `instant.go` and `instant_*`: elapsed-time API, tick conversion, and platform readers.
- `wall.go` and `wall_*`: wall-time API, platform readers, and standard-library fallbacks.
- `vdso_linux_*` and `asmcgocall_linux*.go`: Linux bridge shared by both clock families.
- `internal/darwinwall/`: portable XNU calendar arithmetic, tested with synthetic mappings.
- `*_test.go`: adjacent correctness tests; `coarsetime_bench_test.go` covers both public clock families.
- `research/`: experiments and retained evidence, indexed by topic and adoption status.

Native assembly shares its Go reader's filename stem. The `purego` build tag
selects standard-library readers instead of native clock access.

## License

Copyright 2026 coarsetime contributors.
Licensed under the Open Software License version 3.0 (OSL-3.0).
See [LICENSE](LICENSE) for the full license text.

This license applies to original code in this repository. Third-party code retains
its existing notices and license terms, including the Go-derived code in
`research/linux-vdso-bridge`, covered by its [GO-LICENSE](research/linux-vdso-bridge/GO-LICENSE).
