# Historical production clock measurements

These are retained measurements from the implementation stages described below.
They are host-specific snapshots, not current cross-platform guarantees.
See the [research index](README.md) for later experiments and API measurements.

## Darwin adoption

Initial commpage adoption (before the wall-read optimizations below), same host
and Go 1.26.6, medians of five 500 ms
runs (all zero allocations):

| Operation | ns/op |
| --- | ---: |
| `NowInstant()` | 0.628 |
| `Since()` | 3.651 |
| `UnixNano()` | 2.358 |
| `Now()` | 5.050 |

[Raw adoption measurements](darwin/commpage/adopted.txt).

The subsequent [wall-read optimizations](wall-optimizations/README.md#adoption)
combine the startup support check with a unit-timebase check, outline the generic
fallback, and simplify time construction for nonnegative Unix timestamps. On the
same host with Go 1.26.6, five-run medians for the current public APIs are 0.811 ns
for `UnixNano`, 3.277 ns for `Now`, and 2.482 ns for `Instant.Time`, all without
allocations. Negative timestamps and unexpected timebase ratios retain correct
conversion. These changes preserve the explicit refresh requirement.

## Earlier baselines

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


## Linux wall-read adoption

The amd64 wall APIs fuse vDSO argument setup with the read and construct
`time.Time` directly from the returned timespec. In the Go 1.26.6 adoption
benchmark, `Now` improved from 18.35 to 13.65 ns and `UnixNano` from 16.68 to
13.26 ns, with zero allocations. See the
[wall-read comparison](wall-optimizations/README.md#adoption).
