# BTF and instruction-analysis detector prototypes

This isolated Go module implements both detection mechanisms and connects each
to a direct atomic VVAR clock reader. It requires Go 1.26.6 and x/arch v0.20.0;
the main coarsetime module and its dependencies are unchanged. No new detector
is enabled in the default library or in the earlier exact-hash prototype.

## APIs

- `ParseImage(elfBytes)` resolves the defined x86-64 ELF clock_gettime symbol
  with version LINUX_2.6 and selects executable text.
- `DetectBTF(image, rawBTF)` derives field locations from actual BTF types.
- `DetectInstructions(image)` derives and checks the read protocol from code.
- `Live("btf")` / `Live("instructions")` reads the current mapped vDSO, invokes
  the selected detector, validates addresses against [vvar], and returns a Clock.
- `clock.Now()` reads an even sequence, both timestamp fields, and the sequence
  again with Go atomics. It retries at most eight times, then calls the existing
  coarsetime.Now. A nil Clock also uses that fallback.

All detection is initialization work. The read path is identical for both
mechanisms and has no timer or user-maintained timestamp cache. Layout contains
three signed offsets relative to the vDSO ELF base, so it supports both observed
112/120 and 120/128 timestamp offsets without compiling another reader.

## BTF mechanism

The bounded parser handles BTF type records, typedef/qualifier chains, arrays,
structures and anonymous unions. It locates vdso_clock or vdso_data, resolves
seq/basetime/sec/nsec, checks field widths, byte alignment, array capacity and
record bounds, and computes the CLOCK_REALTIME_COARSE element's offsets.

**BTF does not supply the userspace pointer.** This prototype interprets only the
fixed-clock-ID entry/dispatch prefix to locate its first 32-bit kernel-data read.
It uses that as the sequence-address witness, then applies BTF-relative member
offsets. This is a deliberately narrower code check than the instruction detector.
It assumes that this first read is the sequence of the identified generic clock
record. BTF alone does not prove that assumption or the synchronization protocol;
100 syscall/vDSO-bracket sanity checks supplement it but are not a proof.

A missing or rejected BTF blob makes Live return an error and a nil clock. It
never silently guesses offsets or substitutes the other detector. In normal
caller code, the nil clock's Now method remains usable through fallback.

## Instruction mechanism

A small abstract interpreter uses x86asm solely for decoding. Clock ID, stack
addresses and instruction-relative addresses are constants; sequence values and
timestamps remain **symbolic**, not representative concrete timestamps.

It follows direct wrappers and constant dispatch branches. On the ordinary even
sequence path it requires the low-bit guard before data loads, exactly two
64-bit timestamp loads, a second 32-bit load/comparison of the same sequence,
and successful return of precisely the loaded values. It analyzes an equal
sequence path and a forced-mismatch/retry path and requires the same field
addresses on both. Missing guards, wrong comparison width, altered output stores,
aliased fields, unsupported operations/predicates and out-of-text branches reject.
It bounds each analysis to 256 decoded instructions and bounds local stack access.

The direct reader falls back on odd sequence values; the analyzer does not verify
or accelerate namespace-specific odd-sequence branches. It does not implement a
general CFG verifier, all x86 flags/instructions, arbitrary helpers, or a formal
proof system. Arithmetic flags not modeled are marked unknown and cannot drive
accepted branches. The accepted instruction subset is intentionally small.

This is considerably stronger evidence than finite emulation with a few numeric
timestamps, but is still experimental code needing broader mutation/fuzz testing,
review of decoder/abstract-state assumptions and more live kernels before any
production promotion. The packaged corpus is not substituted for live validation.

## Validation

[Saved full test output](results/tests.txt):

- Instruction detector recognizes all six saved distro vDSOs: Ubuntu 5.15/6.8,
  Debian 6.1/6.12, Fedora 6.17, and Arch 7.2.
- BTF detector agrees with instruction-derived layouts for all five available
  extracted kernel images. No Arch vmlinux/BTF was collected.
- Both Live mechanisms activate on kernel 7.0.14-orbstack-00380-ga7e0a2dc9535.
  Each passes 100,000 direct-read brackets against the existing coarse clock,
  including time.Time representation checks.
- Six code mutations reject: missing even guard, wrong sequence operand, byte
  sequence comparison, aliased sec/nsec, return on changed sequence, and an
  output overwrite. Truncated text rejects too.
- Synthetic BTF tests accept a correct layout and reject incorrect sequence
  width, unaligned sequence, insufficient array count, oversized stride, bad
  type references and aliased timestamp fields. Invalid BTF identity is also
  rejected for each real kernel image.
- Nil/odd-sequence clocks exercise fallback. Linux race, checkptr and vet pass.
  Instrumentation cannot observe kernel writes; it is not proof of the kernel
  shared-memory protocol. There has been no live namespace or second-kernel run.

## Reproduce

The main module's `go test ./...` does not enter this nested research module.
Build a standalone Linux test executable:

```sh
GOCACHE=/tmp/coarsetime-prototype-go-cache GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
  go -C research/linux-wall/vvar/detectors test -c -o /tmp/coarsetime-vvar-mechanisms.test
```

Run in Linux, with the corpus and extracted kernel cache available:

```sh
docker run --rm --network none \
  -e VVAR_REQUIRE_LIVE=1 -e VVAR_KERNEL_CACHE=/cache \
  -v /tmp/coarsetime-distro-vdso:/cache:ro \
  -v /tmp/coarsetime-vvar-mechanisms.test:/test:ro \
  -v "$PWD/research/linux-wall/vvar:/vvar:ro" -w /vvar/detectors \
  python:3.12-slim /test -test.v
```

`VVAR_REQUIRE_LIVE=1` makes unavailable detection a test failure. Without it,
live tests skip on unsupported environments. BTF corpus tests skip unless
`VVAR_KERNEL_CACHE` is supplied. The acquisition scripts in ../corpus create that
cache. For benchmark reproduction use `-test.run '^TestLive$' -test.bench 'Now$'
-test.benchmem -test.benchtime=300ms -test.count=5` without concurrent builds/tests.

## Read benchmark

Same Intel i9-9880H / OrbStack Linux host, Go 1.26.6. Five 300 ms samples per
benchmark, run sequentially after validation with no concurrent builds/tests:

| Reader | Median ns/op |
| --- | ---: |
| Current coarsetime.Now | 15.530 |
| BTF-derived direct Clock.Now | 4.113 |
| Instruction-derived direct Clock.Now | 4.125 |

All samples report zero bytes and allocations per read. Detection and its
allocations happen before the timed section; these measurements do not measure
startup overhead. The BTF and instruction clocks share the same reader, so their
small timing difference is noise, not a mechanism advantage. BTF sample variation
is retained in the [raw benchmark output](results/bench.txt) and
[summary](results/summary.txt). Both methods are about 3.8x faster than the current
public API on this host; this is not a cross-kernel performance guarantee.
