#!/usr/bin/env python3
"""Build same-package research sources through a temporary Go overlay."""
import argparse
import json
import subprocess
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
PROTOTYPES = {
    "darwin-commpage": "commpageprototype",
    "darwin-wall-split": "wallsplitprototype",
    "wall-optimizations": "walloptprototype",
    "linux-wall": "linuxwallprobe",
    "vvar": "vvarprototype",
}

TARGETS = {
    "darwin-commpage": {"darwin/amd64", "darwin/arm64"},
    "darwin-wall-split": {"darwin/amd64"},
    "wall-optimizations": {"darwin/amd64", "linux/amd64"},
    "linux-wall": {"linux/amd64"},
    "vvar": {"linux/amd64"},
}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("prototype", choices=PROTOTYPES)
    parser.add_argument("command", choices=["test", "vet", "build", "list"])
    parser.add_argument("go_args", nargs=argparse.REMAINDER,
                        help="Go arguments; paths are relative to the repository root")
    args = parser.parse_args()
    if any(arg.split("=", 1)[0] in ("-tags", "-overlay") for arg in args.go_args):
        parser.error("the runner supplies -tags and -overlay")
    target = "/".join(subprocess.check_output(
        ["go", "env", "GOOS", "GOARCH"], cwd=ROOT, text=True).split())
    if target not in TARGETS[args.prototype]:
        parser.error(f"{args.prototype} requires one of {sorted(TARGETS[args.prototype])}; got {target}")
    source = ROOT / "research" / "_prototypes" / args.prototype
    replacements = {}
    for path in sorted(source.iterdir()):
        if path.suffix not in (".go", ".s"):
            continue
        target = ROOT / path.name
        if target.exists():
            parser.error(f"overlay would hide existing file: {target}")
        replacements[str(target)] = str(path)
    if not replacements:
        parser.error(f"no prototype sources in {source}")
    with tempfile.TemporaryDirectory(prefix="coarsetime-overlay-") as temp:
        overlay = Path(temp) / "overlay.json"
        overlay.write_text(json.dumps({"Replace": replacements}))
        command = ["go", args.command, "-overlay=" + str(overlay),
                   "-tags=" + PROTOTYPES[args.prototype], *(args.go_args or ["."])]
        result = subprocess.run(command, cwd=ROOT)
    return result.returncode


if __name__ == "__main__":
    raise SystemExit(main())
