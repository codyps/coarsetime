# Split wall-clock base experiment

Prototype sources live under `research/_prototypes/` and run through the
[research overlay runner](../../README.md#same-package-prototypes).


This experiment tests whether Darwin amd64 `Now()` can avoid normalizing a
complete Unix-nanosecond timestamp on every read. It uses the adopted atomic
commpage tick reader. It does not read the calendar commpage record or introduce
OS clock resynchronization. **The default library is unchanged.**

Enable the experiment with `-tags wallsplitprototype`. Sources:

* [Mapping and three readers](../../_prototypes/darwin-wall-split/wall_split_prototype_darwin_amd64.go)
* [Tests and benchmarks](../../_prototypes/darwin-wall-split/wall_split_prototype_darwin_amd64_test.go)

## Representation

An immutable snapshot contains the existing wall correction, calendar seconds,
and the Mach tick corresponding to the beginning of that calendar second.
A read subtracts that tick from the current approximate tick. If the result is
within the second, it supplies already-normalized seconds/nanoseconds to
`time.Unix`. The time has the same Local location and no monotonic component.
The snapshot includes a cached `time.Time` for the Add comparison variant.
A clipped fast window preserves the current UnixNano behavior at its upper
int64 boundary.

Snapshot publication uses `atomic.Pointer`. This prevents readers from combining
fields from different refreshes, and lets old readers safely finish while GC
retains the old snapshot. It avoids unsafe access to time.Time internals.

## Variants

* **Fixed:** use the split base within its second; subsequently fall back to the
  same full timestamp normalization as the current implementation. No automatic
  rebasing, so the benefit expires without another explicit prototype refresh.
* **Rebase:** same fast path. On expiration, allocate a new immutable snapshot
  for the current second, retaining the SAME correction. Publish it with CAS;
  if a concurrent correction refresh wins, do not overwrite its mapping.
* **Add:** use the cached start-of-second time.Time and its public `Add` method
  within the second; use full normalization after expiry.

`refreshSplitPrototype` snapshots the existing wallCorrection and current ticks.
Tests and benchmarks call it explicitly; production RefreshWallClock is untouched.
An eventual adoption must integrate split snapshot publication into calibration.
Rebasing alone does not fix clock steps, suspend error, or NTP drift.

The Rebase design normally allocates once per observed second in serial use.
After an idle gap it needs only one new snapshot. Concurrent readers can allocate
redundant snapshots at a boundary; only one CAS succeeds for a given old mapping.
There is no background timer or goroutine. A separate forced-boundary benchmark
makes these allocations visible rather than rounding them away in steady state.

## Validation and measurement

Native Darwin amd64 / Go 1.26.6; benchmark binary compiled with CGO_ENABLED=0.
The amd64-only experiment assumes the verified 1:1 Mach tick-to-nanosecond ratio.
No ARM64 prototype or performance claim is made.

Tests compare split arithmetic with the current full normalization for random
anchors, negative epochs, carries, unsigned tick wrap, and int64 limits. Native
readers are bracketed by current Now readings. Tests also exercise concurrent
snapshot publication/rebasing under GC and verify that a stale rebase cannot
overwrite a newer correction. Package tests, race, strict checkptr, and vet passed.

The [runner](run.py) runs five shuffled serial rounds of nine 200 ms cases, then
three 3-second runs of Rebase to exercise second transitions. All compilation
and validation finished before timing. No concurrent agent build/test jobs ran.

**Fresh** setup deliberately aligns a synthetic correction to a calendar-second
boundary. This isolates the full subsecond fast window from launch-time phase;
it is a throughput experiment, not a measurement of wall accuracy. **Aged**
starts with a base two seconds old: Fixed/Add remain on their fallback, whereas
Rebase updates once and becomes fresh. The long Rebase runs cross real second
boundaries. The forced-boundary case includes the atomic Store used to expire
the mapping on every iteration, so its timing includes that setup overhead.

These are hot-loop throughput results, not isolated memory-load latency. Read
allocations below one per operation round to zero in steady-state benchmark
output; that does NOT mean Rebase is allocation-free. The same approximate-clock
freshness limits and manual wall-correction refresh requirements still apply.

[Raw results](raw.txt), [summary](summary.json), [pilot](pilot.txt), and
[disassembly](disassembly.txt) are retained locally. The disassembly shows that
normalization instructions remain in the compiled function but their branch is
skipped for an in-range fractional second.

## Reproduce

From the repository root:

```sh
export GOCACHE=/tmp/coarsetime-prototype-go-cache
python3 research/prototype.py darwin-wall-split test -race .
python3 research/prototype.py darwin-wall-split test -gcflags=all=-d=checkptr=2 .
python3 research/prototype.py darwin-wall-split vet .
CGO_ENABLED=0 python3 research/prototype.py darwin-wall-split test -c -o /tmp/coarsetime-wall-split.test .
python3 research/darwin/wall-split/run.py /tmp/coarsetime-wall-split.test
```

## Results: 2026-09-05

Intel i9-9880H, Darwin amd64, Go 1.26.6. Medians in ns/op:

| Case | ns/op |
| --- | ---: |
| Current Now | 4.961 |
| Current UnixNano (control) | 2.221 |
| Fixed base, fresh | 3.079 |
| Fixed base, aged | 4.725 |
| Rebase, fresh | 3.049 |
| Rebase, initially aged | 3.087 |
| Rebase, 3-second steady runs | 3.137 |
| Time.Add, fresh | 7.929 |
| Time.Add, aged | 4.699 |
| Forced rebase, including expiry Store | 57.980 |

The rebasing variant reduces steady read time by 36.8% (1.58x throughput). The three long-run samples were 3.084, 3.137, 3.256 ns/op.

Forced rebase reports **64 B/op and 1 alloc/op**. Other cases report zero after
per-operation rounding. The fresh/aged fixed-base comparison shows that the
static split base alone gives only a temporary benefit; rebasing is necessary
to retain it without frequent explicit refreshes. Time.Add is slower than the
current path in the very window it was intended to optimize.

Recommendation: the split/rebase approach is a viable speed-versus-complexity
tradeoff, not a free substitution. Adoption would need atomic publication of
the new base during RefreshWallClock and decisions about retaining the current
allocation-free read contract. UnixNano already remains faster if a time.Time
value is unnecessary. No default code, calibration behavior, commit, or push
was changed by this experiment.
