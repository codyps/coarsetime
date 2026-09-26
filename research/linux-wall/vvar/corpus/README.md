# Distro vDSO machine-code corpus

Collected 2026-09-05 from distro package archives, x86-64. Six actual binary
vDSOs were recovered, without rebuilding kernels or booting guest systems.

| Sample | Packaged kernel | Acquisition | clock_gettime exported symbol |
| --- | --- | --- | --- |
| [Ubuntu](ubuntu-5.15/) | 5.15.0-25-generic | unsigned image, embedded ELF | 5-byte jump to 0x7b0 |
| [Ubuntu](ubuntu-6.8/) | 6.8.0-31-generic | unsigned image, embedded ELF | 5-byte jump to 0xab0 |
| [Debian](debian-6.1/) | 6.1.0-47-amd64 / 6.1.170-3 | unsigned image, embedded ELF | 608 bytes at 0x960 |
| [Debian backports](debian-6.12/) | 6.12.43+deb12-amd64 | unsigned image, embedded ELF | 961 bytes at 0xbd0 |
| [Fedora 43](fedora-43/) | 6.17.1-300.fc43.x86_64 | kernel-core RPM, embedded ELF | ENDBR64 + jump to 0x8a0, 9 bytes |
| [Arch](arch/) | 7.2.3-arch1-2 | linux-headers, standalone vdso64.so | 1,345 bytes at 0xd00 |

This is a deliberately mixed historical sample, not a claim that these are the
latest kernels or a representative survey of every supported distro release.

Each directory contains:

- `vdso.so`: actual ELF bytes, including the hidden implementation reached by
  wrappers, alternative instruction tables, and other vDSO functions.
- `clock-gettime.bin`: exactly the exported function's symbol-sized bytes.
  **For wrappers this is not the complete clock implementation.**
- `disassembly.txt`: full disassembly with instruction bytes and ELF addresses.
- `elf.txt`: section/symbol/version metadata and GNU build ID.
- `provenance.json`: exact package URL, download size and SHA-256; kernel-embedded
  copies also record the ELF offset in decompressed vmlinux and extracted size.
- `analysis.json`: ELF, exported-symbol, and entire .text SHA-256, entry location,
  wrapper target, and whether full symbols/alternative instruction data exist.

[summary.json](summary.json) collects all code hashes. The whole corpus is under
one megabyte. Large downloaded packages, uncompressed kernel images and package
payloads remain outside the repository at `/tmp/coarsetime-distro-vdso`.

## Acquisition

Official distribution package listings supplied the artifacts:

- [Ubuntu kernel pool](https://archive.ubuntu.com/ubuntu/pool/main/l/linux/)
- [Debian kernel pool](https://deb.debian.org/debian/pool/main/l/linux/)
- [Fedora 43 kernel packages](https://dl.fedoraproject.org/pub/fedora/linux/releases/43/Everything/x86_64/os/Packages/k/)
- [Arch linux-headers file list](https://archlinux.org/packages/core/x86_64/linux-headers/files/)

Debian/Ubuntu packages were opened as ar archives and their data tar members
inspected. Fedora's RPM payload was inspected similarly. Kernel boot images
were decompressed by locating a supported compression stream, requiring ELF
output, then locating embedded ELF64 little-endian ET_DYN/EM_X86_64 images with
`__vdso_clock_gettime` in their string data. Exactly one such image was found in
each kernel. The saved extent includes the ELF headers and file-backed sections;
page-alignment padding outside that extent is not copied.

Arch ships `usr/lib/modules/7.2.3-arch1-2/vdso/vdso64.so` in its headers package,
so no kernel-image extraction was needed. It retains the full symbol table,
unlike the five embedded copies. Packages were fetched over HTTPS; recorded
hashes identify the downloaded artifacts, not an additional signature audit.

The binaries are Linux kernel artifacts; their upstream source and GPL licensing
remain applicable. The collection/analysis scripts are ours. No downloaded
kernel code was executed.

## Implications for our detector

1. **Do not hash only the exported symbol and assume that covers its logic.**
   The Ubuntu/Fedora symbols are wrappers. Follow their direct jumps and validate
   the reached implementation, or match a larger audited executable region.
   Our current single profile remains conservative and rejects all six files.
2. **Whole function hashes vary across builds.** None of these exported-symbol
   hashes matches the existing OrbStack profile. Full text hashes differ too.
3. **Equivalent address calculations look different.** Several use
   `(clock_id + 2) << 4`, with a further load displacement of zero or eight bytes,
   rather than our original specimen's explicit `clock_id*16 + 0x28`.
4. **These are pre-runtime-patching artifacts.** Every sample has alternative
   instruction metadata. The kernel's [vDSO initialization](https://github.com/torvalds/linux/blob/v6.12/arch/x86/entry/vdso/vma.c#L55)
   calls `apply_alternatives`, so mapped code may differ with CPU capabilities.
   The coarse branch may be unaffected while a whole-function hash changes in
   a high-resolution clock branch. Do not simply ignore arbitrary bytes: any
   normalization must validate the alternatives and all reachable coarse logic.
5. This is corpus acquisition and static inspection, **not six live validated
   detector profiles**. We did not add any of these hashes to the allowlist or
   assert that the prototype can safely read their VVAR mappings.

The corpus supports building and testing a bounded recognizer across realistic
compiler output. Runtime profile adoption still needs a review of the complete
clock-ID-5 path, memory protocol, mapping references and CPU-patched variants.
Distro containers would not supply those runtime variants: they share the host
kernel, so obtaining live distro vDSOs requires booted kernels or external dumps.

## Reproduce

Run the saved acquisition stages (curl, bsdtar, Python 3, zstd/xz/gzip/lz4):

```sh
python3 research/linux-wall/vvar/corpus/tools/download.py
python3 research/linux-wall/vvar/corpus/tools/unpack.py
python3 research/linux-wall/vvar/corpus/tools/extract.py
```

These write into `/tmp/coarsetime-distro-vdso`; compare package and recovered ELF
hashes to [downloads.json](downloads.json) and each provenance file. The Arch
URL is pinned to the actual package instead of the moving download endpoint.
Old artifacts may eventually leave the mirrors.

To regenerate symbol bytes and analysis from the saved corpus:

```sh
python3 research/linux-wall/vvar/corpus/analyze.py
```

Disassembly/metadata were generated using GNU objdump/readelf in the existing
`golang:1.26.6` container, with networking disabled. The raw fixtures can be read
by Go's debug/elf without running them; no extra library dependency is required.
