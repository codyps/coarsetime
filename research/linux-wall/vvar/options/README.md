# Further VVAR layout detection options

2026-09-05. Research only: the Go detector and production reader are unchanged.
This follow-up tests independent type metadata and emulated execution against
the six-distro binary corpus, rather than relying only on exact code hashes.

## Findings

### BTF supplies actual field offsets

The running OrbStack kernel exposes readable `/sys/kernel/btf/vmlinux`. Its BTF
contains vdso_time_data, vdso_clock and vdso_timestamp, including structure sizes
and member bit offsets. Anonymous unions can be followed to find basetime.
The observed layout derives directly as:

- vdso_time_data.clock_data at byte 0;
- vdso_clock.seq at byte 0 and basetime at byte 40;
- vdso_timestamp size 16, sec at 0, nsec at 8;
- CLOCK_REALTIME_COARSE = 5 => seconds at 40 + 5*16 = 120, nsec at 128.

All **five extracted distro vmlinux images** also retain BTF with the relevant
types. Arch's kernel image was not collected; its headers-package vDSO was used
for code analysis only. [Live metadata](live-btf.json) and
[distro metadata](distro-btf.json) show the raw member offsets and type IDs.

BTF is therefore a viable optional metadata source, much stronger than a uname
table. Its [documented format](https://docs.kernel.org/bpf/btf.html) explicitly
encodes structure members and offsets. It does not by itself establish the
userspace address of this object, the sequence protocol, or namespace behavior.
It may be unavailable, filtered by container mounts, or omit a type. A production
parser must also validate member types, widths, arrays and bounds, reject
ambiguous definitions, and establish that it describes the running kernel.
The small research parser is not that hardened production parser.

### Emulation discovers addresses across all six code shapes

The [emulator](emulate.py) loads each actual ELF into Unicorn's private memory,
sets the clock ID to 5, and provides synthetic VVAR values through memory hooks.
It does not dereference host VVAR or execute the binary natively. It follows
exported jump wrappers automatically as ordinary x86 instructions.

Two scenarios run per binary: a stable even sequence and a sequence changed at
the verification read, forcing one retry. It asserts successful return, the
expected output values, and the expected sequence-read count. Every scenario
completes within a 1,000-instruction limit. Results: [emulated.json](emulated.json).

| Sample | Sequence address relative to vDSO ELF | sec relative to sequence | nsec relative to sequence | BTF agrees |
| --- | ---: | ---: | ---: | --- |
| Ubuntu 5.15 | -0x3f80 | 112 | 120 | Yes |
| Ubuntu 6.8 | -0x3f80 | 112 | 120 | Yes |
| Debian 6.1 | -0x3f80 | 112 | 120 | Yes |
| Debian 6.12 | -0x3f80 | 120 | 128 | Yes |
| Fedora 6.17 | -0x6000 | 120 | 128 | Yes |
| Arch 7.2 | -0x6000 | 120 | 128 | Not collected |

The ordinary successful paths execute just **36–46 instructions**, including
entry, dispatch, output and return. None of the instructions traversed in either
scenario overlaps a CPU alternative patch site in any of the six binaries.
This is checked against each binary's .altinstructions table using the known
12-byte record format for 5.15/6.1 and 14-byte format for the newer specimens.
This format selection is specific to this corpus, not an automatic format detector.

That gives a practical way around high-resolution-clock patches changing a
whole-function fingerprint: identify and verify the complete **coarse path**,
including dispatch, instead of requiring unrelated high-resolution branches to
be byte-identical. The absence of overlap in these traces is not a guarantee
about every kernel or every branch, especially the namespace path.

Finite emulation is **discovery/testing, not proof of compatibility**. The hooks
expect a 32-bit sequence and two 64-bit data fields; they reject other accesses.
Other input values, namespace cases, extra branches or helper implementations
could behave differently. Do not enable an unfamiliar live reader solely because
these two test cases pass, and do not ship Unicorn as a library dependency.

## Comparison of approaches

| Approach | Useful evidence | Remaining limitation |
| --- | --- | --- |
| Exact audited full-code profiles | Identity to reviewed implementation | Compiler/CPU variants cause false negatives; wrappers must be followed |
| Bounded semantic analysis of coarse path | Data addresses, load widths, sequence ordering and returned values | Requires a carefully limited decoder and control-flow/data-flow validation |
| Live BTF + code-derived data address | Independent layout/type confirmation without version guesses | Optional metadata; does not certify synchronization protocol |
| Isolated emulation with synthetic values | Discovers layouts across compiler output; exercises retries | Finite traces cannot certify unseen behavior |
| Full symbols or DWARF from matching artifacts | Data symbol and/or type information | Only Arch's saved ELF has full symbols; stripped live images cannot assume them |
| CPU-alternative-aware fingerprinting | Avoids harmless high-resolution branch hash changes | Must validate boundaries/replacements; blindly masking bytes is unsound |
| Timestamp/sequence sampling | Detects obvious errors against current kernel output | Matching samples do not establish field identity or behavior under unseen changes |
| Kernel version/build ID | Narrows which audited profile to try | Neither is a self-describing layout contract |

## Recommended next implementation

Build a small, bounded **semantic recognizer for the clock-ID-5 path**, using
these emulation traces to create fixtures and identify the required instruction
subset. Resolve the versioned entry, follow direct wrappers, evaluate dispatch
with a known clock ID, and track register/address expressions. Require:

1. A coherent 32-bit sequence read, evenness test, two 64-bit timestamp loads,
   and a second read/comparison of the same sequence surrounding those loads.
2. Successful output to contain exactly those seconds/nanoseconds with no
   unaccounted scaling or correction.
3. Every relevant successor to be understood: retry edges, odd-sequence handling,
   namespace behavior and return. Unknown branches, helpers or indirect targets
   must reject recognition. Do not treat one successful emulated path as coverage.
4. Derived addresses/alignment inside the expected readable VVAR mapping, and
   rejection or existing fallback for time-namespace pages.
5. Optional BTF cross-check of widths, offsets and strides. Contradictory metadata
   rejects the fast path; missing metadata need not reject an otherwise fully
   recognized and audited code path.
6. A strict instruction/graph-size bound and vDSO fallback for anything unfamiliar.

An intermediate implementation can recognize a few audited coarse-path templates
with decoded addresses/register operands. It must still verify the entry's path
to the template and all relevant branches, rather than search for a byte snippet.
Keep the exact-code recognizer as an initial known implementation while building
coverage. No new automatic acceptance criteria were added in this investigation.

## Reproduce

Research dependencies are isolated outside the library:

```sh
python3 -m pip install --target /tmp/coarsetime-vvar-python unicorn==2.1.4 pyelftools==0.32
PYTHONPATH=/tmp/coarsetime-vvar-python python3 research/linux-wall/vvar/options/emulate.py
PYTHONDONTWRITEBYTECODE=1 PYTHONPATH=/tmp/coarsetime-vvar-python \
  python3 research/linux-wall/vvar/options/collect_btf.py /tmp/coarsetime-distro-vdso
```

For live BTF, copy `/sys/kernel/btf/vmlinux` from the actual running Linux
system, then run `python3 btf_probe.py PATH`. The live result here comes from the
same Docker/OrbStack host as the original VVAR prototype; container userland is
not evidence of a different running kernel. The corpus acquisition scripts supply
the five local vmlinux files. Results were checked against the independently
obtained member offsets and instruction traces; no kernel or production clock
state was modified.

Implemented follow-up: the [standalone Go detector prototypes](../detectors/README.md)
now provide separate BTF and bounded instruction-analysis mechanisms, a live
atomic reader for each, corpus/rejection tests and read benchmarks. The BTF
prototype retains the explicitly documented weaker address/protocol assumption;
the instruction prototype checks symbolic timestamp flow and sequence guards.
