# Research

This directory contains experimental code, implementation plans, source
investigations, comparative benchmarks, and retained measurements. The repository
root contains the production library, correctness tests, and public API benchmarks.
Research does not change the default library build.

The cached-wall API was removed after commit `0393b48`. Darwin's historical
`darwin-commpage`, `darwin-wall-split`, and `wall-optimizations` profiles require
that revision; the runner reports this instead of attempting a broken build.
Use a separate checkout of `0393b48` to reproduce those results. Their sources
and raw measurements remain unchanged. The current calendar reader and its
validation commands are documented [here](darwin/calendar/README.md).

## Topics

| Directory | Contents | How to run code |
| --- | --- | --- |
| [darwin](darwin/README.md) | Apple clock source investigation and release history | Documentation and acquisition records |
| [darwin/calendar](darwin/calendar/README.md) | OS-owned calendar mapping, without local correction | Root Darwin tests; portable arithmetic tests |
| [darwin/commpage](darwin/commpage/README.md) | Commpage reader comparisons and adoption measurements | Overlay profile `darwin-commpage` |
| [darwin/wall-split](darwin/wall-split/README.md) | Split timestamp and rebasing experiment | Overlay profile `darwin-wall-split` |
| [linux-wall](linux-wall/README.md) | Bridge-cost probes, timestamp cache experiment, layout history | Overlay profile `linux-wall` |
| [linux-vdso-bridge](linux-vdso-bridge/README.md) | Cgo-free Go 1.27 vDSO bridges, linker probes, and integration evidence | Standalone module and validation scripts |
| [linux-wall/vvar](linux-wall/vvar/README.md) | Direct VVAR readers and exact-code recognition | Overlay profile `vvar` |
| [linux-wall/vvar/detectors](linux-wall/vvar/detectors/README.md) | BTF and instruction-analysis detector module | `go -C research/linux-wall/vvar/detectors test ./...` |
| [wall-optimizations](wall-optimizations/README.md) | Conversion and wall-read optimization comparisons | Overlay profile `wall-optimizations` |
| [windows](windows/README.md) | Standalone Windows clock comparisons and historical [plan](windows/plan.md) | `go -C research/windows test -short ./...` on Windows amd64 |
| [fastime](fastime/README.md) | Standalone comparison with the fastime cache | `go -C research/fastime test ./...` |
| [api-shape](api-shape/README.md) | Shared API refactor measurements and compiler evidence | Production API benchmarks at the root |
| [measurements.md](measurements.md) | Historical production clock measurements | Retained results, not a runnable suite |

Commands in research documents run from the repository root unless stated
otherwise. Each standalone module has its own Go version and dependency
requirements. Root `go test ./...` does not enter nested modules.

## Same-package prototypes

Sources that need private library internals live in
[`_prototypes/`](_prototypes/README.md). The leading underscore tells Go to ignore
that tree during ordinary package discovery. The [runner](prototype.py) supplies
a temporary Go build overlay that makes one profile's source files visible to the
root package. It selects the existing experiment build tag and removes the
overlay manifest when the command exits. No source files are copied into or
modified at the root, and no production internals need to become public.

```sh
# Native Darwin example; runs library tests and commpage prototype tests.
python3 research/prototype.py darwin-commpage test -race .
python3 research/prototype.py darwin-commpage vet .

# Cross-build a Linux research binary, then execute it on a Linux host.
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
  python3 research/prototype.py vvar test -c -o /tmp/coarsetime-vvar.test .
```

The CLI accepts a profile, a Go command (`test`, `vet`, `build`, or `list`), and
arguments for that command. Paths are relative to the repository root, even when
the runner is invoked elsewhere. `GOOS`, `GOARCH`, `CGO_ENABLED`, and `GOCACHE` are
inherited normally. The runner owns `-tags` and `-overlay`; do not pass those flags.
Python 3 and Go are required. The profile README describes runtime environment
variables and host requirements. Setting a prototype build tag directly at the
root no longer enables its code; always use the runner.

## Reading retained evidence

Source paths, command lines, hashes, and absolute paths inside raw output,
disassembly, and provenance files describe the original run. They are retained
unchanged rather than rewritten to appear freshly measured. Use current README
links and reproduction commands for the reorganized tree. Moving the sources
does not constitute new runtime validation or refresh old performance results.
