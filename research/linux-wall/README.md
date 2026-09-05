# Further Linux wall-read investigation

2026-09-05, Intel i9-9880H, Docker Linux amd64, kernel
`7.0.14-orbstack-00380-ga7e0a2dc9535`, Go 1.26.6. Production remains unchanged.
The `linuxwallprobe` build tag enables the diagnostic assembly and benchmarks.

Five sequential 500 ms samples per candidate, no concurrent build/test jobs,
vDSO explicitly required and verified. Medians from [raw results](probe.txt):

| Candidate | ns/op |
| --- | ---: |
| Current public Now | 13.49 |
| System-stack bridge and time construction, dummy clock | 12.41 |
| Narrow nonnegative nsec to uint32 before time.Unix | 13.60 |
| Atomic pointer to immutable time.Time, active 1 ms updater | 0.8759 |

The dummy callback writes a fixed valid timespec and returns success. It omits
vDSO clock dispatch and shared kernel data access but retains argument setup,
asmcgocall and time.Unix construction. This is a diagnostic comparison, not a
precise additive attribution of instruction costs. The bridge and surrounding
work dominate. Narrowing conversion provides no useful improvement.

The cache publishes a fresh `Now()` result through atomic.Pointer every timer
tick. Reads load one immutable time.Time. It has deliberately different
semantics: a goroutine/timer, one heap allocation per publication, extra cache
staleness, and delayed observation of clock corrections. Scheduling delays mean
1 ms is not a staleness bound. Benchmarks exclude initialization/shutdown and
report zero amortized bytes/allocations per read; this does not mean the updater
is allocation-free. No accuracy or idle CPU measurement was made.

## What could improve the existing API

The current path already avoids the clock_gettime syscall. It invokes the
versioned vDSO entry through Go's system-stack bridge. The local Go 1.26.6
`src/runtime/asm_amd64.s` implementation of asmcgocall saves goroutine state,
switches to g0, aligns the C stack, calls a landing pad, and restores the stack.
Go's own `src/runtime/time_linux_amd64.s` clock path instead uses specialized
runtime assembly, with access to private runtime layout and profiling state.
A specialized bridge is possible research work, but would require maintaining
that integration across Go releases. This probe does not establish a safe
replacement bridge or a measured speedup for one.

Directly reading Linux's VVAR clock data could avoid this bridge, analogous to
the Darwin commpage read. The kernel's
[v6.12 coarse reader](https://github.com/torvalds/linux/blob/v6.12/lib/vdso/gettimeofday.c#L218)
loads seconds/nanoseconds under a sequence counter and handles time namespaces.
However, the data symbol is hidden and its layout is an implementation detail:
compare [v6.12 vdso_data](https://github.com/torvalds/linux/blob/v6.12/include/vdso/datapage.h)
with [current vdso_clock and vdso_time_data](https://github.com/torvalds/linux/blob/master/include/vdso/datapage.h).
The current tree separates clock records, adds auxiliary clock data, and includes
architecture-dependent data. Config-dependent fields also affect offsets.
The supported entry point is the [versioned vDSO function](https://man7.org/linux/man-pages/man7/vdso.7.html).
Reading guessed offsets or accepting a one-time matching timestamp would not
establish compatibility. No VVAR reader was implemented or benchmarked here.

## Recommendation

Keep the current default semantics. For workloads that need fastime-class read
cost, expose an explicit cached-clock object with an interval and Close method,
or let callers reuse a timestamp within a batch. That makes the added staleness
and updater lifetime visible. The measured immutable snapshot is a feasibility
prototype; a public API would also need lifecycle tests and an allocation policy.
If preserving per-call kernel freshness is essential, investigate a specialized
runtime bridge or a rigorously gated VVAR reader separately, accepting a larger
compatibility and maintenance burden. Arithmetic tweaks alone cannot remove
the dominant cost measured here.

## Reproduce

```sh
GOCACHE=/tmp/coarsetime-prototype-go-cache GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
  go test -tags linuxwallprobe -c -o /tmp/coarsetime-linux-wall-probe.test .
docker run --rm --network none -e COARSETIME_REQUIRE_VDSO=1 \
  -v /tmp/coarsetime-linux-wall-probe.test:/test:ro python:3.12-slim \
  /test -test.run 'Test(LinuxWallProbe|VDSO)$' -test.v \
  -test.bench '^BenchmarkLinuxWall' -test.benchmem -test.benchtime=500ms -test.count=5
```

Correctness checks verify the dummy bridge output and bracket the conversion
candidate with realtime syscalls. The cache benchmark runs its updater alongside
reads; race-instrumented smoke benchmarks cover all four candidates.
