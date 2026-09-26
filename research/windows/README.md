# Windows clock experiment

Standalone, Windows/amd64-only benchmark module created while the parent was an
unfinished Darwin sketch. It compares clock sources independently of the public
API, which now has its own benchmarks at the repository root.
See [the implementation plan](plan.md).

Run from this directory in PowerShell:

```powershell
go test -v
go vet ./...
go test -run '^$' -bench . -benchmem -benchtime=500ms -count=5 -cpu=1
```

`benchmark-results.txt` contains the retained benchmark run;
`validation-results.txt` contains correctness checks and resolution observations.
`TestResolution` takes about two seconds; `go test -short` skips it.

Methodology:

- Sequential benchmarks, five repetitions of each case, 500 ms target duration,
  GOMAXPROCS=1. No CPU affinity or power-policy changes. Background system work,
  boost, scheduling, and benchmark order can affect these small timings.
- Every benchmark consumes its result through a global sink. No empty-loop cost
  is subtracted. The costs include the Go call path and result store.
- Windows procedures are resolved before timing. Benchmarks use cached addresses
  and `syscall.SyscallN`, avoiding `LazyProc.Call` lookup checks and error-interface
  boxing. Output storage is allocated before timing and reused. These are costs
  from Go, not isolated native Windows function-body timings.
- `TimeNow` and `SharedTime` both return a `time.Time`, but only `TimeNow` carries
  a Go monotonic reading. `TimeNowUnixNano` and `SharedUnixNano` are the closer
  comparison when consumers only need a wall-clock integer.
- Raw FILETIME and interrupt values are 100 ns ticks; QPC values are counter
  ticks, with separately queried frequency; GetTickCount64 returns milliseconds.
  Raw benchmarks intentionally exclude normalization and do less work than Now.
- Resolution sampling records positive increments during 250 ms per clock.
  Sampling itself has overhead; minimum and median increments are observations,
  not guarantees of accuracy or maximum staleness. Diagnostics use LazyProc.Call
  outside the performance benchmark. No system timer-resolution request is made.
- The interrupt procedures were not exported by kernel32.dll on this machine;
  this experiment resolves them from kernelbase.dll. Production bindings should
  use a reviewed API-set/export strategy for the supported Windows versions.

Shared-page access is experimental assembly, limited to amd64. Do not use its
wall clock for elapsed timing. The FILETIME conversion uses signed Unix
nanoseconds and therefore has the usual limited range around the Unix epoch.
Tests check normal current-clock consistency, not clock changes or suspend/resume.
