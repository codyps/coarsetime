# coarsetime

A small Go clock for inexpensive elapsed-time and approximate wall-time readings. It trades clock freshness
and resolution for read speed where the platform provides an approximate clock.

```go
start := coarsetime.NowInstant()
// Do some work.
elapsed := coarsetime.Since(start) // time.Duration
end := coarsetime.NowInstant()
_ = end.Sub(start)
_ = start.Before(end)
```

`Instant` stores native ticks in an opaque, comparable value. `NowInstant` does no
per-read timebase conversion. `Sub` and `Since` convert differences to nanoseconds,
truncate fractional nanoseconds toward zero, and saturate at the limits of
`time.Duration`. Conversion handles intermediate multiplication overflow.

## Portable API

Every supported target exposes the same API: `Now`, `UnixNano`, `NowInstant`,
`Since`, `RefreshWallClock`, and `Instant` with `Before`, `After`, `Sub`, and
`Time`. Public declarations and documentation live in shared Go files. Private
implementations selected at build time preserve each platform's fast path;
there is no runtime interface dispatch.

| Operation | Shared contract | Platform implementation |
| --- | --- | --- |
| `Now`, `UnixNano` | Approximate wall time; may repeat or jump | Linux kernel coarse realtime; Windows shared clock or standard-library fallback; cached correction elsewhere |
| `NowInstant`, `Since`, `Instant.Sub` | Process-local elapsed time | Mach approximate ticks on Darwin; Linux coarse monotonic clock; Go monotonic clock elsewhere |
| `Instant.Time` | Translate using the current cached wall correction | Same contract everywhere, with optimized conversion on Darwin amd64 |
| `RefreshWallClock` | Refresh the correction used by `Instant.Time` | Also updates `Now`/`UnixNano` where they use that correction |

`Now` never carries Go's monotonic component. Suspend behavior, resolution, and
freshness remain platform dependent as described below. There are no platform-only
exported entry points.

Build with `-tags=purego` to use standard-library clocks on **every platform**:

- `NowInstant` and `Since` use Go's monotonic clock (`time.Since` from a process-local origin).
- `Now` and `UnixNano` read `time.Now().UnixNano()` directly. `Now` still has no
  monotonic component, preserving the public contract.
- `Instant.Time` still uses the cached correction; `RefreshWallClock` updates
  that correction but does not affect direct wall reads.

This disables this package's assembly, shared-page reads, native syscalls, and
private runtime bridges. The standard library may itself use native code.
It is a compatibility and diagnostic fallback, not a requirement for disabling
cgo: normal builds already work with `CGO_ENABLED=0`. Resolution, speed, and
suspend behavior follow Go's clocks and can differ from the native coarse clocks.
The platform implementation descriptions below refer to builds without `purego`.

The API surface test checks 12 OS/architecture combinations, both with and
without `purego`, and ensures purego selects no package assembly or native access
imports/directives. See the [API refactor measurements](research/api-shape/README.md)
for before/after timings and validation limits.

## Clock contract

- Use readings for elapsed time and ordering within one process. The zero value
  is not an initialized reading. Instants have no calendar/Unix epoch and should
  not be persisted or exchanged between processes.
- Successive readings can be equal. There is no promised resolution, update
  interval, or maximum staleness. Short measured intervals may be zero.
- Darwin uses the Mach approximate clock, excluding system sleep: amd64 reads
  the commpage directly when supported; arm64 calls `mach_approximate_time`.
- Linux uses `CLOCK_MONOTONIC_COARSE`, excluding system suspend. It follows
  gradual NTP/clock-rate adjustments, without wall-clock discontinuities.
- Other targets use `time.Since` with an initialization-time monotonic origin.
  Their sleep behavior follows Go and the operating system. This is a functional
  fallback, with no promised performance improvement over Go's clock.
- There is no background updater goroutine. The Instant-to-wall correction is
  calibrated once at startup and can be refreshed explicitly.

Sleep behavior is not uniform across platforms. Do not use this API for a
portable deadline that must count suspended time.

## Approximate wall time

```go
stamp := coarsetime.Now()    // time.Time, local location, no monotonic component
unixNS := coarsetime.UnixNano() // int64; avoids constructing time.Time
instant := coarsetime.NowInstant()
converted := instant.Time()    // translates with the current cached correction
_, _, _ = stamp, unixNS, converted
```

Linux `Now` and `UnixNano` read `CLOCK_REALTIME_COARSE` directly, using the
same vDSO bridge (or syscall fallback) as the elapsed clock. The kernel maintains
wall corrections, including clock steps and suspend time. This is cheaper than
adding a userspace correction and requires no periodic recalibration.

On Darwin and other non-Windows platforms, these functions use one coarse read plus one
atomic load of a cached nanosecond correction. `Instant.Time` uses that correction
on every platform. Calibration samples eight brackets using `time.Now` around a
coarse read and selects the shortest precise monotonic bracket; it associates the
coarse reading with the bracket's wall-time midpoint. This reduces scheduling
error but cannot remove coarse-clock staleness.

The correction is initialized once. Call `coarsetime.RefreshWallClock()` after
resume or clock changes, or periodically from your application's existing timer
if you want it to follow wall-time adjustments. There is no timer check, lock,
or allocation on the read path, and no automatic refresh goroutine. Concurrent
refreshes serialize; readers observe an atomic correction without taking a lock.

**Cached wall time can be wrong until refreshed.** On suspend-pausing clocks it
will be behind by the suspended duration; it also misses wall-clock steps and
accumulates drift. Refreshing can jump converted timestamps forward or backward,
and can change the result of converting the same Instant. Keep the converted
`time.Time` if a historical timestamp must remain stable. Linux's direct wall
reads can also jump when the system clock changes. Use `Instant` for elapsed-time
comparisons unaffected by wall corrections.

Nanosecond units do not promise nanosecond accuracy; no error bound is guaranteed.
These wall APIs use int64 Unix-nanosecond dates (roughly 1678–2262), subject to
underlying OS clock limits. On 32-bit Linux the wall syscall uses the time64 ABI
when available; older kernels fall back to their legacy 32-bit seconds ABI. Benchmarks
include `Now`, `UnixNano`, conversion of an existing Instant, concurrent wall
reads, and the separate cost of `RefreshWallClock`.

## Windows implementation

Windows/amd64 `UnixNano` reads the shared SystemTime clock directly, preserving
the Windows fast path. The `purego` tag and other Windows architectures use
`time.Now().UnixNano()`. `Now` constructs an approximate wall time from that
reading, without a monotonic component; use `NowInstant` and `Since` for elapsed
time. Neither Windows wall API requires `RefreshWallClock`; the cached correction
is used only by `Instant.Time` on Windows.

The shared-page implementation follows the Go runtime's Windows layout and makes
no maximum-staleness guarantee. See the [Windows investigation](research/windows/plan.md)
and [benchmark results](research/windows/public-api-benchmark-results.txt).
Those historical `Now` benchmarks measured the earlier standard-library wrapper.

## Darwin implementation

Darwin amd64 checks the commpage approximate-time support flag once at
initialization. When supported, each read uses an inlineable Go atomic load of
the aligned 64-bit timestamp. Small assembly helpers supply the support flag and
pointer; the hot path has no libc call or system-stack transition. This depends
on XNU's private commpage ABI (`APPROX_TIME` at `0x7fffffe00080`). The support flag
handles unavailable approximate clocks; it does not detect arbitrary ABI changes.

Darwin arm64, and the amd64 unsupported-clock fallback, call
`mach_approximate_time` from libSystem. The Go linker imports Mach functions
without cgo. Architecture-specific assembly supplies C-ABI trampolines. Calls use
the internal `syscall.rawSyscall` bridge, which reaches `runtime.libcCall` and the
system stack without scheduler syscall bookkeeping. Timebase factors are
initialized once through this bridge on both architectures.

The bridge is an internal Go compatibility dependency. Its errno interpretation
is ignored for Mach timestamp reads, which return all 64 timestamp bits without
an error result. Test new Go toolchains and macOS releases before adopting them.
Direct commpage execution is validated on amd64 only; arm64 retains libSystem.
The cached wall-time correction and its refresh requirements are unchanged.

See the [prototype comparison](research/darwin/commpage/README.md) for the three
implementations, validation, and measured differences.

The optimized wall readers preserve explicit refresh requirements and handle
negative timestamps and non-unit timebase ratios. Historical measurements are
collected in the [research results](research/measurements.md).

## Linux implementation

Linux amd64 resolves the kernel's versioned `clock_gettime` vDSO symbol
once at initialization. The resolver reads the `[vdso]` mapping through
`/proc/self/maps` and `/proc/self/mem` and checks the ELF architecture, symbol,
version, and executable segment. No libc or cgo dependency is required.

Reads call the vDSO with `CLOCK_MONOTONIC_COARSE` through a small C-ABI trampoline
and Go's internal `runtime.asmcgocall` system-stack bridge. This avoids a kernel
transition on kernels supporting the coarse vDSO clock. It does not reference
private runtime struct offsets or the runtime's vDSO symbol variables. Like the
Darwin bridge, `asmcgocall` is an internal Go compatibility dependency. Profiling
samples taken inside the vDSO may be attributed to the runtime's vDSO bucket
rather than the calling Go stack.

The amd64 wall APIs fuse vDSO argument setup with the read. `Now` constructs
`time.Time` directly from the returned timespec. Measurements are retained in
the [wall-read comparison](research/wall-optimizations/README.md#adoption).

If procfs access or symbol resolution fails, or the vDSO rejects the call, the
implementation uses a real `clock_gettime` syscall with the same clock ID and
epoch. Other Linux architectures, including arm64, use that syscall path directly.
ARM64 requires additional Go runtime signal-stack integration for safe vDSO
calls without cgo; the amd64 bridge must not simply be copied to it. The fallback
preserves clock semantics but can be substantially slower than `time.Since`.
A syscall error panics rather than returning a fabricated reading or changing
clock epochs. Linux must support `CLOCK_MONOTONIC_COARSE` (introduced in 2.6.32).

See Linux's [clock_gettime documentation](https://man7.org/linux/man-pages/man3/clock_gettime.3.html)
and [vDSO documentation](https://man7.org/linux/man-pages/man7/vdso.7.html).

## Validation and benchmarks

With Go 1.23 or newer (or `direnv exec .` for the Nix development shell):

```sh
go test ./...
go test -race ./...
go vet ./...
go test -run '^$' -bench . -benchmem -count=5
CGO_ENABLED=0 go test ./...
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go test -c -o /tmp/coarsetime-arm64.test
```

On Linux, `COARSETIME_REQUIRE_VDSO=1 go test ./...` additionally requires an
active, working vDSO path. Tests compare public readings against coarse-clock
syscall brackets and force the syscall fallback to check epoch consistency.
`BenchmarkLinuxCoarseSyscall` measures that fallback separately.

`BenchmarkNowInstant` measures reading an Instant. `BenchmarkSince` includes duration
conversion. Compare elapsed-time reads with `BenchmarkTimeSince`; `time.Now`
also retrieves wall time. Parallel benchmarks exercise concurrent reads without
a shared result variable. `BenchmarkClockProgress` reports repeated readings and
maximum observed step; its loop includes sampling work and is not a read-latency
benchmark or proof of an accuracy bound. Run benchmarks on an otherwise idle
machine, separately from builds and race tests.

Tests check conversion against arbitrary-precision arithmetic, ordering,
progress, and concurrent reads during GC and stack growth. The progress test's
three-second timeout detects a stuck implementation; it is not a clock guarantee.

## Research

Experiments, prototypes, implementation plans, comparative benchmarks, and raw
results live under [`research/`](research/README.md). The root contains the
production library, its tests, and public API benchmarks. Research sources are
not included by ordinary `go test ./...` or by setting prototype tags alone.
Use the documented overlay runner or the standalone research modules.
