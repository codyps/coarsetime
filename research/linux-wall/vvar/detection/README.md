# Detecting compatibility with direct VVAR reads

The running **vDSO code** is the useful authority. Neither uname nor a timestamp
match establishes the private data layout. This investigation adds a narrow,
fail-closed detection mode to the existing build-tagged prototype. Production
code is unchanged.

## Live evidence

On the tested 7.0.14 OrbStack x86-64 kernel, the mapped ELF has .dynsym, GNU
symbol versions, a Linux version note and a GNU build ID, but no data layout
symbol, full .symtab, DWARF or BTF. Its `__vdso_clock_gettime@LINUX_2.6` body is
1,208 bytes at ELF virtual address 0xaf0. The version describes the function
interface, not VVAR. See [ELF metadata](elf-metadata.txt).

The [actual mapped function](clock-gettime-disassembly.txt) proves this normal
coarse-clock path for clock ID 5:

1. Dispatch at 0xb01–0xb18 and 0xcb2 selects the coarse path for ID 5.
2. At 0xcbc a RIP-relative LEA computes **vDSO base - 0x6000**.
3. The timestamp address is that base + **clock ID * 16 + 0x28**: seconds at
   **120**, nanoseconds at **128**.
4. At 0xccb it reads a 32-bit sequence from that same base and tests its low bit.
5. At 0xce8/0xcee it copies two 64-bit values, then reloads and compares the
   original sequence at 0xcf6–0xd02, retrying on change.
6. Odd sequence plus mode 0x7fffffff takes the namespace path; our bounded reader
   falls back for odd pages rather than implementing namespace translation.
7. The ordinary success path returns these values without correction or scaling.

These addresses come from instructions executed by the kernel-provided public
reader, not from scanning VVAR for plausible timestamps. The RIP-relative data
address is also checked against the readable [vvar] mapping before dereference.

## Working detection prototype

Set `COARSETIME_VVAR_LAYOUT=detect` with the `vvarprototype` build tag. At init:

- Read the mapped vDSO using /proc/self/mem and validate ELF64, little endian,
  EM_X86_64, ET_DYN, executable load bounds and mapped-image addressing.
- Resolve the defined function symbol with GNU version LINUX_2.6 using the
  existing version parser.
- Match its start, size and **SHA-256 of the complete function** to an audited
  profile. This profile is
  `278894e4280a170ff9bad9c3352bed501d5eeae8ae73eac5ce4a1f2ba3557fa7`.
- Use the profile's audited data displacement; require the result to equal the
  beginning of the readable [vvar] mapping and have sufficient mapped space.
- Only then dereference and run the existing syscall-bracket sanity checks.
- Return nil and use the existing vDSO path on any mismatch.

This proves identity to one reviewed machine-code implementation, assuming the
normal contract that the kernel's own vDSO implements its public clock API
correctly. It is not a universal recognizer, kernel attestation, or proof against
a malicious kernel. The build ID is recorded for provenance, not used as proof.

The read path is unchanged: detection occurs once and publishes the same pointer.
The explicit `modern` mode is retained solely as the earlier manual experiment;
`detect` does not fall back to guessing that layout.

## Validation and limitations

The detector recognizes the live mapped code. All existing real-clock, concurrent
sequence, and fallback tests pass in detect mode. Tests reject malformed ELF,
changed ELF identities, and **all 1,208 individual byte mutations** of the
recognized function. Linux Go 1.26.6 race and vet checks also pass. See
[test output](test.txt). These tests verify profile matching and existing reader
behavior; they do not establish portability to other kernel builds.

Full function fingerprints will often miss compatible kernels built by another
compiler or patched for different CPU features. Those are safe false negatives,
with no fast-path coverage. An exact hash does not survive arbitrary changes and
is intentionally stricter than matching a build ID or a short instruction string.

## How to broaden coverage

First collect and audit additional mapped vDSO images from supported kernels and
CPU configurations. Add profiles only where the entire selected coarse path and
its data addressing/protocol have been reviewed.

If profile maintenance becomes excessive, a bounded instruction recognizer can
accept a small set of known code-generation templates. It must follow the actual
entry's control flow for clock ID 5, track address expressions, recognize the
32-bit even/equal sequence checks surrounding both 64-bit loads, and establish
that the returned values are unmodified. Validate all branch targets and reject
unknown instructions, indirect targets, helpers, or protocols. Resolve explicit
RIP-relative references; never merely search for a LEA or mask displacement bytes
without checking their targets. This broader recognizer is a proposal, not code
implemented by this change.

Timestamp bracketing remains a secondary sanity check. It cannot certify the
sequence offset, namespace semantics, or an unseen rollover/update case: the
coarse timestamp can remain constant through many comparisons. Kernel-version
and build-ID tables can help select profiles but are not independently sufficient.
A portable kernel-exported layout descriptor would be preferable; the inspected
x86-64 vDSO exposes no such contract. The documented stable interface remains the
[versioned function](https://man7.org/linux/man-pages/man7/vdso.7.html).

## Reproduce

```sh
GOCACHE=/tmp/coarsetime-prototype-go-cache GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
  go test -tags vvarprototype -c -o /tmp/coarsetime-vvar-detect.test .
docker run --rm --network none \
  -e COARSETIME_VVAR_LAYOUT=detect -e COARSETIME_REQUIRE_VDSO=1 \
  -v /tmp/coarsetime-vvar-detect.test:/test:ro python:3.12-slim /test -test.v
```

The test invocation explicitly requires a recognized direct reader and therefore
fails on an unrecognized kernel; ordinary API calls simply use the vDSO fallback.

Follow-up: [BTF and emulation experiments](../options/README.md) recovered the
required offsets independently across the distro corpus. They support pursuing
a bounded coarse-path recognizer with optional BTF cross-checks; neither finite
emulation nor BTF alone was promoted to an automatic acceptance rule.
