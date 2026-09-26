# Further wall-read optimization experiments

Prototype sources live under `research/_prototypes/` and run through the
[research overlay runner](../README.md#same-package-prototypes).


These are opt-in prototypes under `-tags walloptprototype`. The default library
is unchanged. The candidates preserve the current clock source, correction
refresh policy, Local time zone, and absence of a Go monotonic component. No
background updater, periodic allocation, cached second, or private time.Time
representation is introduced.

## Candidates

### Darwin amd64

The current wall reader uses `scaleTicks(ticks, clockNumer, clockDenom, ...)`.
The disassembly confirms it loads both factors and compares them on every read,
even though XNU's Intel timebase is 1:1. The **unit-timebase** candidate simply
adds native ticks and the correction, with the same unsigned wrap semantics.
The support check and libc fallback remain intact. This specialization is not
appropriate for ARM64, whose timebase is not generally 1:1.

**Flat** combines the tick read, correction load, and time.Time construction in
one function. **FlatUnsigned** additionally uses unsigned division/remainder
for nonnegative UnixNano values; negative epochs use the original time.Unix
conversion. This avoids signed division corrections on today's usual path.
All integer division by constant 1e9 here is compiler-generated reciprocal
multiplication; it is not a hardware division instruction on the measured path.

**SignedNow** pre-normalizes with signed quotient/remainder, then passes already
normalized values to time.Unix. **UnsignedNow** uses unsigned normalization but
retains the current UnixNano reader. These isolate normalization from the
clock-reader change. **UnitUnsignedNow** combines two helpers rather than
flattening them, to test the effect of function boundaries.

The **Inline** candidate attempts to fuse the atomic clock/correction loads.
Despite its name, its local-variable form exceeds Go 1.26.6's inlining budget:
small source changes are not enough to guarantee elimination of the call.
The measured disassembly, not the function's name, determines the mechanism.

The existing Instant.Time conversion is also compared with a unit-timebase,
unsigned-normalization version. It uses the same cached correction and date
range as the current implementation.

### Linux amd64

**TimespecNow** constructs time.Time directly from the seconds/nanoseconds
already returned by CLOCK_REALTIME_COARSE, avoiding conversion to a scalar and
back. Its syscall fallback remains the current code, including error handling.

**FusedNow** also folds the vDSO argument preparation into the wall reader,
avoiding the generic helper call and result copy. **FusedUnixNano** isolates that
fusion for scalar output. Both retain the current runtime.asmcgocall system-stack
bridge, C ABI, function resolver, clock ID, and syscall fallback. They do not
attempt to call the vDSO on a Go goroutine stack or read private Linux vvar data.

## Validation

Native Darwin amd64 and Docker Linux amd64, Go 1.26.6. Common normalization is
checked against time.Unix for 100,000 random int64 values plus signed-range and
second boundaries, comparing complete time.Time values. Darwin readers are
bracketed by current Now samples and exercised during concurrent calibration
and GC. The fused direct-reader fallback is forced with its commpage pointer
set to nil, so accidentally taking the direct path would fail.

Linux readers are bracketed by CLOCK_REALTIME_COARSE syscalls with vDSO both
active and forced off. Native Linux tests explicitly require vDSO availability.
Race and vet passed on both platforms; Darwin strict checkptr also passed.
The complete library tests run alongside prototype tests. No OS clock-setting
or suspend experiments were performed because these candidates retain the
existing clock source and refresh behavior.

ARM64 performance was not measured. These architectural specializations target
amd64; the shared normalization helper remains independently testable.

## Benchmark method

The [runner](run.py) executes already-built CGO_ENABLED=0 binaries in five
shuffled serial rounds, requesting 300 ms per case. Darwin finishes before Linux
starts. All compilation and verification completes before timing. Linux runs in
a network-disabled container and asserts vDSO availability in every benchmark
process. All reported candidates are required to allocate zero bytes/objects.

These measurements report hot-loop throughput with results consumed by the same
global sinks; they do not promise an individual call latency, portable speedup,
or clock accuracy. Helper boundaries and compiler inlining are deliberately part
of the measurement, since they affect real API calls.

Sources: [Darwin readers](../_prototypes/wall-optimizations/wall_opt_prototype_darwin_amd64.go),
[Linux readers](../_prototypes/wall-optimizations/wall_opt_prototype_linux_amd64.go), and
[normalization](../_prototypes/wall-optimizations/wall_opt_prototype.go). Local generated-code evidence
is in [Darwin disassembly](darwin-disassembly.txt) and
[Linux disassembly](linux-disassembly.txt). Results are retained under `darwin/`
and `linux/`. The [Darwin pilot](darwin-pilot.txt) is exploratory and should not
be mixed with the shuffled final samples.

## Follow-up: fully outlined Darwin fallback

**ColdUnixNano** checks support first. Its supported branch reads correction
before ticks; the unsupported branch calls an out-of-line helper that reads
correction before its libc clock call. Moving the complete fallback allows the
Go 1.26.6 reader to fit the inlining budget exactly (cost 80). The
[follow-up disassembly](darwin-cold-disassembly.txt) confirms there is no call on
the supported scalar path. Both branches preserve the original acquisition order.

ColdNow uses ordinary time.Unix conversion; ColdUnsignedNow uses the same
nonnegative normalization as FlatUnsignedNow. The follow-up is a separate five-
round shuffled run with fresh current-API controls. Its raw results and summary
are under `darwin-cold/`; the initial results under `darwin/` are not overwritten.
Only the Darwin binary changed for this follow-up.

## Results: 2026-09-05

Intel i9-9880H host, Go 1.26.6; Linux runs in Docker on that host. Medians in
ns/op. The current and candidate values in each row come from the same run.

| Platform / operation | Current | Candidate | Reduction |
| --- | ---: | ---: | ---: |
| Darwin Now | 4.8950 | 3.2600 | 33.4% |
| Darwin UnixNano | 2.3040 | 0.8135 | 64.7% |
| Darwin Instant.Time | 3.0950 | 2.4200 | 21.8% |
| Linux Now | 17.9400 | 13.1400 | 26.8% |
| Linux UnixNano | 16.0500 | 12.7600 | 20.5% |

All timed fast-path cases report zero bytes and allocations. Fallback
correctness is tested separately; fallback allocation counts were not measured.
No per-second rebasing allocation or background work is introduced.

Useful negative/control results:

* Darwin normalization alone (unsigned): 5.051 ns/op.
* Darwin unit-timebase helper alone: 4.623 ns/op.
* Darwin flattened unit-timebase with original normalization: 3.843 ns/op.
* Darwin fully outlined scalar fallback, original normalization: 3.830 ns/op.
* Linux timespec preserved, generic vDSO helper retained: 15.860 ns/op.
* Linux unsigned normalization alone: 17.830 ns/op.

Changing normalization alone is not the main win. The useful combination removes
unnecessary conversion and helper boundaries. The Darwin no-allocation Now path
is close to the earlier split/rebase prototype's 3.14 ns, without its publication
and periodic allocation costs; those numbers are from separate runs, so their
small difference should not be ranked confidently.

## Recommended implementation work (subsequently adopted)

1. On Darwin amd64, specialize wall-time conversion for nanosecond Mach ticks
   and fully outline the unsupported branch. For production, the startup fast
   flag can combine approximate-clock support with a verified 1:1 timebase;
   an unexpected ratio can retain the current generic conversion fallback.
2. Construct Darwin time.Time in the same function, using unsigned normalization
   for nonnegative timestamps and the normal conversion for negative ones.
3. On Linux amd64, preserve the vDSO timespec for Now and combine argument setup
   with the read for both Now and UnixNano. Retain the existing runtime bridge
   and syscall fallback.

These are lower-complexity candidates than a mutable split-second cache. They
preserve current wall freshness and correction semantics. These recommendations were subsequently adopted as described below. This experiment used only Go 1.26.6; the ARM64
build was checked but its execution and performance were not measured.

The original prototype experiment changed no default implementation files.

## Reproduce

From the project root:

```sh
export GOCACHE=/tmp/coarsetime-prototype-go-cache
python3 research/prototype.py wall-optimizations test -race .
python3 research/prototype.py wall-optimizations vet .
CGO_ENABLED=0 python3 research/prototype.py wall-optimizations test -c -o /tmp/coarsetime-wallopt-darwin.test .
python3 research/wall-optimizations/run.py /tmp/coarsetime-wallopt-darwin.test --output /tmp/wallopt-darwin
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 python3 research/prototype.py wall-optimizations test -c -o /tmp/coarsetime-wallopt-linux.test .
```

Run the Linux binary and runner on Linux with `COARSETIME_REQUIRE_VDSO=1` and
`--linux`. The runner uses only Python's standard library. For the narrow Darwin
follow-up use `--filter '^BenchmarkWallOpt(Cold.*|CurrentNow|CurrentUnixNano|FlatUnsignedNow)$'`.

Output trailing whitespace is removed before saving the final research artifact;
measurements and instruction contents are unchanged.

## Adoption

The default Darwin amd64 APIs now use the outlined fallback and unsigned
normalization. A startup flag combines commpage availability with a verified
unit timebase. Unexpected timebases retain generic scaling; unsupported clocks
retain the libc fallback. `Instant.Time` also specializes unit-timebase conversion.
Linux amd64 now constructs `Now` directly from the vDSO timespec and fuses
argument setup with the bridge call for both APIs. The runtime system-stack
bridge, syscall fallback, and wall correction/refresh semantics are preserved.
Other architectures retain their previous implementation.

Public API comparison against commit `794a710`, Go 1.26.6, same Intel host;
Linux runs in Docker's Linux VM. Five 300 ms samples, alternating baseline and
adopted binaries in seeded shuffled order, platforms run serially. Medians:

| API | Darwin before | Darwin adopted | Linux before | Linux adopted |
| --- | ---: | ---: | ---: | ---: |
| Now | 5.029 | 3.277 | 18.35 | 13.65 |
| UnixNano | 2.315 | 0.8106 | 16.68 | 13.26 |
| Instant.Time | 3.080 | 2.482 | 3.076 | 3.214 |

Units are ns/op; every sample reports zero bytes and allocations. Linux
`Instant.Time` is unchanged; its small measured variation illustrates noise.
One late Darwin sample and late Linux samples were slower; raw samples are
retained, without filtering. These are local microbenchmarks, not performance
guarantees. The Darwin `UnixNano` inlining cost remains exactly 80 on Go 1.26.6.

See [Darwin raw results](adopted-darwin.txt), [Linux raw results](adopted-linux.txt),
and [comparison runner](benchmark-adopted.py). The runner expects baseline and
adopted test binaries at `/tmp/coarsetime-{baseline,adopted}-{darwin,linux}.test`.
Build the baseline from `794a710` and the adopted binaries from this worktree
using `go test -c`; cross-build Linux with `GOOS=linux GOARCH=amd64 CGO_ENABLED=0`.
Linux benchmark processes require and verify the vDSO fast path.

Validation: Darwin race tests with all three prototype tags, default checkptr
and vet; Linux race tests with walloptprototype and vet in Go 1.26.6, plus default
execution requiring the vDSO and forcing the syscall fallback. Cross-builds pass
for Darwin arm64, Linux 386, and Windows amd64. New default Darwin tests cover
signed timestamp boundaries, randomized Instant conversion, forced unavailable
commpage access, and unexpected timebase ratios. No ARM64 performance claim is made.

Go 1.23.12 compatibility also passes: native Darwin race tests, Linux amd64
cross-build and full execution with the vDSO required, including forced syscall
fallback tests. Darwin `UnixNano` also inlines at cost 80 with Go 1.23.12.
The timing table above uses Go 1.26.6 only.
