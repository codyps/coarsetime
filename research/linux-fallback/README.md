# Linux standard-library fallbacks

GitHub-hosted runner measurements from [run 36338781065](https://github.com/codyps/coarsetime/actions/runs/36338781065),
revision `c57f355`, comparing the old raw coarse-clock syscalls with Go clocks.
The change is based on `main`, independently of the ARM64 vDSO PR.

Each entry is the median of ten 500ms samples, in ns/op. All cases allocate
zero bytes. Raw outputs, CPU details, kernel versions, and Go versions are in
[`results/`](results/).

| Runner | Go | syscall Now | Go Now | syscall UnixNano | Go UnixNano | syscall Instant | Go NowInstant |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |
| ubuntu-latest (amd64) | 1.23 | 283.40 | 46.97 | 283.00 | 46.42 | 291.10 | 25.13 |
| ubuntu-latest (amd64) | stable | 241.65 | 57.68 | 237.80 | 56.72 | 248.25 | 31.21 |
| ubuntu-24.04-arm | 1.23 | 177.60 | 72.72 | 174.00 | 71.39 | 175.90 | 39.12 |
| ubuntu-24.04-arm | stable | 177.35 | 72.52 | 174.85 | 71.61 | 175.95 | 38.44 |

Wall fallback reads are about 4.2–6.1x faster on these amd64 runners and 2.4x on
ARM64. The public `NowInstant` Go fallback is 8.0–11.6x faster on amd64 and
4.5–4.6x on ARM64. These are comparisons within each runner, not between CPUs
or Go releases. Runner noise and CPU differences prevent generalizing exact
ratios to every Linux machine.

`SyscallNow` includes `time.Unix` construction; `TimeNow` includes `Round(0)`
to remove the monotonic component required by the API. UnixNano cases include
their respective conversions. `Syscall` for Instant is the former raw read;
`NowInstant` forces the new Go fallback through the public API. `TimeSince`
measures its underlying standard-library operation separately. The raw syscall
implementations remain only in test files as baselines and correctness oracles.

The workflow also runs native tests, purego tests, vet, pointer checks, and
32-bit x86 tests. Normal CI covers race tests and the amd64 vDSO bridge across
supported Go versions. The benchmark workflow additionally runs native race
tests on amd64 and ARM64.

## Clock semantics

Wall fallbacks use `time.Now` and can be selected on each read. Instant's source
is selected once at initialization. With no usable coarse vDSO clock it uses
`time.Since` from a fixed `time.Now()` origin, including on other Linux
architectures. If the coarse source was selected but later fails, the reader
panics rather than switching between boot-relative and process-relative epochs.
This also avoids transitions between coarse and fine resolutions that could
make a later reading appear earlier. Tests force each source, check fallback
timestamps and concurrent monotonic progress, and check the failure policy.

## Reproduce

Run `.github/workflows/linux-fallback.yml` on GitHub runners, or on Linux:

```sh
CGO_ENABLED=0 go test -run '^$' -bench '^BenchmarkLinux(Wall|Instant)Fallback$' -benchmem -benchtime=500ms -count=10 .
```
