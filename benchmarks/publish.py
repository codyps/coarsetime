"""Trusted workflow_run publisher. Treat downloaded artifacts exclusively as data."""
import argparse
import json
import math
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile

from stats import ALTERNATIVES, NAMES, compare, summary

HERE = Path(__file__).resolve().parent
RUNNERS = {"ubuntu-24.04", "ubuntu-24.04-arm", "macos-15", "macos-15-intel", "windows-2025", "windows-11-arm"}
MARKER = "<!-- coarsetime-benchmarks -->"


def gh(path, method="GET", body=None):
    args = ["gh", "api", path, "--method", method]
    if body is not None:
        args += ["--input", "-"]
    text = subprocess.check_output(args, input=json.dumps(body) if body is not None else None, text=True)
    return json.loads(text) if text.strip() else None


def validate(record, run):
    if record.get("schema") != 1 or record.get("run_id") != run["id"] or record.get("run_attempt") != run["run_attempt"]:
        raise ValueError("Artifact does not belong to the completed run/attempt")
    if record.get("event") != run["event"] or record.get("runner") not in RUNNERS:
        raise ValueError("Unexpected event or runner")
    if not re.fullmatch(r"(?:stable|1\.\d+\.x)", record.get("go_selector", "")):
        raise ValueError("Invalid Go selector")
    if not re.fullmatch(r"[a-f0-9]{64}", record.get("harness", "")):
        raise ValueError("Invalid harness hash")
    if type(record.get("pr")) is not int or record["pr"] < 0:
        raise ValueError("Invalid PR number")
    for sha in record.get("commits", {}).values():
        if not isinstance(sha, str) or not re.fullmatch(r"[a-f0-9]{40}", sha):
            raise ValueError("Invalid commit")
    if record["commits"].get("head") != run["head_sha"]:
        raise ValueError("Head SHA does not match workflow run")
    version = record["environment"]["go"]["GOVERSION"]
    if not re.fullmatch(r"go1\.\d+\.\d+", version):
        raise ValueError("Expected exact release Go version")
    labels = {"head", "base"} if run["event"] == "pull_request" else {"head"}
    if set(record["samples"]) != labels or set(record["commits"]) != labels:
        raise ValueError("Missing revision")
    for cases in record["samples"].values():
        if set(cases) != set(NAMES):
            raise ValueError("Missing benchmark")
        for samples in cases.values():
            if len(samples) != 10:
                raise ValueError("Expected ten samples per case")
            for sample in samples:
                if set(sample) != {"ns", "bytes", "allocs"}:
                    raise ValueError("Invalid sample fields")
                if any(type(n) not in (int, float) or not math.isfinite(n) or n < 0 or n > 1e12 for n in sample.values()) or sample["ns"] == 0:
                    raise ValueError("Invalid sample value")
    return record


def comment_body(records, run, repo, config):
    lines = [MARKER, f"<!-- benchmark-run: {run['id']} -->", "## Performance comparison", "",
             f"Base `{records[0]['commits']['base'][:12]}` → head `{run['head_sha'][:12]}`.", "",
             "Both revisions were built with the same harness and Go version, then run alternately on each worker (10 paired rounds).",
             f"Flags require ≥{config['threshold_percent']}% and ≥{config['threshold_ns']} ns/op change, with a "
             f"{config['bootstrap_confidence']:.0%} paired-bootstrap interval excluding zero. Positive means slower.", ""]
    flagged = []
    controls = []
    for record in records:
        label = f"{record['runner']} / {record['environment']['go']['GOVERSION']}"
        for name in NAMES:
            values = {rev: [s["ns"] for s in record["samples"][rev][name]] for rev in ("base", "head")}
            change = compare(values["base"], values["head"], config)
            if not change["alert"]:
                continue
            row = (f"| {label} | {name} | {change['before']:.3f} | {change['after']:.3f} | "
                   f"{change['percent']:+.1f}% | {change['interval'][0]:+.1f}% to {change['interval'][1]:+.1f}% |")
            (flagged if name in ALTERNATIVES else controls).append(row)
    if flagged:
        lines += ["| Runner / Go | Operation | Before ns/op | After ns/op | Change | Interval |",
                  "| --- | --- | ---: | ---: | ---: | ---: |"] + flagged
    else:
        lines += ["No coarsetime operation crosses the reporting thresholds in this run."]
    if controls:
        lines += ["", "Stdlib controls also moved; inspect these when interpreting the changes:", "",
                  "| Runner / Go | Operation | Before ns/op | After ns/op | Change | Interval |",
                  "| --- | --- | ---: | ---: | ---: | ---: |"] + controls
    lines += ["", "These are informational measurements; same-worker pairing reduces environment drift but does not eliminate noise.",
              f"[All measurements and raw samples](https://github.com/{repo}/actions/runs/{run['id']}) · "
              f"[Historical dashboard](https://{repo.split('/')[0]}.github.io/{repo.split('/')[1]}/)"]
    return "\n".join(lines), bool(flagged)


def update_comment(records, run, repo, config):
    numbers = {r["pr"] for r in records}
    if len(numbers) != 1 or not next(iter(numbers)):
        raise ValueError("Inconsistent PR number")
    number = next(iter(numbers))
    pr = gh(f"repos/{repo}/pulls/{number}")
    # Artifact metadata cannot select another PR, fork, commit, or obsolete base.
    if (pr["state"] != "open" or pr["head"]["sha"] != run["head_sha"]
            or pr["head"]["repo"]["full_name"] != run["head_repository"]["full_name"]
            or pr["head"]["ref"] != run["head_branch"]
            or any(r["commits"]["base"] != pr["base"]["sha"] for r in records)):
        print("Skipping obsolete or mismatched PR comparison")
        return
    body, flagged = comment_body(records, run, repo, config)
    comments = []
    page = 1
    while True:
        batch = gh(f"repos/{repo}/issues/{number}/comments?per_page=100&page={page}")
        comments.extend(batch)
        if len(batch) < 100:
            break
        page += 1
    previous = next((c for c in comments if c["user"]["login"] == "github-actions[bot]" and c["body"].startswith(MARKER)), None)
    if previous:
        old_run = re.search(r"benchmark-run: (\d+)", previous["body"])
        if old_run and int(old_run[1]) > run["id"]:
            return
        gh(f"repos/{repo}/issues/comments/{previous['id']}", "PATCH", {"body": body})
    elif flagged:
        gh(f"repos/{repo}/issues/{number}/comments", "POST", {"body": body})


def git(*args, cwd=None, check=True):
    return subprocess.run(["git", *args], cwd=cwd, check=check, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)


def save_history(records, sources, run, repo):
    # A temporary worktree contains only generated files. Retry non-fast-forward
    # pushes so concurrent completed runs never discard each other's history.
    root = Path(git("rev-parse", "--show-toplevel").stdout.strip()).resolve()
    with tempfile.TemporaryDirectory(prefix="benchmark-history-") as temp:
        worktree = Path(temp) / "site"
        git("worktree", "add", "--detach", str(worktree), "HEAD", cwd=root)
        try:
            for attempt in range(5):
                remote = git("ls-remote", "--heads", "origin", "benchmarks", cwd=root).stdout.strip()
                if remote:
                    git("fetch", "origin", "refs/heads/benchmarks", cwd=root)
                    fetched = git("rev-parse", "FETCH_HEAD", cwd=root).stdout.strip()
                    git("reset", "--hard", fetched, cwd=worktree)
                else:
                    git("switch", "--orphan", f"benchmark-history-{run['id']}-{attempt}", cwd=worktree)
                data = worktree / "data"
                data.mkdir(exist_ok=True)
                for record, source in zip(records, sources):
                    key = f"{run['id']}-{run['run_attempt']}-{record['runner']}-{record['go_selector']}"
                    record["summary"] = summary(record)
                    record["run_url"] = f"https://github.com/{repo}/actions/runs/{run['id']}"
                    (data / f"{key}.json").write_text(json.dumps(record, indent=2), encoding="utf-8")
                    shutil.copyfile(source.parent / "raw.txt", data / f"{key}.txt")
                entries = []
                for path in sorted(data.glob("*.json")):
                    if path.name == "index.json":
                        continue
                    record = json.loads(path.read_text(encoding="utf-8"))
                    entries.append({key: record[key] for key in ("timestamp", "runner", "go_selector", "environment", "harness", "commits", "summary", "run_url", "run_id", "run_attempt")} | {"file": path.name})
                entries.sort(key=lambda e: (e["run_id"], e["run_attempt"]))
                (data / "index.json").write_text(json.dumps(entries), encoding="utf-8")
                for asset in ("index.html", "dashboard-data.js", "dashboard.js"):
                    shutil.copyfile(HERE / asset, worktree / asset)
                (worktree / ".nojekyll").touch()
                git("add", "--all", cwd=worktree)
                if git("diff", "--cached", "--quiet", cwd=worktree, check=False).returncode == 0:
                    return
                git("-c", "user.name=github-actions[bot]", "-c", "user.email=41898282+github-actions[bot]@users.noreply.github.com",
                    "commit", "-m", f"Record benchmark run {run['id']} attempt {run['run_attempt']}", cwd=worktree)
                if git("push", "origin", "HEAD:refs/heads/benchmarks", cwd=worktree, check=False).returncode == 0:
                    return
            raise RuntimeError("Could not publish benchmark history after five push attempts")
        finally:
            git("worktree", "remove", "--force", str(worktree), cwd=root, check=False)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("artifacts", type=Path)
    args = parser.parse_args()
    event = json.loads(Path(os.environ["GITHUB_EVENT_PATH"]).read_text())
    run = event["workflow_run"]
    repo = os.environ["GITHUB_REPOSITORY"]
    live = gh(f"repos/{repo}/actions/runs/{run['id']}")
    if (live["run_attempt"] != run["run_attempt"] or live["conclusion"] != "success"
            or live["path"] != ".github/workflows/benchmarks.yml"):
        raise ValueError("Unexpected or superseded source workflow")
    sources = sorted(args.artifacts.glob("benchmark-*/result.json"))
    if not sources or len(sources) > 30:
        raise ValueError("Missing or excessive result artifacts")
    records = []
    for source in sources:
        if source.is_symlink() or source.stat().st_size > 2_000_000:
            raise ValueError("Invalid result file")
        raw = source.parent / "raw.txt"
        if raw.is_symlink() or not raw.is_file() or raw.stat().st_size > 20_000_000:
            raise ValueError("Invalid raw output")
        records.append(validate(json.loads(source.read_text(encoding="utf-8")), run))
    keys = {(r["runner"], r["go_selector"]) for r in records}
    selectors = {"1.23.x", "stable"} | ({"1.24.x", "1.25.x", "1.26.x"} if run["event"] == "schedule" else set())
    if keys != {(runner, selector) for runner in RUNNERS for selector in selectors} or len(keys) != len(records):
        raise ValueError("Incomplete or duplicate benchmark matrix")
    if len({r["harness"] for r in records}) != 1:
        raise ValueError("Matrix used different harnesses")
    config = json.loads((HERE / "config.json").read_text())
    if run["event"] == "pull_request":
        update_comment(records, run, repo, config)
    elif (run["event"] in ("push", "schedule", "workflow_dispatch")
          and run["head_branch"] == "main" and run["head_repository"]["full_name"] == repo):
        save_history(records, sources, run, repo)
        with open(os.environ["GITHUB_OUTPUT"], "a", encoding="utf-8") as out:
            out.write("deploy=true\n")


if __name__ == "__main__":
    main()
