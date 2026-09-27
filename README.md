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

Wall time comes from the OS on every call. Capture `Now()` separately when you
need a calendar timestamp; Instants cannot be converted to wall time.

## Platforms

Default builds use these clock sources:

| Platform | Elapsed time | Wall time (`Now`, `UnixNano`) |
| --- | --- | --- |
| Darwin amd64/arm64 | Mach approximate clock | XNU calendar mapping plus approximate ticks; Go fallback |
| Linux | Kernel coarse monotonic clock | Kernel coarse realtime clock |
| Windows amd64 | Shared InterruptTime counter | Shared SystemTime page |
| Other Windows architectures | Go monotonic clock | Go wall clock |
| Other platforms | Go monotonic clock | Go wall clock |

The native Darwin and Linux elapsed clocks exclude suspend time. The native
Windows amd64 clock includes it. Elsewhere, suspend behavior follows Go's clock.
This is not a portable clock for deadlines that must include time spent asleep.
Some optimized paths depend on OS layouts
or private Go runtime bridges; compatibility can vary with the toolchain.

Build with `-tags=purego` to use Go's standard-library clocks on every platform.
This disables the package's assembly and native clock access. The API is
unchanged, but speed, resolution, and suspend behavior may differ.

Darwin reads XNU's shared calendar page once per call and falls back to Go's
wall clock if the sample is unusable. Switching between these sources can move
wall time backward even without an OS clock adjustment. See the
[calendar reader notes](research/darwin/calendar/README.md) for implementation
details and validation limits.

Linux amd64 reads the coarse clocks through the kernel vDSO, using Go's runtime
to locate and call it. Initialization requires `/proc/self/exe` to resolve and
verify the runtime bridge, and panics if verification fails. This path depends
on Go's private runtime ABI and executable metadata. Stripped and PIE executables
are supported; custom packers, obfuscation, and shared-library builds are not
validated. Clock reads are allocation-free.

If the vDSO is unavailable or rejects a call, Linux amd64 uses a syscall with
the same clock ID. Other Linux architectures use syscalls directly. See the
[bridge investigation](research/linux-vdso-bridge/README.md) for implementation
details, compatibility tests, and measurements.
On Windows amd64, `NowInstant` directly reads `KUSER_SHARED_DATA.InterruptTime`
at `0x7ffe0008`, using the atomic 64-bit load described in
[Go's shared-page definitions](https://go.dev/src/runtime/time_windows.h) and
used by [Go's monotonic reader](https://go.dev/src/runtime/sys_windows_amd64.s).
Instants retain native 100 ns ticks; `Sub` and `Since` convert differences to
nanoseconds. The counter is unaffected by wall-clock adjustments, and its units
do not imply 100 ns resolution. See Microsoft's
[QueryInterruptTime documentation](https://learn.microsoft.com/en-us/windows/win32/api/realtimeapiset/nf-realtimeapiset-queryinterrupttime)
and [interrupt-time overview](https://learn.microsoft.com/en-us/windows/win32/sysinfo/interrupt-time).
This is a direct OS-layout dependency, not a call to the documented Windows API.


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

## License

Copyright (c) 2026 coarsetime contributors.
Licensed under the Open Software License version 3.0 (OSL-3.0).
See [LICENSE](LICENSE) for the full license text.

This license applies to original code in this repository. Third-party code retains
its existing notices and license terms, including the Go-derived code in
`research/linux-vdso-bridge`, covered by its [GO-LICENSE](research/linux-vdso-bridge/GO-LICENSE).
