# Continuous benchmarking options

Investigation dated 2026-09-27. This is a proposed design, not an installed
service or an implemented workflow. Service availability and pricing were checked
against the linked documentation; no service was trialed and no new performance
measurements were collected.

The subsequent [implemented workflow](../benchmarks/README.md) uses a repository
history branch and GitHub Pages instead of a hosted tracking service. This document
retains the original options analysis; consult that workflow guide for current
behavior and the initial suite's scope.

## Recommendation

Use native GitHub Actions runners for broad OS/architecture coverage, ordinary
Go benchmarks for measurement, and Bencher Cloud for searchable history and
charts. Keep raw samples and a portable JSON summary independently of the service.
A GitHub Pages dashboard is a reasonable alternative if avoiding another service
account is more important than built-in analysis.

Start with informational reports. Establish noise levels before making performance
a required check. Add a controlled Linux runner if small regressions cannot be
resolved on hosted CI. Dedicated macOS and Windows hosts can follow if needed.
No reviewed self-service benchmark offering supplies the entire six-platform
matrix on controlled hardware out of the box.

There are three distinct comparisons:

1. **Against stdlib:** measure coarsetime and its stdlib alternative in the same
   session with the same Go version; report both costs and their ratio.
2. **Between commits:** rebuild the baseline and candidate with one toolchain,
   then alternate their execution on the same runner.
3. **Between Go versions:** rebuild one fixed commit and benchmark harness with
   each exact toolchain, then alternate execution on the same runner. Separate
   matrix jobs alone do not isolate compiler/runtime effects from machine effects.

## Existing repository coverage

- `coarsetime_bench_test.go` already measures `NowInstant`, `Since`, `Now`,
  `UnixNano`, `time.Now`, and `time.Since`, plus some parallel and callback cases.
- A portable `time.Now().UnixNano()` benchmark is missing. A Darwin-specific
  equivalent exists, but cannot provide the common cross-platform baseline.
- Matching parallel and callback alternatives are incomplete. Stored-reading
  subtraction is described in the README but lacks a paired public benchmark.
- `.github/workflows/test.yml` tests five native runner types. Its ARM64
  benchmark alternates a fixed historical syscall revision and the current
  implementation, but that answers an adoption question rather than tracking
  general regressions.
- `.github/workflows/linux-fallback.yml` samples Linux amd64/arm64 across the
  minimum and stable Go versions. It retains raw artifacts, without a unified
  public-API dashboard or durable comparison history.
- Research captures have valuable provenance, but describe different revisions,
  machines, harnesses, and toolchains. They should remain historical evidence,
  not be combined into a current platform ranking.

## Services and tools

| Option | What it provides | Fit and limitations |
| --- | --- | --- |
| [Bencher](https://bencher.dev/docs/how-to/track-benchmarks/) | History, branches, testbeds, graphs, statistical and relative regression checks; accepts existing benchmark output | Recommended tracking layer. Keep each environment/toolchain/build mode separate. Requires project setup and CI publishing credentials. |
| [Bencher on-demand runners](https://bencher.dev/pricing/) | Controlled execution on Linux x86_64 (`intel-v1`); Firecracker microVM isolation | Useful additional Linux reference. Free public tier has one concurrent job and a five-minute timeout. ARM64 is listed as coming soon; dedicated/custom macOS and Windows are enterprise options. The sandbox kernel and clocksource still matter for this library. |
| [CodSpeed](https://codspeed.io/docs/benchmarks/go) | Go integration, PR reporting, profiling, walltime measurement | Go support is marked early development, walltime-only, with only `-bench` documented as a supported `go test` flag. Validate sample count, CPU setting, build tags, supported toolchains, and bridge verification before adoption. |
| [CodSpeed Macro Runners](https://codspeed.io/docs/features/macro-runners) | Managed bare-metal Linux ARM64 Graviton and x64 Ryzen runners | Attractive for controlled Linux measurements; not a documented self-service six-platform matrix. Graviton includes 600 free minutes/month; Ryzen Pro access requires contact and lists $0.06/min. |
| [github-action-benchmark](https://github.com/benchmark-action/github-action-benchmark) | Parses Go results, retains history on a branch, charts on GitHub Pages, percentage alerts | Best lightweight alternative. It does not supply runners or remove noise. We would own sample aggregation, uncertainty, ratios, environment partitioning, and serialized publication from matrix jobs. |
| [benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat) | Local/CI sample summaries, confidence intervals, comparisons | Recommended analysis tool regardless of storage provider. It is not hosting or a historical dashboard. Pin its version separately from the tested Go toolchains. |

[Bencher's Go adapter](https://bencher.dev/docs/explanation/adapters/#-go-bench)
imports latency, but does not supply confidence bounds. Use its custom JSON format
for explicitly computed summaries, allocations, and stdlib ratios when needed;
verify repeated-name handling before feeding multi-sample output to any adapter.

[Bencher pricing](https://bencher.dev/pricing/) currently makes public projects
free, with a 65,535-metric daily limit for the free plan. Its paid Pro plan starts
at $100/month; runner time is separate. [CodSpeed pricing](https://codspeed.io/pricing)
lists a three-month free history and unlimited history on Pro. Retention matters
for comparisons spanning multiple Go releases, so preserve our own samples.

## Native coverage

These labels are documented in the
[GitHub-hosted runner reference](https://docs.github.com/en/actions/reference/runners/github-hosted-runners).
Standard runners are free for public repositories. Explicit OS labels avoid
`latest` switching OS families, but do not freeze image updates or physical CPUs.

| Target | Proposed runner | Measurement role |
| --- | --- | --- |
| Linux amd64 | `ubuntu-24.04` | vDSO fast path plus separate purego control |
| Linux arm64 | `ubuntu-24.04-arm` | vDSO fast path plus separate purego control |
| Darwin amd64 | `macos-15-intel` | Mach/calendar paths plus purego control |
| Darwin arm64 | `macos-15` | Mach/calendar paths plus purego control |
| Windows amd64 | `windows-2025` | Shared-page paths plus purego control |
| Windows arm64 | `windows-11-arm` | Current stdlib fallback, still worth quantifying |

Add Linux/Windows 386 execution on x64 hosts after validating compatibility;
label it as a 32-bit process on an x64 host. Other targets require additional
native hosts. Cross-compilation and QEMU execution establish coverage, not
representative timing on the target hardware. Unsupported combinations should
say "not measured" rather than inherit another platform's numbers.

The first matrix should use Go 1.23 and the current stable release, resolved to
exact patch versions and recorded. A weekly run should cover each supported Go
minor from 1.23 onward, using a checked-in list of exact patch versions. Update
that list explicitly and retain old release snapshots. A separate rolling-stable
lane can detect new releases; never mix different resolutions of `stable` into
one supposedly fixed-toolchain series. Optional prerelease runs stay advisory.

## Benchmark contract

| Operation | coarsetime | Standard-library baseline |
| --- | --- | --- |
| Capture elapsed start | `NowInstant()` | `time.Now()` |
| Elapsed duration | `Since(start)` | `time.Since(start)` |
| Wall timestamp | `UnixNano()` | `time.Now().UnixNano()` |
| Calendar time | `Now()` | `time.Now()` |
| Subtract stored readings | `end.Sub(start)` | `time.Time.Sub` using monotonic-bearing values |

`NowInstant` captures only elapsed time; `Now` omits Go's monotonic component.
These are task alternatives, not identical precision or semantics. Optionally
include `time.Now().Round(0)` as a secondary calendar baseline, while preserving
the familiar `time.Now()` comparison. Keep clock granularity/repetition diagnostics
separate from speed: neither ns/op nor `BenchmarkClockProgress` establishes an
accuracy or maximum-staleness bound.

Use direct calls in each timed loop and consistent result consumption. Do not
introduce a function-pointer dispatch table into primary benchmarks: dispatch and
inlining differences can dominate these very small operations. Keep callback
benchmarks as their own series with equivalent stdlib callbacks. Retain a common
Go-1.23-compatible loop harness across toolchains; changing to `b.Loop` should be
a separately evaluated harness revision, not an implicit Go-version difference.

Consider a shared external-package harness (`package coarsetime_test`) for public
claims, so calls reflect a consumer importing the package. Keep internal-path
diagnostics in the existing package. Version the harness and ensure baseline and
candidate use identical benchmark source; do not mistake a harness change for a
library change. Old commits lacking an API should be marked unavailable.

## Measurement and attribution protocol

1. Build all comparison binaries before measuring. Validate native fast paths
   separately, using the existing Linux and Darwin required-path tests. These
   checks must actually execute: `-run '^$'` skips them. Record the result and
   distinguish fallback measurements; Darwin validation does not prove every
   later wall read avoids fallback.
2. Use optimized builds, with consistent CGO/build settings, without race,
   checkptr, tracing, or profiling instrumentation in the timing run. Warm each
   binary once and discard that warmup. Run one benchmark process at a time on
   a worker.
3. Start with 10 measured rounds at 500 ms per benchmark, `-cpu=1`, `-benchmem`.
   Alternate baseline/candidate order each round; similarly alternate the order
   of paired stdlib/coarsetime cases. Fix the sampling budget before examining
   results. Longer runs can be a separately defined confirmation protocol.
4. Use benchstat for medians, intervals, and changes between revisions. Its
   guidance recommends at least 10 samples, ideally 20. Many comparisons create
   false positives; statistical significance alone is insufficient for a gate.
   [benchstat documentation](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat)
5. Report absolute ns/op for both implementations, allocations, and speedup
   (`stdlib ns/op / coarsetime ns/op`, larger is better). Compute a ratio in each
   paired round and summarize the ratios with an interval. Preserve pair IDs for
   paired resampling. Ratios reduce some shared noise but do not cancel different
   clock paths, CPU responses, or all scheduling effects.
6. Repeat a fixed commit across fresh jobs to characterize between-job variation.
   Ten samples in one VM describe within-job noise, not fleet variability.
   Assess code deltas using same-job baseline/candidate measurements; use the
   long-term chart as context, not as the sole regression test.
7. Keep parallel throughput separate, with matched stdlib benchmarks and explicit
   `GOMAXPROCS` values such as 1 and 2. Parallel ns/op is aggregate throughput
   expressed per operation, not individual-call latency. Avoid varying the CPU
   count with each runner's default.

An initial unpaired smoke command for the existing portable serial subset is:

```sh
go test -run '^$' -bench '^Benchmark(NowInstant|Since|TimeSince|TimeNow|Now|UnixNano)$' -benchmem -benchtime=500ms -count=10 -cpu=1 .
```

This command alone is not the proposed comparison workflow; it still lacks the
portable UnixNano stdlib case, paired execution, metadata, and publication.

Record commit/base SHA, harness revision, exact `go version`, GOOS/GOARCH,
GOAMD64/GOARM settings where relevant, GOEXPERIMENT, build tags/flags, CGO,
GOMAXPROCS, CPU model, OS/kernel build, runner label/image version, clocksource
where available, fast-path validation, UTC time, CI run URL, and round/order.
Retain raw Go output alongside structured records.

Partition history by OS/arch, environment/CPU class, exact Go version, build mode,
CPU count, and harness revision. Annotate image/kernel changes and establish new
baselines when environments materially change. Keep commit SHA as the history
axis, not part of the testbed identity. OS comparisons describe the measured
OS/hardware combinations; they cannot isolate an OS effect across different CPUs.

## Rollout and published results

1. Complete the paired public benchmark suite and a portable runner/parser.
2. Add an independent benchmark workflow: main pushes and manual runs on the six
   targets; weekly full Go-version coverage and purego controls. On PRs, start
   with stable Go and same-runner base/head comparisons, expanding when useful.
   Preserve the ARM64 bridge correctness/profile regression checks if consolidating
   old experiment jobs.
3. Publish raw artifacts and a job-summary table first. A central publication job
   can then upload to Bencher or update Pages without matrix write races. Fork
   PR measurement jobs should not need service credentials; publish trusted main
   results separately.
4. Generate a current results page with one row per target/toolchain/operation:
   coarsetime median and interval, stdlib median and interval, speedup, allocations,
   commit, date, environment, and raw-data link. Link it from the README. Retain
   immutable release snapshots instead of committing new numbers on every push.
5. After a pilot across multiple fresh runners, set practical regression thresholds
   using measured noise and absolute as well as percentage cost. Keep uncertain
   results advisory; use controlled hosts for small changes requiring resolution.

For sizing, seven unique serial cases covering the four clock-reading operations
need about 35 seconds of timed work per binary at 10 x 500 ms. Two revisions across
six targets and two toolchains imply about 14 runner-minutes of timed work before
calibration, warmup, setup, tests, and builds. Stored-subtraction cases, purego,
parallel tests, and a broader version matrix add to that budget. Measure actual
workflow duration during the pilot rather than assuming these lower-bound costs.
