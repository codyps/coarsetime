# Cgo-free vDSO bridges on Go 1.27

Investigation on 2026-09-27, Linux amd64. Both prototypes call the kernel's
versioned `__vdso_clock_gettime` with `CLOCK_MONOTONIC_COARSE` (6) and
`CLOCK_REALTIME_COARSE` (5), without cgo or disabling the Go linker checks.
The prototypes are a standalone research module. Candidate A has now been
adopted in the production library for every supported Go version, with explicit
initialization failure if the runtime bridge cannot be verified. Production
does not import this research module. The measurements below retain the original
investigation results.

**Recommendation: resolve the runtime's existing `asmcgocall` assembly entry at
initialization and invoke it through a tiny indirect ABI0 trampoline.** This
worked on Go 1.23.12, 1.26.6, and 1.27.1, including stripped PIE builds. It avoids
maintaining copies of the runtime's goroutine/thread layout. A dedicated bridge
also works on the tested Go 1.27.1 layout, but did not show a meaningful speed
advantage on this host.

## Findings

| Approach | Outcome |
| --- | --- |
| `//go:linkname` to `runtime.asmcgocall` | Rejected by Go 1.27.1 |
| Assembly `JMP runtime·asmcgocall(SB)` | Rejected too |
| Assembly taking `$runtime·asmcgocall(SB)` | Rejected too |
| Those references with `-ldflags=-checklinkname=0` | All build; callers must supply the flag |
| Resolve the runtime entry dynamically, then indirect call | Works with default linker checks, `CGO_ENABLED=0`, and no private `g`/`m` offsets |
| Dedicated vDSO system-stack bridge | Works in the tested Go 1.27.1 configuration; needs private runtime offsets and profiling integration |
| Add a push `//go:linkname asmcgocall` annotation to Go | Would require a patched toolchain or acceptance upstream; not attempted or submitted |
| Replace the bridge with `runtime.cgocall` | Not a drop-in cgo-free replacement: Linux checks `iscgo`, and the function adds scheduler/cgo transitions |

[Linker probes](linker-probes.sh) and [their output](linker-go1.27.1.txt) establish
the first four rows. Go's [assembly-symbol linker change](https://github.com/golang/go/commit/aee6009ba5e1d71948b03ac0458fbc99e3a14ace)
checks references originating in assembly as well as linkname references.
`runtime.systemstack` is also unannotated and is not a stable replacement API.
The `cgocall` constraints are visible in
[Go 1.27.1 cgocall.go](https://github.com/golang/go/blob/go1.27.1/src/runtime/cgocall.go).

## Candidate A: resolve and reuse asmcgocall

[Go resolver](bridge_linux_amd64.go), [indirect assembly](indirect_linux_amd64.s).

At initialization:

1. Open `/proc/self/exe` with `debug/elf`.
2. Read `.gopclntab` (or `.data.rel.ro.gopclntab` for PIE) with `debug/gosym`.
   This is Go's runtime PC/line table, which remains with `-ldflags='-s -w'`.
   The resolver does not depend on ELF `.symtab` or DWARF.
3. Find `runtime.asmcgocall` whose source is `runtime/asm_amd64.s`. The assembly
   entry and its generated ABIInternal wrapper have the SAME name in this table;
   accepting the first name match without distinguishing them is incorrect.
4. Use the linked and live addresses of `runtime.Gosched` to determine the load
   bias for PIE/ASLR. Require `runtime.FuncForPC` to confirm the selected entry,
   name, and source file in the running process. Reject missing/ambiguous entries.
5. Store the resulting ABI0 entry address once.

Each call places the same two pointer arguments and int32 return slot expected
by `asmcgocall` in the ABI0 stack frame. A `NOSPLIT|NOFRAME` assembly function
loads the cached address and tail-jumps to it. The real runtime implementation
switches stacks, calls the existing C-ABI vDSO trampoline, and restores state.
There is no static reference to the blocked symbol in the caller's object file.

The prototype uses the production ELF/version-checked vDSO resolver, copied into
[resolver_linux_amd64.go](resolver_linux_amd64.go), so it does not introduce
kernel VVAR layout assumptions. Its argument block has no Go pointers in it;
`//go:noescape` keeps it on the caller's stack. The only foreign target is the
vDSO; callbacks into Go are outside the contract.

This still depends on an internal runtime function's ABI, name, retention, and
source metadata. It is not a Go-supported API. `-trimpath` and ordinary stripping
were tested; garbling, custom packers, removed ELF section headers, plugins,
shared-library builds, and unavailable `/proc/self/exe` were not. The existing
vDSO resolver already requires procfs access. These conditions must cause an
explicit resolution failure, not an unchecked indirect jump. For a deployment
requiring the vDSO, production integration should fail clearly if resolution
fails rather than silently select a syscall because the Go version is new.

The runtime owns the actual stack-switching implementation, including changes
such as the `runtimesecret` experiment. An experiment-enabled build passed this
suite, but we did not test calls made inside an active secret region. Profiling
attribution retains the original bridge's limitation: samples inside the vDSO
can land in the runtime's generic vDSO bucket.

A separate project has also explored runtime-PC-table resolution of this symbol:
[graphics.gd pclntab](https://pkg.go.dev/graphics.gd@v0.0.0-20260915100915-c0f8609c5418/internal/pclntab).
This prototype does not copy that implementation or scan unbounded live memory.

## Candidate B: a dedicated vDSO bridge

[Dedicated assembly](dedicated_linux_amd64.s) follows the
[Go 1.27.1 nanotime1 call sequence](https://github.com/golang/go/blob/go1.27.1/src/runtime/sys_linux_amd64.s):

- Get the current goroutine and its `m` from TLS.
- Save and publish `m.vdsoPC` and `m.vdsoSP` so signal-time profiling can recover
  the Go caller while SP points into the system stack.
- Switch to the saved g0 stack when called on the current user goroutine.
- Align SP for the C ABI, reserve the timespec on that stack, and call the
  already-resolved vDSO pointer with clock ID 5 or 6.
- Retrieve seconds, nanoseconds and status, restore SP and profiling state,
  and return through ABI0.

No private runtime symbol is referenced. However, the prototype embeds offsets
of `g.m`, `g.sched.sp`, `m.g0`, `m.curg`, `m.vdsoSP`, and `m.vdsoPC`.
[The layout test](layout_linux_amd64_test.go) checks the actual assembly constants
against the executable's runtime DWARF. The validation script performs this
check before invoking the bridge in an unstripped default build.

It is deliberately enabled only for Go 1.27 on linux/amd64 without
`goexperiment.runtimesecret`. Only Go 1.27.1's layout was exercised. Supporting
another Go release/configuration requires a new layout audit, not assuming the
constants still hold. Supporting secret regions also requires the runtime's
register-erasure behavior. The assembly is derived from Go and carries its BSD
license in [GO-LICENSE](GO-LICENSE).

Calling the vDSO directly on an ordinary goroutine stack is not an adequate
replacement: the kernel does not provide a fixed small stack-usage bound. Go's
own code documents hardened kernel builds consuming a full page. Simply
reserving an arbitrary extra frame also leaves signal and profiling integration
unresolved.

## Measurements

Go 1.27.1, `CGO_ENABLED=0`, AMD Ryzen 9 7940HS, Linux
`6.18.33.2-microsoft-standard-WSL2`. Five 300 ms samples per case in a single
process; no concurrent builds during this Go 1.27 benchmark. Medians from
[raw validation output](validation-go1.27.1.txt):

| Coarse realtime read | ns/op | allocs/op |
| --- | ---: | ---: |
| Resolved runtime bridge | 12.57 | 0 |
| Dedicated bridge | 12.59 | 0 |
| Direct kernel syscall (comparison only) | 523.6 | 0 |

The read benchmark includes calling through a Go function value and converting
the returned timespec to nanoseconds. It is not the entire public `Now()` API.
These are single-host measurements, not portable latency guarantees. Older-Go
benchmark samples in the validation logs ran while other validation work was
active and should not be used for cross-version performance comparisons.

In the temporary full-library integration, medians were 14.00 ns for
`NowInstant`, 11.93 ns for `Now`, and 11.96 ns for `UnixNano`, all allocation-free.
The straightforward `debug/gosym` resolver costs about 1.7 ms and 2.74 MB of
temporary allocations per lookup in this test executable; production would run
it once at initialization. See [resolver measurements](resolution-bench-go1.27.1.txt).
Larger executables can increase this startup cost.

A separate `strace -f -e trace=clock_gettime` run observed **zero clock_gettime
syscalls** during 100,000 reads through each bridge. See
[output](clock-syscalls.txt) and [trace](clock-syscalls.trace). Timing under strace
is not used in the table. The invalid-clock error test deliberately permits the
vDSO's kernel fallback; the normal read-loop trace excludes that test.

## Validation and integration

### Direct asmcgocall comparison across Go versions

A subsequent matched comparison uses the original `//go:linkname` plus
`//go:noescape` binding on Go 1.23 and 1.26, versus the resolved ABI0 entry.
Both sides use the same argument record, vDSO trampoline, realtime coarse clock,
Go function-value call, and result conversion. Go 1.27 runs the resolved case
through that same benchmark harness; its default linker rejects the direct case.

Eight fresh-process rounds per Go version, 300 ms per case, `CGO_ENABLED=0`,
`GOMAXPROCS=1`. All binaries were compiled before measurement. Versions ran
sequentially, alternating forward/reverse order, and the direct/resolved case
order alternated each round. Same host as above. Medians:

| Go version | Original direct binding | Resolved bridge |
| --- | ---: | ---: |
| 1.23.12 | 12.96 ns | 12.84 ns |
| 1.26.6 | 12.97 ns | 12.50 ns |
| 1.27.1 | Blocked by default linker | 12.48 ns |

All cases report zero bytes/allocations per read. This is evidence of essentially
unchanged steady-state cost, not a reliable sub-nanosecond speedup claim; samples
vary by more than the differences between medians. Resolution's one-time startup
cost remains additional work, as described above.

[Go 1.26 disassembly](direct-comparison-disassembly-go1.26.6.txt) explains why
indirection need not add net cost. The original linkname call reaches the
ABIInternal wrapper, which builds an ABI0 frame and calls the assembly function.
The replacement directly prepares ABI0 arguments and tail-jumps to the assembly
entry, skipping that wrapper's extra frame/call/return.

[Raw measurements](direct-comparison.txt),
[benchmark](comparison_linux_amd64_test.go), and
[reproduction script](compare-direct.sh). Run from the Nix shell:

```sh
bash research/linux-vdso-bridge/compare-direct.sh
```

### Correctness and build coverage

Both prototypes passed Go 1.27.1 tests for:

- 100,000 syscall-bracketed readings per clock and correct `-EINVAL` propagation.
- Concurrent reads during repeated GC, 64+ KiB goroutine-stack growth, goroutine
  stack dumps, and CPU profiling (SIGPROF), checking monotonicity.
- A synthetic C-ABI callee that checks it is on g0, checks alignment, and touches
  16 KiB of stack. This is a stress fixture, not a claimed vDSO stack bound.
- Ordinary, stripped, PIE, stripped PIE with trimpath, and checkptr builds, all
  with `CGO_ENABLED=0` and default linker checks.
- `go vet` and race-instrumented execution (the race detector requires cgo).
- External linking, including stripped PIE, with cgo enabled for the linker.

Candidate A passed the same main validation matrix on Go
[1.23.12](validation-go1.23.12.txt) and [1.26.6](validation-go1.26.6.txt).
Candidate B is excluded on those toolchains. Additional Go 1.27.1 results:
[external linking](external-go1.27.1.txt),
[runtimesecret build](secret-go1.27.1.txt),
[final assembly-offset verification and vet](layout-final-go1.27.1.txt).
Passing stress/race tests is evidence, not proof about every signal interleaving;
assembly and kernel memory accesses are not race-instrumented.

[integration.sh](integration.sh) copies the current root package into a temporary
directory and tests the adopted bridge. During the initial investigation it
instead injected Candidate A into the previous implementation. The complete
production suite passed with the vDSO
required, along with vet, race, and stripped PIE checks. The API-surface test
still passes; no new exports are added. See
[integration results](integration-go1.27.1.txt) and
[public-API benchmark samples](integration-bench-go1.27.1.txt).

## Reproduce

### Production adoption validation

After adoption, the production package passed required-vDSO tests, vet, and
stripped PIE/trimpath tests on Go 1.23.12, 1.24.0, 1.25.0, 1.26.6, and 1.27.1.
Go 1.27.1 also passed checkptr, race, external-linking, and purego-tag checks.
Eight Linux/Darwin/Windows targets cross-compiled. The root tests now exercise
resolver metadata rejection and concurrent vDSO reads during profiling, GC,
goroutine stack dumps, and stack growth.

The adopted implementation's Go 1.27.1 medians (three 300 ms samples, one P) were
13.36 ns for `NowInstant`, 11.62 ns for `Now`, and 11.94 ns for `UnixNano`, with
zero allocations. Tracing 100,000 calls to each public reader observed zero
`clock_gettime` syscalls. These are measurements on the same single host above.
See [integration validation](adoption-go1.27.1.txt),
[benchmarks](adoption-bench-go1.27.1.txt), and
[syscall trace results](adoption-clock-syscalls.txt).

### Commands

From the repository root, in `nix develop` (Go, gcc and bash are required):

```sh
GOTOOLCHAIN=go1.27.1 bash research/linux-vdso-bridge/linker-probes.sh
GOTOOLCHAIN=go1.27.1 bash research/linux-vdso-bridge/validate.sh
GOTOOLCHAIN=go1.27.1 bash research/linux-vdso-bridge/integration.sh
GOTOOLCHAIN=go1.23.12 bash research/linux-vdso-bridge/validate.sh
GOTOOLCHAIN=go1.26.6 bash research/linux-vdso-bridge/validate.sh
```

The scripts print temporary binary paths. To trace only real vDSO reads:

```sh
strace -f -e trace=clock_gettime -o /tmp/clock.trace /path/to/default.test \
  -test.run='^$' -test.bench='BenchmarkRead/(indirect|dedicated)$' \
  -test.benchtime=100000x
```

Production CI retains stripped/PIE and required-vDSO checks across supported Go
versions. Unavailable runtime metadata causes an initialization panic, and the
runtime ABI dependency is documented in the root README. The measured dedicated bridge does not
justify taking on its greater runtime-layout maintenance burden as the first
choice.
