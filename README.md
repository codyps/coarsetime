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
| `instant.Time()` | Convert an Instant using the cached wall-clock correction |
| `RefreshWallClock()` | Refresh that correction |

Readings may repeat; short intervals may measure zero. There is no guaranteed
resolution or maximum staleness, and nanosecond units do not imply nanosecond
accuracy. Benchmark your workload before choosing this over Go's `time` package.

Instants are meaningful only within the process that created them. Their zero
value is uninitialized; do not persist them or exchange them between processes.
`Sub` truncates fractional nanoseconds and saturates at `time.Duration` limits.

## Wall time and refreshes

`Now()` and `Instant.Time()` return local `time.Time` values **without Go's
monotonic component**. Wall time can jump backwards or forwards; use Instants
for elapsed-time measurements. Wall timestamps have the signed Unix-nanosecond
range, roughly 1678–2262, subject to OS clock limits.

`Instant.Time()` uses a correction calibrated at startup. On Darwin and other
platforms with cached wall time, `Now()` and `UnixNano()` use it too (see below).
There is no background updater. Call `RefreshWallClock()` after resume or clock
changes, or periodically if those timestamps need to follow the system clock.

**Cached wall time can drift or remain wrong until refreshed.** Refreshing can
change the wall-time conversion of an existing Instant. Retain the converted
`time.Time` if you need a stable historical timestamp.

## Platforms

Default builds use these clock sources:

| Platform | Elapsed time | Wall time (`Now`, `UnixNano`) |
| --- | --- | --- |
| Darwin amd64/arm64 | Mach approximate clock | Cached correction |
| Linux | Kernel coarse monotonic clock | Kernel coarse realtime clock |
| Windows amd64 | Go monotonic clock | Shared SystemTime page |
| Other Windows architectures | Go monotonic clock | Go wall clock |
| Other platforms | Go monotonic clock | Cached correction |

The native Darwin and Linux elapsed clocks exclude suspend time. Elsewhere,
suspend behavior follows Go's clock. This is not a portable clock for deadlines
that must include time spent asleep. Some optimized paths depend on OS layouts
or private Go runtime bridges; compatibility can vary with the toolchain.

Build with `-tags=purego` for a standard-library fallback on **every platform**.
It disables this package's assembly and native clock access. Instants use Go's
monotonic clock; `Now` and `UnixNano` read Go's wall clock directly, without
requiring refreshes. `Instant.Time()` still uses the cached correction. The API
is unchanged, but speed, resolution, and suspend behavior may differ.

## Tests and benchmarks

```sh
go test ./...
go test -tags=purego ./...
go vet ./...
go test -run '^$' -bench . -benchmem -count=5
```

Run benchmarks on an otherwise idle machine, separately from builds and race
tests. Compare `BenchmarkSince` with `BenchmarkTimeSince` for elapsed timing;
`time.Now()` also reads wall time. `BenchmarkClockProgress` measures observed
clock updates, not read latency or an accuracy guarantee.

See [research](research/README.md) for platform investigations, implementation
details, prototypes, and [retained measurements](research/measurements.md).
Research code is separate from ordinary `go test ./...` runs.
