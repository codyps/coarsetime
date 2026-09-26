# Windows timing investigation

Historical investigation and plan, retained from the original Windows work. The
current portable API is documented in the [root README](../../README.md): `Now`
returns approximate wall time without a monotonic component, `UnixNano` retains
the Windows fast path, and `NowInstant`/`Since` provide elapsed-time readings.
The descriptions and recommendations below refer to the original implementation
checkpoint and are not the current library contract.

## Original repository state

At the start of this investigation, the repository had **no Windows support** and was not yet a buildable
Go library. At commit `75cf45e` it contains a partial Darwin Go file (no package
or imports, unfinished `time.T` expression), an assembly trampoline, and a Nix
development shell. The Go call names `mach_absolute_time_trampoline`, while the
assembly defines `runtime·mach_approximate_time_trampoline`. There is no module,
API contract, test suite, or CI configuration. The Nix input targets Darwin.

The Windows experiment is a separate module in `research/windows`, so it can
run without first redesigning or completing the Darwin implementation.

## Recommendation

Keep `Now() time.Time` backed by `time.Now()` on Windows initially. For a
meaningful fast path, expose an explicitly coarse scalar wall timestamp, such as
`UnixNano() int64`, and a separate monotonic `Instant`/elapsed-time API if needed.
Do not use wall timestamps for duration measurement.

The installed Go runtime already reads Windows' shared clock page directly for
both the wall and monotonic parts of `time.Now()`. A custom implementation is not
removing a kernel transition: it is mostly removing work that produces the full
`time.Time` value. A direct scalar read is promising; constructing a `time.Time`
again largely erases the advantage and does not preserve its monotonic component.

“High performance” and “high resolution” need separate contracts. The shared
page is cheap but coarse. QPC is appropriate for precise intervals, and
`GetSystemTimePreciseAsFileTime` for precise wall timestamps. These documented
APIs cost more through Go's supported foreign-call path.

## Measured results

Measured locally on 2026-09-26: Windows 11 Pro build 26200, AMD Ryzen 9 7940HS
(8 cores / 16 logical processors), Go 1.27.0 windows/amd64, CGO disabled.
Five sequential 500 ms repetitions per case, GOMAXPROCS=1; medians and observed
min/max below. These are microbenchmarks on one machine, not statistical claims
about every Windows system. All retained cases report zero allocations per op.

| Operation | Median ns/op | Min–max ns/op |
| --- | ---: | ---: |
| `time.Now()` | 7.470 | 7.294–9.440 |
| `time.Now().UnixNano()` | 12.320 | 11.700–12.490 |
| `time.Since(start)` | 11.480 | 10.920–12.420 |
| Shared wall clock → `time.Time` | 7.374 | 7.095–8.002 |
| Shared wall clock → Unix nanoseconds | 3.363 | 3.158–3.497 |
| Shared raw FILETIME | 2.918 | 2.888–3.039 |
| Shared raw InterruptTime | 2.951 | 2.881–3.190 |
| GetSystemTimeAsFileTime, raw | 70.270 | 68.970–76.500 |
| GetSystemTimeAsFileTime → `time.Time` | 73.240 | 65.520–74.400 |
| GetSystemTimePreciseAsFileTime, raw | 99.080 | 95.380–101.200 |
| GetSystemTimePreciseAsFileTime → `time.Time` | 110.100 | 104.500–118.200 |
| QueryPerformanceCounter, raw | 100.200 | 99.510–101.900 |
| QueryInterruptTime, raw | 70.500 | 69.940–72.910 |
| QueryInterruptTimePrecise, raw | 106.400 | 104.200–110.300 |
| GetTickCount64, raw | 65.170 | 62.870–75.560 |

The scalar Unix timestamp is about **3.7× faster than `time.Now().UnixNano()`**,
or 2.2× faster than `time.Now()` while doing less work. The full-value prototype
is effectively tied with `time.Now()` within the observed variability, despite
omitting the monotonic component. It does not justify replacing `time.Now()`.
Raw counter costs must not be mistaken for equivalent full timestamp operations.

In the retained resolution sample, shared clocks and `time.Now()` had median
positive increments around **1 ms**; documented coarse calls were around
0.55 ms in their separate sampling windows. An earlier diagnostic saw roughly
0.53 ms for the shared clocks too. This variation reinforces that update cadence
is system-dependent. QPC and both precise APIs had 100 ns minimum and median
observed positive increments; QPC frequency was 10 MHz. These observations do
not establish 100 ns accuracy. No backwards readings occurred during sampling.

`go test -v` and `go vet ./...` passed in the experiment module. The wall-clock
test checks 10,000 reads bracketed by `time.Now()`, plus current interrupt-clock
ordering and Unix-epoch conversion. The original root package remains unfinished.

[Reproduction and methodology](README.md),
[raw benchmark output](benchmark-results.txt), and
[validation output](validation-results.txt) are retained.

## Implementation sequence

1. Establish a buildable package: choose the real module path, define the public
   API, finish or isolate the Darwin sketch, and provide a portable `time.Now`
   fallback. Define wall-clock adjustments, resolution, suspend behavior, units,
   and valid timestamp range before implementing optimizations.
2. Start Windows `Now()` with `time.Now()`. It preserves the standard monotonic
   component and local-location behavior. Do not forge the private `time.Time`
   layout or use `go:linkname` to runtime internals.
3. If scalar timestamps meet the use case, promote the Windows/amd64 shared-page
   prototype into `UnixNano()`: read SystemTime, subtract the Windows epoch, and
   convert 100 ns units to nanoseconds. Nanosecond units do not imply nanosecond
   resolution. Document the UnixNano range limitation and backward wall jumps.
4. If coarse elapsed time is needed, use shared InterruptTime in a distinct
   `Instant` representation. Its origin is boot-related; do not compare it with
   Unix time or QPC ticks. Explicitly decide whether suspend time counts. Test
   resume behavior before release. Prefer `time.Since` where its overhead suffices.
5. Keep precise timing opt-in: cache QPC frequency once, convert deltas with
   overflow-safe integer arithmetic, and use the precise FILETIME API for wall
   time. Resolve procedures once and call `syscall.SyscallN` (or an appropriate
   maintained Windows binding). Do not assume raw QPC is UTC or nanoseconds.
6. Restrict shared-page assembly to Windows/amd64 initially. Audit Windows/arm64
   separately: alignment and memory ordering cannot be copied from x86. For any
   supported 32-bit target, use a reviewed high/low/high consistency protocol;
   do not copy an unconditional 64-bit read. Use the portable fallback elsewhere.
7. Validate on Intel and AMD, Windows client/server, and a VM, with the minimum
   and current supported Go versions. Add Windows CI. Exercise clock adjustments,
   sleep/resume, concurrent readers, and boundary conversions. Keep correctness
   tests separate from noisy benchmark gates. Require a repeatable improvement
   in the consuming workload before making a custom path the default.

## Alternatives and tradeoffs

| Option | Meaning | Decision |
| --- | --- | --- |
| `time.Now()` | Wall time plus Go monotonic reading | Default full-value API |
| Shared SystemTime | Coarse wall time in FILETIME units | Candidate scalar fast path |
| Shared InterruptTime | Coarse boot-related elapsed clock | Candidate separate Instant API |
| `GetSystemTimeAsFileTime` | Documented coarse wall API | Useful reference/fallback; call overhead |
| `GetTickCount64` | Millisecond uptime count | Simpler but coarse and foreign-call overhead |
| `QueryInterruptTime` | Coarse interrupt count | Documented elapsed-time option |
| QPC | High-resolution counter with separate frequency | Precise intervals |
| Precise FILETIME / interrupt APIs | Finer wall / interrupt clock | Opt-in precision |
| Background atomic cache | Last published sample | Defer: adds goroutine, wakeups, and scheduler-dependent staleness |
| Direct RDTSC/RDTSCP | Hardware ticks | Reject for default: calibration, migration, and portability burden |

The shared-page addresses are an OS-layout dependency, even though the Go runtime
uses them. This experiment does not establish a stable public Win32 guarantee.
No timer-resolution or power-policy changes were made. A cached clock cannot
promise a hard maximum staleness just by requesting a short ticker interval.

## Sources

- Installed Go: `C:/Program Files/Go/src/runtime/time_windows_amd64.s`,
  `runtime/time_windows.h`, and `time/time.go`. The measured toolchain reports
  `go1.27.0 windows/amd64`. These local sources are the basis for the runtime claim;
  recheck them when changing Go versions. [Upstream runtime source](https://go.dev/src/runtime/time_windows_amd64.s).
- [Microsoft timing guidance](https://learn.microsoft.com/en-us/windows/win32/sysinfo/acquiring-high-resolution-time-stamps):
  clock selection, QPC frequency, virtualization, and guidance against direct TSC.
- [QueryPerformanceCounter](https://learn.microsoft.com/en-us/windows/win32/api/profileapi/nf-profileapi-queryperformancecounter).
- [GetSystemTimeAsFileTime](https://learn.microsoft.com/en-us/windows/win32/api/sysinfoapi/nf-sysinfoapi-getsystemtimeasfiletime).
- [QueryInterruptTime](https://learn.microsoft.com/en-us/windows/win32/api/realtimeapiset/nf-realtimeapiset-queryinterrupttime):
  interrupt-clock units, update cadence, and timer-resolution effects.
