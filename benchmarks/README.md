# Continuous benchmarks

The [dashboard](https://codyps.github.io/coarsetime/) is generated from the
[`benchmarks` branch](https://github.com/codyps/coarsetime/tree/benchmarks).
It contains main-branch measurements and raw evidence, separated by runner,
exact Go version, benchmark harness, and OS/CPU/image environment. CI runs on
Linux, macOS, and Windows, each on amd64 and arm64.

## Workflows

- `Benchmarks` runs on main pushes, pull requests, manual dispatch, and Monday's
  schedule. Each target runs Go 1.23 and stable. Scheduled runs also cover
  1.24, 1.25, and 1.26. Update the workflow matrix and publisher's expected
  selectors when adding a Go minor. Exact resolved patch versions are recorded;
  the dashboard does not combine different versions of `stable`.
- Each PR worker checks out the event's exact base and head SHAs, builds both
  with the same toolchain and the same external-package harness, validates them,
  warms both binaries, and alternates measurement order for ten rounds. There
  is no dependency on last week's CI measurements for PR decisions. The base is
  the target branch tip captured by the PR event, not the merge-base or a
  synthetic merge commit.
- `Publish benchmarks` runs trusted default-branch scripts after successful
  measurement workflows. Main results are appended to the `benchmarks` branch,
  which is created automatically. Each result and its raw log have a unique
  run/attempt/runner/toolchain filename. Concurrent writers retry normal
  fast-forward pushes; nothing force-pushes or truncates history.
- A separate Pages job deploys the latest accumulated branch through the Pages
  artifact API. This works even though commits made with `GITHUB_TOKEN` do not
  trigger a new branch-based Pages build.
- PR results stay in run artifacts (30 days), with a sticky bot comment when a
  coarsetime operation crosses the configured thresholds. Both improvements and
  regressions are reported. An existing comment is updated to clear old warnings
  when subsequent measurements no longer cross the thresholds. Results for
  obsolete PR heads or changed base SHAs are not posted. Update/rebase the PR or
  trigger a new PR event if its base has advanced.

Publishing requires the publisher workflow to exist on `main`. The first PR
introducing it cannot produce a trusted workflow-run comment until it is merged.
Benchmark workers have read-only permissions and no retained checkout credentials.
The publisher parses artifacts as data; it never executes downloaded scripts or
checks out PR code. It validates run identity, revision, matrix completeness,
samples, and the current PR before posting.

## Reporting thresholds

[`config.json`](config.json) sets the measurement and reporting defaults:

- Ten rounds, 500 ms per operation/revision, one Go execution thread, CGO disabled.
- At least **10%** median paired change and **0.5 ns/op** median paired absolute
  difference, with a **99% paired-bootstrap interval excluding zero**.
- Statistical analysis resamples per-round head/base percentage differences
  together, using 5,000 deterministic bootstrap draws. Positive change means
  slower. These are informational alerts, not required performance gates.

The publisher uses the config from `main`, so a PR cannot lower its own reporting
thresholds. Changing the ten-round protocol also requires updating publisher
validation and tests. The confidence interval is an estimate from within-job
samples, not a guarantee or a correction for multiple comparisons. Same-worker
pairing reduces machine drift; scheduling and thermal noise can still remain.

Stdlib controls are included in every revision's run and displayed separately in
comments if they move substantially. They do not replace the absolute coarsetime
change test: normalizing everything by stdlib could hide a real shared regression.
The dashboard's speedup is the median of per-round stdlib/coarsetime ratios.

`harness_test.go.txt` is deliberately outside Go's normal test discovery. The
runner temporarily adds identical source to both checkouts. It uses direct calls
and a Go-1.23-compatible loop, with global sinks to preserve results. Harness
hashes partition history when the measurement method changes. Benchmarks cover
`NowInstant`, `Since`, `Now`, and `UnixNano`, plus their three unique stdlib
alternatives. Parallel, callback, purego, and clock-resolution diagnostics remain
outside this initial history suite.

## Setup and maintenance

Set repository **Settings → Pages → Build and deployment → Source** to
**GitHub Actions**. The workflows use only `GITHUB_TOKEN`; no service account or
additional secret is needed. Repository/organization policies must permit the
declared contents, pull-request, and Pages write permissions, and the `github-pages`
environment must allow the publisher to deploy from main.

Merge the workflows to main, then run `Benchmarks` manually if the main push has
not already triggered a run. A complete 12-cell matrix should create the history
branch and deploy the dashboard. Partial or failed matrices retain diagnostic
artifacts but do not publish a misleading complete report.

All archived samples remain in the history branch, including repeated runs of
the same commit. `data/index.json` is a derived dashboard index; the per-run JSON
and text files are the durable evidence. No history retention limit is applied.
The index currently loads in full; paginate it if the archive becomes large.

Run local checks with:

```sh
python -m unittest discover -s benchmarks -p 'test_*.py' -v
node --check benchmarks/dashboard.js
```

For a native end-to-end smoke run, provide two separate checkouts (or the same
checkout for an A/A comparison) and an output directory outside those checkouts:

```sh
python benchmarks/run.py --base /path/to/base --head /path/to/head --output /tmp/coarsetime-bench
```

The runner builds and runs each revision's short correctness tests before timing.
Linux vDSO and Darwin calendar required-path checks are enabled on native targets.
This validates path availability, not that every subsequent Darwin read avoids
fallback. A build or validation failure aborts measurements.

Go-version comparisons in the dashboard describe separate hosted jobs, not
controlled compiler experiments on one physical machine. PR before/after
comparisons are the same-worker comparisons. For attribution of small differences
between Go versions, run a fixed commit with both toolchains on a controlled host.

See the [initial investigation](../research/continuous-benchmarking.md) for service
alternatives and extensions. Relevant GitHub documentation:
[workflow_run](https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows#workflow_run),
[Pages custom workflows](https://docs.github.com/en/pages/getting-started-with-github-pages/using-custom-workflows-with-github-pages),
and [native runner labels](https://docs.github.com/en/actions/reference/runners/github-hosted-runners).
