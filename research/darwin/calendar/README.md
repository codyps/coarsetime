# Darwin wall time without local calibration

Implemented 2026-09-27, following the pinned XNU investigation in
[the Darwin notes](../README.md). `Instant.Time()` and `RefreshWallClock()` are
removed from the production API. Elapsed clocks are unchanged; wall time is now
independent of process-local calibration on every platform.

## Why the shared page is useful

XNU publishes a calendar anchor and an adjusted rate, not merely a Unix offset.
The existing [source investigation](../README.md#commpage-layout-calculation-and-synchronization)
describes its five fields and publication protocol. The production reader uses
that record with the separately published approximate Mach tick. All correction
state belongs to the kernel, including NTP rate changes and wall-clock steps.

The boot-time field alone is insufficient: adding uptime to it would miss the
calendar clock's accumulated rate adjustments. Nor can the calendar anchor alone
serve as a fresh wall clock: its update cadence is not guaranteed.

The fast path uses atomic shared-memory loads and integer arithmetic. It makes
one attempt, validates the anchor before/after the payload, and rejects samples
before the anchor or outside its one-second interpolation window. It handles
the full binary fraction and multiplication carry, then bounds-checks signed
Unix nanoseconds. Unlike libc's timeval interface, fractional conversion here
retains nanosecond units; that is not a precision or accuracy claim.

Invalid or changing records, unsupported approximate clocks, out-of-window
samples, and unrepresentable dates go to `time.Now().UnixNano()`. This is an
on-demand fallback, not a timer, polling loop, or local offset refresh. It can
cost more than a normal Go wall read when the attempted fast path misses.
Switching from a precise fallback to a lagging approximate sample can make wall
time regress even without an OS wall-clock step. No monotonic wall guarantee is
added; Instants remain the elapsed-time API.

An approximate sample can be older than a newly published anchor. Returning a
fabricated offset or extrapolating backwards would be wrong; falling back is
deliberate. The reader does not obtain precise Mach time just to check freshness.
Its accepted value is wall time at the approximate sample, with the same lack
of a hard staleness guarantee as the package's coarse clock. The one-second
window is relative to that sample, not a guarantee about the current instant.

## Implementation and assumptions

- [`internal/darwinwall`](../../../internal/darwinwall/calendar.go) implements
  one-pass snapshot validation and binary-fraction conversion. The same code can
  be tested against ordinary Go memory on any host.
- [`wall_darwin.go`](../../../wall_darwin.go) selects the fast path and fallback.
  Assembly supplies the calendar pointer, approximate-time pointer, and support
  flag once at startup. It does not read the clock on each call.
- Intel uses base `0x7fffffe00000`, calendar `+0xd0`, approximate ticks `+0x80`,
  and support `+0x88`. ARM64 uses base `0xfffffc000`, calendar `+0x120`, ticks
  `+0xc0`, and support `+0xc8`. Go atomic loads preserve reader ordering on both.
- These are private XNU layouts observed in the macOS versions supported by
  Go 1.23+. See [layout history](../history/README.md). The approximate support
  flag is not a general ABI/mapping probe. `purego` avoids these package-owned
  accesses entirely.

Sources checked against XNU `xnu-12377.1.9`:

- [Calendar reader](https://github.com/apple-oss-distributions/xnu/blob/xnu-12377.1.9/libsyscall/wrappers/__commpage_gettimeofday.c)
- [Intel writer](https://github.com/apple-oss-distributions/xnu/blob/xnu-12377.1.9/osfmk/i386/commpage/commpage.c)
- [Kernel calendar maintenance](https://github.com/apple-oss-distributions/xnu/blob/xnu-12377.1.9/osfmk/kern/clock.c)
- [Intel layout](https://github.com/apple-oss-distributions/xnu/blob/xnu-12377.1.9/osfmk/i386/cpu_capabilities.h)
- [ARM layout](https://github.com/apple-oss-distributions/xnu/blob/xnu-12377.1.9/osfmk/arm/cpu_capabilities.h)
- [Approximate-clock reader](https://github.com/apple-oss-distributions/xnu/blob/xnu-12377.1.9/libsyscall/wrappers/mach_approximate_time.c)

## Validation and performance

Portable tests cover the five-field layout, changed/invalidated anchors, samples
older than an anchor, expiry, zero rates, fractional carry, overflow, simulated
wall steps and rate changes, and concurrent publication. Randomized arithmetic
is compared with an independent arbitrary-precision calculation.

Darwin-specific tests exercise the production selection/fallback paths with
synthetic records, and sample the real shared page against Go wall time. The
live test spreads ten sampling bursts across more than one second, covering
more than one interpolation window rather than just a tight startup loop. It
logs the acceptance count; setting `COARSETIME_REQUIRE_DARWIN_WALL=1`
requires at least one accepted snapshot rather than allowing a fallback-only run.

The initial change was developed on Windows; native validation followed on
2026-09-27. On Intel macOS 15.7.9 (24G830), Core i7-4960HQ, Go 1.26.6, the
extended live test accepted 10000/10000 snapshots. Default, purego, race (both
build modes), checkptr=2, cgo-disabled tests, and default/purego vet passed.
Darwin/arm64 default and purego test binaries also cross-compiled successfully.

[CI for the initial implementation](https://github.com/codyps/coarsetime/actions/runs/36325441150)
also passed all four native Darwin jobs: Intel and Apple Silicon with Go
1.23.12 and 1.27.1, including race detection and required live calendar reads.
Intel accepted 10000/10000 samples in both jobs; ARM64 accepted 9990/10000
and 9982/10000 respectively. CI now also runs checkptr=2 on both architectures.
The run's Linux/amd64 Go 1.27.1 linker failures are unrelated to the Darwin reader.

These checks do not validate real suspend/resume, NTP slews, or clock steps.
Do not reuse historical cached-wall benchmark numbers for this implementation.

On each Intel and Apple Silicon Mac, from the repository root:

```sh
COARSETIME_REQUIRE_DARWIN_WALL=1 go test -v ./...
go test -race ./...
go test -gcflags=all=-d=checkptr=2 ./...
go test -tags=purego ./...
go vet ./...
go test -run '^$' -bench 'Benchmark(DarwinCalendarRead|DarwinStandardUnixNano|UnixNano|Now|TimeNow)$' -benchmem -count=8
```

`BenchmarkDarwinCalendarRead` includes fallback work and reports `fallback-%`.
Compare the ordinary public `BenchmarkUnixNano` to the standard-library control;
the diagnostic benchmark adds miss-count bookkeeping. Also exercise idle time,
CPU load, sleep/resume, and controlled clock changes on a disposable test host.
No system clock settings were changed during development.

On the Intel host above, five 500 ms samples with `GOMAXPROCS=1` gave these
medians, all with zero allocations:

| Benchmark | Median ns/op |
| --- | ---: |
| `UnixNano` | 10.19 |
| `Now` | 15.11 |
| `DarwinCalendarRead` (includes fallback accounting) | 8.971 |
| `DarwinStandardUnixNano` | 91.21 |
| `TimeNow` | 93.23 |

Calendar fallback rates ranged from 0.001330% to 0.002326% in this tight-loop
workload. [Raw output](native-intel.txt) was collected with the benchmark command
above, adding `GOMAXPROCS=1 -benchtime=500ms -count=5` (the environment assignment
precedes `go test`). Results were noisy, especially `TimeNow` (89.41–294.1 ns/op),
and are host/workload observations, not a latency or fallback-rate guarantee.
Apple Silicon latency and fallback rates remain unmeasured.

`BenchmarkSyntheticRead` measures Go-owned test memory only. It is useful for
the snapshot/arithmetic cost but cannot establish Darwin latency or hit rate:

```sh
go test ./internal/darwinwall -run '^$' -bench . -benchmem -count=5
```

On Windows/amd64, Ryzen 9 7940HS, Go 1.27.0, five 500 ms samples at
GOMAXPROCS=1 gave **6.535 ns/op median**, range 6.349–7.598 ns/op, with zero
allocations. [Raw output](synthetic-windows.txt). This excludes OS update
contention, the outer platform branch, and real fallback work.

Local validation completed:

- Windows default/purego tests and vet, with Go 1.27.0; tests and vet with Go
  1.23.12 also passed.
- Default/purego root test binaries compiled for Darwin amd64/arm64, Windows
  amd64/arm64/386, Linux arm64, FreeBSD amd64, and js/wasm. Darwin vet passed.
- Linux/amd64 purego root tests and the portable calendar tests ran under WSL.
- ARM64 disassembly contains ordered `LDAR` loads for all seven shared-memory
  reads, followed by arithmetic; no Mach/libc call is in that snapshot path.

The pre-existing default Linux/amd64 `runtime.asmcgocall` linker incompatibility
with this local toolchain remains outside this change. Linux's wall and elapsed
implementations were not modified.
