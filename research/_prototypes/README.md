# Same-package prototype sources

These files intentionally share the production `coarsetime` package so they can
compare private implementation details without exporting them. They are not
standalone Go packages. The underscore directory is excluded from normal Go
package discovery. Build them using [`../prototype.py`](../prototype.py), as
explained in the [research index](../README.md#same-package-prototypes).

Darwin profiles that use the removed cached-wall API must be run from revision
`0393b48`; the current runner reports that requirement. They are retained as
historical experiments, not adapted to imply equivalent current wall semantics.

| Source directory / profile | Supported target | Notes and measurements |
| --- | --- | --- |
| [darwin-commpage](darwin-commpage) | Darwin amd64 or arm64 | [Commpage](../darwin/commpage/README.md) |
| [darwin-wall-split](darwin-wall-split) | Darwin amd64 | [Split wall time](../darwin/wall-split/README.md) |
| [wall-optimizations](wall-optimizations) | Darwin amd64 or Linux amd64 | [Wall optimizations](../wall-optimizations/README.md) |
| [linux-wall](linux-wall) | Linux amd64 | [Bridge probes](../linux-wall/README.md) |
| [vvar](vvar) | Linux amd64 | [Direct VVAR](../linux-wall/vvar/README.md) |

Existing source basenames and build tags are retained so historical disassembly
and recorded hashes can still be traced to the corresponding source. The runner
injects only the selected profile, alongside current production code and tests.
