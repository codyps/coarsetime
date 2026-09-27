"""Build once, validate, warm up, then alternate base/head on one native worker."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import subprocess
import time

from stats import NAMES, summary

HERE = Path(__file__).resolve().parent
LINE = re.compile(r"^BenchmarkCI(\w+)(?:-1)?\s+\d+\s+([\d.]+) ns/op\s+(\d+) B/op\s+(\d+) allocs/op$", re.M)


def command(args, cwd=None, env=None):
    try:
        return subprocess.check_output(args, cwd=cwd, env=env, text=True, stderr=subprocess.STDOUT)
    except subprocess.CalledProcessError as error:
        print(error.output, flush=True)
        raise


def parse(text, expected):
    matches = LINE.findall(text)
    if len(matches) != 1 or matches[0][0] != expected:
        raise ValueError(f"Missing or ambiguous benchmark {expected}: {text}")
    _, ns, size, allocs = matches[0]
    if float(ns) <= 0:
        raise ValueError("Nonpositive benchmark duration")
    return {"ns": float(ns), "bytes": int(size), "allocs": int(allocs)}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--head", type=Path, required=True)
    parser.add_argument("--base", type=Path)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    args.output = args.output.resolve()
    args.output.mkdir(parents=True, exist_ok=True)
    config = json.loads((HERE / "config.json").read_text())
    # Windows checkouts may use CRLF. Hash and inject one canonical byte stream
    # so all native matrix cells identify the same harness.
    harness = (HERE / "harness_test.go.txt").read_text(encoding="utf-8").encode("utf-8")
    env = dict(os.environ, CGO_ENABLED="0", GOTOOLCHAIN="local", GOMAXPROCS="1")
    goenv = json.loads(command(["go", "env", "-json", "GOOS", "GOARCH", "GOAMD64", "GOARM", "GOEXPERIMENT", "GOVERSION"], env=env))
    record = {"schema": 1, "run_id": int(os.environ.get("GITHUB_RUN_ID", "0")),
              "run_attempt": int(os.environ.get("GITHUB_RUN_ATTEMPT", "1")),
              "pr": int(os.environ.get("PR_NUMBER", "0")),
              "event": os.environ.get("GITHUB_EVENT_NAME", "local"),
              "runner": os.environ.get("BENCH_RUNNER", platform.system()),
              "go_selector": os.environ.get("GO_SELECTOR", "local"),
              "harness": hashlib.sha256(harness).hexdigest(), "config": config,
              "timestamp": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
              "environment": {"go": goenv, "os": platform.platform(), "cpu": platform.processor(),
                              "image": os.environ.get("ImageVersion", "unknown"),
                              "gomaxprocs": 1, "cgo": 0},
              "samples": {}, "commits": {}}
    if platform.system() == "Linux":
        details = command(["lscpu"], env=dict(env, LC_ALL="C"))
        model = re.search(r"^Model name:\s*(.+)$", details, re.M)
        record["environment"]["cpu"] = model.group(1).strip() if model else platform.processor()
        record["environment"]["cpu_details"] = details
        clock = Path("/sys/devices/system/clocksource/clocksource0/current_clocksource")
        record["environment"]["clocksource"] = clock.read_text().strip() if clock.exists() else "unknown"
    elif platform.system() == "Darwin":
        record["environment"]["cpu"] = command(["sysctl", "-n", "machdep.cpu.brand_string"])
    binaries = {}
    raw = args.output / "raw.txt"
    with raw.open("w", encoding="utf-8") as log:
        for label, checkout in (("base", args.base), ("head", args.head)):
            if checkout is None:
                continue
            checkout = checkout.resolve()
            record["commits"][label] = command(["git", "rev-parse", "HEAD"], cwd=checkout).strip()
            target = checkout / "zz_ci_benchmark_test.go"
            if target.exists():
                raise ValueError(f"Reserved harness path already exists: {target}")
            target.write_bytes(harness)
            try:
                binary = args.output / (label + (".exe" if os.name == "nt" else ".test"))
                command(["go", "test", "-c", "-o", str(binary), "."], cwd=checkout, env=env)
                validation_env = dict(env)
                if goenv["GOOS"] == "linux" and goenv["GOARCH"] in ("amd64", "arm64"):
                    validation_env["COARSETIME_REQUIRE_VDSO"] = "1"
                if goenv["GOOS"] == "darwin":
                    validation_env["COARSETIME_REQUIRE_DARWIN_WALL"] = "1"
                log.write(f"validation: {label}\n" + command([str(binary), "-test.short", "-test.v"], env=validation_env))
                command([str(binary), "-test.run=^$", "-test.bench=^BenchmarkCI", "-test.benchtime=50ms", "-test.cpu=1"], env=env)
                binaries[label] = binary
                record["samples"][label] = {name: [] for name in NAMES}
            finally:
                target.unlink()
        for round_id in range(config["samples"]):
            # Reverse both revision and case order to reduce temporal/order bias.
            names = NAMES if round_id % 2 == 0 else tuple(reversed(NAMES))
            labels = list(binaries) if round_id % 2 == 0 else list(reversed(binaries))
            print(f"Measured round {round_id + 1}/{config['samples']}", flush=True)
            for name in names:
                for label in labels:
                    output = command([str(binaries[label]), "-test.run=^$", f"-test.bench=^BenchmarkCI{name}$",
                                      f"-test.benchtime={config['benchtime']}", "-test.cpu=1", "-test.benchmem"], env=env)
                    log.write(f"round: {round_id}\nrevision: {label}\n{output}\n")
                    log.flush()
                    cpu = re.search(r"^cpu: (.+)$", output, re.M)
                    if cpu and goenv["GOOS"] == "windows":
                        record["environment"]["cpu"] = cpu.group(1).strip()
                    record["samples"][label][name].append(parse(output, name))
    (args.output / "result.json").write_text(json.dumps(record, indent=2), encoding="utf-8")
    lines = ["### Benchmark measurements", "", f"Go `{goenv['GOVERSION']}` · {goenv['GOOS']}/{goenv['GOARCH']}", "",
             "| Operation | Head ns/op | Stdlib speedup |", "| --- | ---: | ---: |"]
    for name, row in summary(record).items():
        ratio = f"{row['speedup']:.2f}x" if "speedup" in row else "—"
        lines.append(f"| {name} | {row['ns']:.3f} | {ratio} |")
    if os.environ.get("GITHUB_STEP_SUMMARY"):
        with open(os.environ["GITHUB_STEP_SUMMARY"], "a", encoding="utf-8") as out:
            out.write("\n".join(lines) + "\n")


if __name__ == "__main__":
    main()
