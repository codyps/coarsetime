# Comparison with fastime

A separate [benchmark module](fastime_test.go) pins
[`github.com/kpango/fastime` v1.1.10](https://github.com/kpango/fastime/tree/v1.1.10).
It requires Go 1.24.4 or newer; the main library remains dependency-free with Go
1.23 support. Run this suite explicitly (root `go test ./...` does not enter
nested modules):

```sh
go -C research/fastime test -run '^$' -bench . -benchmem -benchtime=500ms -count=3
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

Fastime comparison after adopting the wall-read optimizations (2026-09-05,
Intel i9-9880H, Go 1.26.6, Darwin amd64 and Docker Linux amd64; medians of three
500 ms runs, platforms run sequentially with no concurrent builds or tests).
The Linux vDSO fast path was explicitly verified before the comparison:

| Serial read | Darwin ns/op | Linux ns/op |
| --- | ---: | ---: |
| `fastime.Now()`, 1 ms updater | 1.964 | 2.034 |
| `fastime.Now()`, 5 ms updater | 1.953 | 2.070 |
| `fastime.UnixNanoNow()`, 1 ms updater | 1.694 | 1.723 |
| `fastime.UnixNanoNow()`, 5 ms updater | 1.688 | 1.698 |
| `coarsetime.Now()` | 3.411 | 12.93 |
| `coarsetime.UnixNano()` | 0.842 | 13.10 |
| `time.Now()` | 76.92 | 40.61 |

| Parallel read (16 logical CPUs) | Darwin ns/op | Linux ns/op |
| --- | ---: | ---: |
| `fastime.Now()`, 1 ms updater | 0.2264 | 0.2341 |
| `fastime.Now()`, 5 ms updater | 0.2164 | 0.2431 |
| `coarsetime.Now()` | 0.3826 | 1.716 |
| `time.Now()` | 6.237 | 3.829 |

[Raw Darwin results](results/darwin.txt) and
[raw Linux results](results/linux.txt) include all
samples. Darwin `coarsetime.UnixNano()` now reads faster than fastime in this
comparison; fastime retains the lower `time.Time` read cost on both platforms.
The clocks have different freshness and correction behavior, as described above.

All reported zero per-operation allocations after rounding/amortization. Earlier
race-instrumented smoke runs exercised the benchmark harness on both platforms,
and vet passed; the adopted library also passed its own race tests. Parallel
ns/op reports aggregate throughput, not the latency of an individual read.
