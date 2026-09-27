# Native Darwin amd64 benchmarks, 2026-09-27

Current production code at `408e5aefbb0a6a541ae07cffd4c00e7ca7aac6a3`, measured on an Intel Core i7-4960HQ (2.60 GHz), macOS 15.7.9 (24G830), Go 1.26.6. Default and purego tests passed; default tests required a usable live calendar snapshot. No production code was changed.

Five 500 ms samples per case, prebuilt binaries, GOMAXPROCS=1 and `-test.cpu=1`. Default/purego/synthetic suite order reversed on alternating rounds; benchmark order within each suite was fixed. No concurrent build or test jobs were launched during measurement. These are hot-loop throughput observations, not serialized clock latency or freshness bounds. Parallel cases here measure the single-worker harness, not multicore scaling.

All samples reported zero bytes and zero allocations per operation. Values below are median ns/op with minimum–maximum in parentheses. ClockProgress is a sampling diagnostic, not a read-latency benchmark.

| Benchmark | Default | purego |
| --- | ---: | ---: |
| NowInstant | 0.6667 (0.6619–0.7958) | 36.17 (35.73–41.76) |
| Since | 3.721 (3.666–4.221) | 38.18 (37.61–39.12) |
| TimeSince | 36.61 (36.44–38.38) | 36.79 (36.24–37.18) |
| TimeNow | 77.14 (76.71–82.31) | 77.8 (76.2–92.09) |
| NowInstantParallel | 1.799 (1.787–1.889) | 35.82 (35.2–36.33) |
| TimeSinceParallel | 37.1 (36.02–38.36) | 36.09 (35.75–36.82) |
| ClockProgress | 3.87 (3.679–4.672) | 38.63 (38.12–42.85) |
| Now | 12.99 (11.99–13.68) | 81.25 (80.23–94.29) |
| UnixNano | 9.209 (8.792–9.704) | 79.36 (78.28–90.62) |
| NowParallel | 13.43 (13.1–15.38) | 81.27 (79.65–96.78) |
| NowFuncValue | 13.29 (13.2–15.04) | 83.37 (82.3–95.67) |
| UnixNanoFuncValue | 9.578 (9.54–11.18) | 79.67 (79.17–93.83) |
| DarwinCalendarRead | 7.906 (7.694–9.33) | N/A |
| DarwinStandardUnixNano | 79.02 (78.11–84.85) | N/A |

Calendar diagnostic fallback: median 0.001857%, range 0.000952–0.029000%. This benchmark includes fallback bookkeeping; compare public UnixNano with DarwinStandardUnixNano for API cost.

Synthetic calendar read: 7.759 ns/op (7.726–7.96); Go-owned memory excludes live OS publication and fallback effects.

## Reproduce

Build with the repository Nix development shell (`nix develop`):

```sh
COARSETIME_REQUIRE_DARWIN_WALL=1 go test ./...
go test -tags=purego ./...
go test -c -o /tmp/coarsetime-darwin-amd64.test .
go test -tags=purego -c -o /tmp/coarsetime-darwin-amd64-purego.test .
go test -c -o /tmp/coarsetime-darwinwall.test ./internal/darwinwall
```

Run each binary once per round for five rounds, reversing suite order on alternating rounds:

```sh
GOMAXPROCS=1 /tmp/coarsetime-darwin-amd64.test -test.run='^$' -test.bench=. -test.benchmem -test.benchtime=500ms -test.count=1 -test.cpu=1
```

Substitute the purego and synthetic binary paths for their runs. [Provenance](provenance.json) records exact invocation order and binary hashes. Raw output: [default](default.txt), [purego](purego.txt), [synthetic](synthetic.txt). [Summary JSON](summary.json) retains all samples and diagnostic metrics. Historical cached-wall prototypes were not rerun; they require the older API checkout described in the research index.
