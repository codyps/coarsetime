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

On Darwin and other platforms, these functions use one coarse read plus one
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

See the [prototype comparison](research/darwin/benchmarks/README.md) for the three
implementations, validation, and measured differences.

Adopted amd64 implementation, same host and Go 1.26.6, medians of five 500 ms
runs (all zero allocations):

| Operation | ns/op |
| --- | ---: |
| `NowInstant()` | 0.628 |
| `Since()` | 3.651 |
| `UnixNano()` | 2.358 |
| `Now()` | 5.050 |

[Raw adoption measurements](research/darwin/benchmarks/adopted.txt).

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

Pre-commpage sample (2026-09-05, Darwin amd64, Intel i9-9880H, Go 1.26.6):
medians of three one-second runs, with no concurrent build or test jobs:

| Operation | ns/op |
| --- | ---: |
| `coarsetime.NowInstant()` | 17.06 |
| `coarsetime.Since(start)` | 20.56 |
| `time.Since(start)` | 35.13 |
| `time.Now()` | 75.58 |

All four reported zero allocations. These are host-specific measurements, not
portable performance guarantees. Darwin arm64 and Windows amd64
were cross-compiled; runtime behavior on those targets has not been verified.

Linux sample (2026-09-05, Docker Linux amd64 on the same Intel host, Go 1.26.6,
vDSO explicitly required): medians of three one-second runs:

| Operation | ns/op |
| --- | ---: |
| `coarsetime.NowInstant()` | 16.80 |
| `coarsetime.Since(start)` | 19.93 |
| `time.Since(start)` | 24.80 |
| `time.Now()` | 43.15 |
| Coarse clock syscall alone | 117.7 |

All reported zero allocations. Native Linux amd64 tests passed with Go 1.23.2
and 1.26.6, including vDSO selection and forced syscall fallback. Go 1.26.6 race,
vet, and concurrent CPU-profiling checks passed. Linux 386 syscall tests ran
successfully; Linux arm64, arm, and riscv64 were cross-compiled only.

Pre-commpage wall-time sample (2026-09-05, same Intel host, Go 1.26.6; medians of three
500 ms runs per platform, run sequentially):

| Operation | Darwin amd64 ns/op | Linux amd64 ns/op |
| --- | ---: | ---: |
| `Now()` | 21.78 | 18.61 |
| `UnixNano()` | 21.33 | 16.59 |
| Existing `Instant.Time()` | 3.446 | 3.051 |
| `time.Now()` | 81.44 | 40.89 |
| `RefreshWallClock()` | 1538 | 853.1 |

All reported zero allocations. These timings do not measure wall-clock accuracy.

## Comparing with fastime

A separate [benchmark module](benchmarks/fastime_test.go) pins
[`github.com/kpango/fastime` v1.1.10](https://github.com/kpango/fastime/tree/v1.1.10).
It requires Go 1.24.4 or newer; the main library remains dependency-free with Go
1.23 support. Run this suite explicitly (root `go test ./...` does not enter
nested modules):

```sh
go -C benchmarks test -run '^$' -bench . -benchmem -benchtime=500ms -count=3
```

It compares `fastime.Now` and `fastime.UnixNanoNow` with `Now`, `UnixNano`,
and `time.Now`, plus concurrent time.Time reads. Fastime runs with active 1 ms
and 5 ms updater intervals; 5 ms is its upstream global default. The suite stops
its import-time global updater before reference benchmarks, starts it outside
each fastime measurement, and cancels/stops it during cleanup. Both serial and
parallel results are consumed to prevent dead-code elimination.

Fastime reads a timestamp cached by a background goroutine. Its v1.1.10 updater
advances that timestamp on ticks and periodically corrects it from system time
(at a nominal 100 ms correction interval). Its timer interval is not an accuracy
bound: scheduling delays and missed ticks affect freshness. These benchmarks
measure read throughput with the updater active, excluding startup and shutdown;
per-operation allocation averages do not imply that the updater allocates
nothing or has no CPU cost. They do not measure accuracy or idle updater cost.

Fastime comparison sample (2026-09-05, Intel i9-9880H, Go 1.26.6, Darwin amd64
and Docker Linux amd64; medians of three 500 ms runs, platforms run sequentially):

| Serial read | Darwin ns/op | Linux ns/op |
| --- | ---: | ---: |
| `fastime.Now()`, 1 ms updater | 2.074 | 2.560 |
| `fastime.Now()`, 5 ms updater | 2.123 | 2.536 |
| `fastime.UnixNanoNow()`, 1 ms updater | 1.760 | 1.738 |
| `fastime.UnixNanoNow()`, 5 ms updater | 1.731 | 1.731 |
| `coarsetime.Now()` | 20.52 | 22.41 |
| `coarsetime.UnixNano()` | 20.34 | 21.06 |
| `time.Now()` | 77.21 | 49.54 |

All reported zero per-operation allocations after rounding/amortization. Race
instrumented smoke runs exercised all serial and parallel benchmarks on both
platforms, and vet passed. Parallel ns/op reports aggregate throughput, not the
latency of an individual read.
