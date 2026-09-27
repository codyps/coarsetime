# coarsetime

Inexpensive elapsed-time and approximate wall-time readings for Go. Uses coarse
platform clocks where available, trading resolution and freshness for speed.
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

Every platform exposes the same API:

| API | Purpose |
| --- | --- |
| `NowInstant()` | Read an opaque, process-local `Instant` |
| `Since(start)`, `end.Sub(start)` | Measure elapsed time |
| `Before`, `After` | Compare Instants |
| `Now()`, `UnixNano()` | Read approximate wall time |

Readings may repeat; short intervals may measure zero. There is no guaranteed
resolution or maximum staleness, and nanosecond units do not imply nanosecond
accuracy. Benchmark your workload before choosing this over Go's `time` package.

Instants are meaningful only within the process that created them. Their zero
value is uninitialized; do not persist them or exchange them between processes.
`Sub` truncates fractional nanoseconds and saturates at `time.Duration` limits.

## Wall time

`Now()` returns local `time.Time` values **without Go's
monotonic component**. Wall time can jump backwards or forwards; use Instants
for elapsed-time measurements. Wall timestamps have the signed Unix-nanosecond
range, roughly 1678–2262, subject to OS clock limits.

Wall time comes from the OS on every platform. There is no local calibration,
background polling, or application-managed refresh. Instants cannot be converted
to wall time: capture `Now()` separately when you need a calendar timestamp.

**API change:** `Instant.Time()` and `RefreshWallClock()` have been removed.
Replace the former with a wall timestamp captured at the event; remove refresh
calls. The old conversion depended on a mutable global clock correction.

## Platforms

Default builds use these clock sources:

| Platform | Elapsed time | Wall time (`Now`, `UnixNano`) |
| --- | --- | --- |
| Darwin amd64/arm64 | Mach approximate clock | XNU calendar mapping plus approximate ticks; Go fallback |
| Linux | Kernel coarse monotonic clock | Kernel coarse realtime clock |
| Windows amd64 | Go monotonic clock | Shared SystemTime page |
| Other Windows architectures | Go monotonic clock | Go wall clock |
| Other platforms | Go monotonic clock | Go wall clock |

The native Darwin and Linux elapsed clocks exclude suspend time. Elsewhere,
suspend behavior follows Go's clock. This is not a portable clock for deadlines
that must include time spent asleep. Some optimized paths depend on OS layouts
or private Go runtime bridges; compatibility can vary with the toolchain.

Build with `-tags=purego` for a standard-library fallback on **every platform**.
It disables this package's assembly and native clock access. Instants use Go's
monotonic clock; `Now` and `UnixNano` read Go's wall clock directly, without
requiring refreshes. The API is unchanged, but speed, resolution, and suspend
behavior may differ.

The Darwin reader makes one attempt to read a consistent, usable calendar
mapping from XNU's shared page. It falls back to Go's wall clock on invalidation,
concurrent updates, or an out-of-window approximate sample. No polling loop is
used. Switching back from a precise fallback to an approximate sample can also
move wall time backward without an OS clock adjustment. See the
[calendar reader notes](research/darwin/calendar/README.md) for
source evidence, native Intel measurements, and validation limits.

Linux amd64 uses the vDSO address already resolved by Go's runtime through the
compatibility linkname `runtime.vdsoClockgettimeSym`. It does not read
`/proc/self/maps` or `/proc/self/mem` to discover the vDSO. Calls use Go's
`runtime.asmcgocall` system-stack bridge. On every supported Go version,
initialization resolves the bridge's
assembly entry from `/proc/self/exe`, verifies it against the running process,
and caches its address for indirect calls. This avoids the direct runtime-symbol
references rejected by Go 1.27 and works with stripped and PIE executables,
without cgo or special linker flags. Reads remain allocation-free.

This depends on Go's private runtime ABI, vDSO variable, and executable metadata.
The bridge resolver still requires `/proc/self/exe`. Initialization
panics if the bridge cannot be verified; a Go compatibility failure does not
silently select a syscall. Custom packers, obfuscation, and shared-library builds
are not validated. If the kernel vDSO itself is unavailable or rejects a call,
Linux uses a syscall with the same clock ID; other Linux architectures use that
syscall path directly. See the [bridge investigation](research/linux-vdso-bridge/README.md)
for implementation details, compatibility tests, and measurements.

## Tests and benchmarks

```sh
go test ./...
go test -tags=purego ./...
go vet ./...
go test -run '^$' -bench . -benchmem -count=5
```

On Linux amd64, `COARSETIME_REQUIRE_VDSO=1 go test ./...` requires a working
vDSO path. Tests also force the syscall fallback to check clock consistency;
`BenchmarkLinuxCoarseSyscall` measures that fallback separately.

Run benchmarks on an otherwise idle machine, separately from builds and race
tests. Compare `BenchmarkSince` with `BenchmarkTimeSince` for elapsed timing;
`time.Now()` also reads wall time. `BenchmarkClockProgress` measures observed
clock updates, not read latency or an accuracy guarantee.

See [research](research/README.md) for platform investigations, implementation
details, prototypes, and [retained measurements](research/measurements.md).
Research code is separate from ordinary `go test ./...` runs.
