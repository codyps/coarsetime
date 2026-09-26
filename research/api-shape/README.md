# Shared public clock API

The starting point is commit `d2ab9bf403286771882dec8952dbe5106ed143fe`.
All platforms already exposed the same names and signatures, but public wall API
bodies and documentation were duplicated across build-selected files. The
refactor defines `Now`, `UnixNano`, and `Instant.Time` once in `wall_api.go` and
calls private `readWallTime`, `readWallUnixNano`, and `instantTime` implementations.
Elapsed-time APIs and calibration were already shared. There are no new exported
platform APIs, runtime interfaces, allocations, or background workers.

The specialized Darwin unsigned conversion and Linux timespec-to-time conversion
remain intact. Windows keeps its FILETIME reader and purego fallback; the unused
Windows nanosecond fallback is no longer compiled into non-Windows builds.
Clock sources, cached-correction requirements, suspend behavior, and wall-time
semantics are unchanged. The README now distinguishes the uniform API from those
platform-dependent properties. Research detectors remain isolated in their own
module; prototype readers remain behind explicit experiment build tags.

## Direct-call performance

Darwin amd64, Intel i9-9880H, Go 1.26.6. Seven 300 ms samples per operation,
GOMAXPROCS=1, alternating before/after order each round, with no concurrent build,
test, or benchmark jobs. Values are median ns/op; every sample reports zero bytes
and allocations. Binaries were built before measurement. The baseline and final
binaries have different test/link layouts, so small timing shifts are not evidence
of an optimization.

| Operation | Before | After |
| --- | ---: | ---: |
| `NowInstant` (unchanged control) | 0.5556 | 0.5505 |
| `Since` (unchanged control) | 3.665 | 3.251 |
| `Now` | 2.938 | 2.714 |
| `UnixNano` | 0.7398 | 0.6701 |
| `Instant.Time` | 2.195 | 2.185 |

No measured direct-call regression. The unchanged `Since` control also improved,
so these results should not be described as an abstraction-induced speedup.
[Raw before](before.txt), [raw after](after.txt).

The initial wrapper exceeded Darwin's inlining budget: the private reader cost
80 and the public wrapper cost 83 against a budget of 80. Its first sequential
pilot measured UnixNano at 0.7594 -> 1.722 ns. Keeping the correction load and tick
load in one left-to-right expression reduces the costs to 75 and 78, respectively,
and preserves the required correction-before-ticks ordering. The table above is
from the corrected implementation, not that discarded pilot.

The [compiler report](inlining.txt) confirms that all three public wrappers can
inline on Darwin, Linux, and Windows amd64. Darwin benchmark disassembly shows
no additional call layer: `Now` and `Instant.Time` call the renamed private bodies,
and UnixNano retains its inlined fast path. See [before](before-disassembly.txt)
and [after](after-disassembly.txt) disassembly. Calls through function values
cannot rely on that wrapper inlining and are measured separately below.

## Calls through stored function values

Seven alternating 300 ms samples, same machine/settings, zero allocations:

| Operation | Before ns/op | After ns/op | Difference |
| --- | ---: | ---: | ---: |
| `Now` callback | 3.116 | 3.743 | +0.627 ns (+20%) |
| `UnixNano` callback | 2.035 | 1.840 | -0.195 ns |
| `Instant.Time` method expression | 2.470 | 3.575 | +1.105 ns (+45%) |

The generic `Now` and `Instant.Time` wrappers retain an extra call when reached
indirectly. This is a measured cost of centralizing their definitions. UnixNano's
private Darwin fast path inlines into its public wrapper, so it has no additional
call layer even in this case; the small measured decrease is not a promised gain.
Direct-call inlining results must not be generalized to callbacks or builds that
disable inlining. [Raw before](callback-before.txt), [raw after](callback-after.txt).

## Validation

- Native Darwin root tests, purego tests, race tests, vet, and purego vet pass.
- The source API guard passes for 12 OS/architecture pairs with and without
  purego, requiring the public declarations to live in the shared files.
- All 24 test-binary cross-compilations pass; see [target list](cross-compile.txt).
- Linux and Windows runtime performance was not measured. Docker's OrbStack
  socket was unavailable, and there is no Windows runtime in this session.
  Compiler evidence and cross-compilation do not replace those measurements.
- No live Linux VVAR, time-namespace, or additional hardware claim is made.

## Reproduction

Build the baseline from the commit above and the changed source with
`GOCACHE=/tmp/coarsetime-api-cache go test -c -o /tmp/coarsetime-api-LABEL.test`.
Run each binary with:

```sh
/tmp/coarsetime-api-LABEL.test -test.run '^$' \
  -test.bench 'Benchmark(Now$|UnixNano$|InstantTime$|NowInstant$|Since$)' \
  -test.benchmem -test.benchtime=300ms -test.count=1 -test.cpu=1
```

Repeat seven rounds, alternating which binary runs first. Do not build or run
other tests during the measurements. Function-value benchmarks require copying
the updated `coarsetime_bench_test.go` into the baseline checkout before building;
select them with `-test.bench 'FuncValue$'` using the same settings and ordering.
Binary and source hashes are recorded in [provenance](provenance.json).
