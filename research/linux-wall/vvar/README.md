# Direct VVAR prototype

2026-09-05, Go 1.26.6, Intel i9-9880H, Docker Linux amd64,
`7.0.14-orbstack-00380-ga7e0a2dc9535`. The default library is unchanged.

## Results

Five shuffled rounds, 300 ms per benchmark, one Docker process per sample,
with no concurrent compilation or tests. Every process requires an active vDSO
and direct VVAR reader and brackets 100,000 sets of VVAR reads with vDSO reads
before benchmarking. Medians from [all raw samples](raw.txt):

| Implementation | Now ns/op | UnixNano ns/op |
| --- | ---: | ---: |
| Current vDSO implementation | 16.92 | 17.41 |
| Direct VVAR, Go atomics | **4.738** | **3.579** |
| Direct VVAR, Go assembly | 6.114 | 4.410 |

Every sample reports zero bytes and allocations per read. The Go implementation
is about 3.6x faster for Now and 4.9x for UnixNano. This run's vDSO baseline was
slower than the earlier 13–14 ns measurements; compare candidates within this
run. These are single-host microbenchmarks, not portable performance guarantees.

Go inlines the sequence-counter reader (cost 72) into the wall conversion
functions. The assembly reader crosses an additional Go assembly call boundary.
[Disassembly](disassembly.txt) shows that the Go fast path has no system-stack
bridge or vDSO call. Both paths keep ordinary time.Unix construction, without
accessing the private representation of time.Time.

## Mechanism and scope

Build tag `vvarprototype` and environment `COARSETIME_VVAR_LAYOUT=modern` are
required to activate the experiment. This is **explicit layout selection**, not
automatic kernel compatibility detection. The code finds the read-only `[vvar]`
mapping and interprets its first page using the observed upstream x86-64
6.13–7.0 layout: sequence at byte 0, coarse realtime seconds at 120, and
nanoseconds at 128. Only the kernel named above was executed.

Initialization brackets 100 direct snapshots with realtime syscalls as a sanity
check. Matching timestamps do not establish that an unknown kernel has this ABI.
The expected data layout comes from [upstream source history](../layout-history.md),
including [v7.0 datapage.h](https://github.com/torvalds/linux/blob/v7.0/include/vdso/datapage.h).
The pointer conversion is isolated in a small assembly helper. Bounds and
mapping permissions are checked before dereferencing, but this cannot make an
incorrect explicit layout selection safe on arbitrary kernels.

The Go reader loads an even sequence, seconds, nanoseconds, and sequence again
using atomics. It accepts a snapshot only when sequences match and nanoseconds
are below one billion. The assembly version uses amd64 load/load ordering for
the same protocol. Both make at most eight attempts before falling back to the
existing public clock APIs. No goroutine, periodic publication, or cached wall
correction is added: successful reads obtain kernel-maintained coarse realtime.

A missing pointer, unavailable mapping, or failed initialization check disables
the direct reader. Permanently odd time-namespace pages are rejected by the
bounded retry path; there is no attempt to locate their underlying host page.
This avoids an infinite spin but does not demonstrate namespace acceleration.
Older layouts and actual non-default time namespaces were not executed.

## Validation

- Default Linux tests pass with both vDSO and direct VVAR required.
- 100,000 sets of Go/assembly Now and UnixNano readings bracketed by vDSO.
- Complete time.Time representation checked, including location and absence of
  a monotonic component; GC runs during the real-clock test.
- Synthetic nil pointers, odd sequences, invalid nanoseconds, and valid records.
- Concurrent synthetic sequence-counter publication checks for torn snapshots.
- Forced nil/odd-page Now fallbacks bracketed by the existing clock.
- Race, checkptr, and vet pass on Linux Go 1.26.6. Race instrumentation cannot
  observe kernel writes or instrument the assembly loads; those checks alone
  do not prove the shared-memory protocol.
- The forced-fallback test also passes without the layout environment variable.

## Reproduce

From the repository root:

```sh
GOCACHE=/tmp/coarsetime-prototype-go-cache GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
  go test -tags vvarprototype -c -o /tmp/coarsetime-vvar.test .
python3 research/linux-wall/vvar/run.py
```

The [runner](run.py) requires Docker and its existing `python:3.12-slim` image;
containers have networking disabled. It saves raw output and medians. To run
correctness checks separately:

```sh
docker run --rm --network none \
  -e COARSETIME_REQUIRE_VDSO=1 -e COARSETIME_VVAR_LAYOUT=modern \
  -v /tmp/coarsetime-vvar.test:/test:ro python:3.12-slim /test -test.v
```

The Go-atomic variant is the candidate to pursue. Production adoption still
needs a defensible layout-selection policy and coverage of other supported
kernels and namespace behavior. It is not enabled by default in this change.

## Detection follow-up

`COARSETIME_VVAR_LAYOUT=detect` now enables a separate
[exact-code recognition experiment](detection/README.md). It validates one audited
mapped vDSO function and its data address before dereferencing VVAR. Unknown
code falls back to the existing APIs. This removes manual layout selection for
that recognized implementation, but does not provide broad kernel coverage.
