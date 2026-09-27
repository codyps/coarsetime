# Linux ARM64 vDSO clocks

The production ARM64 reader calls the kernel's vDSO for both
`CLOCK_MONOTONIC_COARSE` and `CLOCK_REALTIME_COARSE`. Missing symbols or a vDSO
error retain the syscall fallback. `purego` retains standard-library clocks.

## Runtime integration

The shared ELF/pclntab resolver locates and verifies the ABI0 implementation of
`runtime.asmcgocall` in `runtime/asm_arm64.s`, including relocation in PIE
executables. A local ABI0 wrapper invokes that bridge indirectly. The runtime
switches to its system stack and restores the goroutine stack afterwards.

Unlike amd64, ARM64 also needs to publish `g` at the bottom of the signal stack
while executing vDSO code. Without cgo, `runtime.sigFetchG` recovers `g` from
that slot when a signal interrupts the vDSO. Before the stack switch, the ABI0
wrapper saves the previous slot value and publishes the original goroutine.
It restores the slot after returning from the runtime bridge. It skips publication if there is no signal goroutine
or execution is already on that goroutine. With cgo, signal handling uses TLS;
publishing and restoring the slot is harmless.

The wrapper also saves and publishes `m.vdsoPC` and `m.vdsoSP` with the original
Go caller PC/SP, restoring both on return. This lets CPU profiling unwind the
user stack when a signal interrupts the vDSO instead of reporting `_VDSO`.

The C ABI tail-call trampolines preserve the C ABI's callee-saved registers, including Go's
assembler scratch register R27, and uses an aligned runtime system stack.
It does not call the vDSO on a growable goroutine stack.

The `g.m` and `m.gsignal` offsets are decoded from the verified runtime bridge's
initial adjacent loads, register comparison, and conditional branch. The
`m.vdsoPC` and `m.vdsoSP` offsets come from the checked save/publish sequence in
`runtime.nanotime1`, including matching load/store operands. Unknown
instruction sequences fail initialization rather than guessing a layout.
`g.stack.lo` remains the first word of `g`, also a runtime/cgo layout dependency.
This is still a private runtime ABI dependency, not an exported Go guarantee.
Stripped binaries work because neither DWARF nor ELF symbols are needed.
Initialization requires `/proc/self/exe` and allocates temporary parsing data;
steady-state reads allocate nothing.

Relevant Go source: [ARM64 runtime bridge](https://go.dev/src/runtime/asm_arm64.s),
[native clock readers](https://go.dev/src/runtime/sys_linux_arm64.s), and
[signal recovery](https://go.dev/src/runtime/signal_unix.go).

## Native validation

GitHub Actions uses `ubuntu-24.04-arm` runners, executing native aarch64 code.
The matrix covers Go 1.23, 1.24, 1.25, 1.26, and stable. Checks include default
and purego tests/vet, race detection, cgo-disabled reads with a required vDSO,
stripped executables, stripped/trimpath PIE executables, and checkptr.

The shared Linux tests bracket readings with the corresponding kernel syscall
and force the package-local fallback without changing the runtime's vDSO
symbol. Profiling stress combines concurrent reads, recursive stack growth,
GC, and all-goroutine stack traces. The benchmark job repeats it 20 times.
A separate test inspects CPU profiles for caller attribution, rejecting lost
`runtime._VDSO` samples. CI verifies that this test fails on the pre-review
bridge, then passes five times on the corrected implementation.

The benchmark job also traces 100,000 reads of each public clock reader and
rejects any `clock_gettime` syscall. A traced baseline read loop must contain
syscalls as a control. Timing measurements run separately from tracing.

## Performance method

Both binaries are built before measurement with the same stable Go toolchain,
`CGO_ENABLED=0`. The syscall baseline is commit
`cfab1351632b48dc33f8830f3dd330b2ea7fed78`. Five fresh-process rounds alternate
baseline/candidate order, with 500 ms per benchmark. Standard-library clocks
and the explicit syscall benchmark provide controls. These measurements cover
steady-state reads, not initialization, and are specific to the hosted VM.

## Results: 2026-09-27

[GitHub Actions run](https://github.com/codyps/coarsetime/actions/runs/36338885399),
candidate `8469f56`, Go 1.27.1, Neoverse-N2 (4 vCPUs), Linux
`6.17.0-1022-azure`. Medians of five samples, in ns/op:

| Operation | Syscall baseline | vDSO candidate | Speedup |
| --- | ---: | ---: | ---: |
| `NowInstant` | 178.7 | 19.47 | 9.2x |
| `Since` | 182.3 | 22.04 | 8.3x |
| `Now` | 177.7 | 18.36 | 9.7x |
| `UnixNano` | 175.1 | 18.38 | 9.5x |
| `time.Since` (control) | 36.63 | 36.44 | — |
| `time.Now` (control) | 71.81 | 71.63 | — |
| Explicit coarse syscall (control) | 175.7 | 175.5 | — |

Every row reported zero allocations. The performance advantage is substantial
on this host, but the small differences between control samples are noise, not
evidence of a change to the standard library or syscall cost.

All five native ARM64 Go-version jobs passed, including race detection and
the required cgo-disabled vDSO checks. The 20 profiling/GC/stack-growth stress
runs passed. The [profile regression output](arm64-profile-regression.txt)
shows lost caller samples on the pre-review bridge; the corrected bridge passed
five consecutive attribution checks. These measurements include the added
caller-metadata handling. The [vDSO trace](arm64-vdso.trace) contains zero `clock_gettime`
syscalls across the read benchmarks; the [baseline trace](arm64-syscall.trace)
contains 101 (100 requested reads plus benchmark calibration).

[Raw test and benchmark output](arm64-results.txt) retains the host details,
test names, individual samples, and allocation counts. The workflow uploads
these files as `linux-arm64-results` on every run.
