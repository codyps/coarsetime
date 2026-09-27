import copy
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import publish
from run import parse
from stats import NAMES, compare, summary

CONFIG = json.loads((Path(__file__).parent / "config.json").read_text())


def fixture():
    run = {"id": 42, "run_attempt": 1, "head_sha": "a" * 40, "event": "pull_request",
           "head_repository": {"full_name": "owner/repo"}, "head_branch": "feature"}
    record = {"schema": 1, "run_id": 42, "run_attempt": 1, "event": "pull_request", "pr": 2,
              "runner": "windows-2025", "go_selector": "stable", "harness": "b" * 64,
              "environment": {"go": {"GOVERSION": "go1.27.0"}},
              "timestamp": "2026-09-27T12:00:00Z", "commits": {"base": "c" * 40, "head": "a" * 40},
              "samples": {rev: {name: [{"ns": ns, "bytes": 0, "allocs": 0} for _ in range(10)]
                                 for name in NAMES} for rev, ns in (("base", 10), ("head", 12))}}
    return run, record


class AnalysisTests(unittest.TestCase):
    def test_changes_in_both_directions(self):
        self.assertTrue(compare([10] * 10, [12] * 10, CONFIG)["alert"])
        result = compare([10] * 10, [8] * 10, CONFIG)
        self.assertTrue(result["alert"])
        self.assertLess(result["percent"], 0)

    def test_absolute_threshold_and_noise(self):
        self.assertFalse(compare([1] * 10, [1.2] * 10, CONFIG)["alert"])
        self.assertFalse(compare([10] * 10, [10.1] * 10, CONFIG)["alert"])
        self.assertFalse(compare([10] * 10, [5, 6, 7, 8, 12, 14, 16, 18, 20, 22], CONFIG)["alert"])

    def test_pairing_cancels_shared_round_drift(self):
        before = list(range(10, 110, 10))
        after = [x * 1.2 for x in before]
        result = compare(before, after, CONFIG)
        self.assertAlmostEqual(result["percent"], 20)
        self.assertTrue(result["alert"])
        self.assertEqual(result, compare(before, after, CONFIG))

    def test_pair_count(self):
        with self.assertRaises(ValueError):
            compare([1] * 10, [2] * 9, CONFIG)

    def test_parse(self):
        self.assertEqual(parse("BenchmarkCINow-1  10000  2.125 ns/op  0 B/op  0 allocs/op\n", "Now")["ns"], 2.125)
        self.assertEqual(parse("BenchmarkCINow  10000  2.125 ns/op  0 B/op  0 allocs/op\n", "Now")["ns"], 2.125)
        for output in ("", "BenchmarkCINow-2 1000 2 ns/op 0 B/op 0 allocs/op", "BenchmarkCISince-1 1000 2 ns/op 0 B/op 0 allocs/op"):
            with self.assertRaises(ValueError):
                parse(output, "Now")

    def test_validation_rejects_malformed_or_wrong_run(self):
        run, record = fixture()
        publish.validate(record, run)
        for field, value in (("run_id", 43), ("run_attempt", 2), ("runner", "evil|runner")):
            invalid = copy.deepcopy(record)
            invalid[field] = value
            with self.assertRaises(ValueError):
                publish.validate(invalid, run)
        for value in (float("nan"), float("inf"), -1, 0):
            invalid = copy.deepcopy(record)
            invalid["samples"]["head"]["Now"][0]["ns"] = value
            with self.assertRaises(ValueError):
                publish.validate(invalid, run)

    def test_summary_ratio(self):
        _, record = fixture()
        record["samples"]["head"]["TimeNow"] = [{"ns": 24, "bytes": 0, "allocs": 0}] * 10
        self.assertEqual(summary(record)["Now"]["speedup"], 2)

    def test_comment_and_controls(self):
        run, record = fixture()
        body, flagged = publish.comment_body([record], run, "owner/repo", CONFIG)
        self.assertTrue(flagged)
        self.assertIn("+20.0%", body)
        self.assertIn("Stdlib controls also moved", body)
        record["samples"]["head"] = copy.deepcopy(record["samples"]["base"])
        self.assertFalse(publish.comment_body([record], run, "owner/repo", CONFIG)[1])

    def test_stale_pr_is_not_commented(self):
        run, record = fixture()
        pr = {"state": "open", "head": {"sha": "d" * 40}}
        with patch.object(publish, "gh", return_value=pr) as api:
            publish.update_comment([record], run, "owner/repo", CONFIG)
            self.assertEqual(api.call_count, 1)

    def test_existing_comment_cleared_when_changes_disappear(self):
        run, record = fixture()
        record["samples"]["head"] = copy.deepcopy(record["samples"]["base"])
        pr = {"state": "open", "head": {"sha": run["head_sha"], "repo": run["head_repository"], "ref": run["head_branch"]}, "base": {"sha": record["commits"]["base"]}}
        old = {"id": 9, "user": {"login": "github-actions[bot]"}, "body": publish.MARKER}
        with patch.object(publish, "gh", side_effect=[pr, [old], None]) as api:
            publish.update_comment([record], run, "owner/repo", CONFIG)
            self.assertEqual(api.call_args.args[1], "PATCH")
            self.assertIn("No coarsetime operation", api.call_args.args[2]["body"])

    def test_no_comment_when_no_threshold_crossed(self):
        run, record = fixture()
        record["samples"]["head"] = copy.deepcopy(record["samples"]["base"])
        pr = {"state": "open", "head": {"sha": run["head_sha"], "repo": run["head_repository"], "ref": run["head_branch"]}, "base": {"sha": record["commits"]["base"]}}
        with patch.object(publish, "gh", side_effect=[pr, []]) as api:
            publish.update_comment([record], run, "owner/repo", CONFIG)
            self.assertEqual(api.call_count, 2)

    def test_history_creation_append_and_idempotence(self):
        # Exercise actual git against an isolated local bare remote; no network.
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            repo, remote = root / "repo", root / "remote.git"
            def git(*args, cwd=None):
                return subprocess.check_output(["git", *args], cwd=cwd, stderr=subprocess.STDOUT, text=True)
            git("init", "--bare", str(remote))
            git("init", str(repo))
            (repo / "README").write_text("source")
            git("add", ".", cwd=repo)
            git("-c", "user.name=test", "-c", "user.email=test@example.org", "commit", "-m", "init", cwd=repo)
            git("remote", "add", "origin", str(remote), cwd=repo)
            source = root / "result.json"
            source.write_text("{}")
            (root / "raw.txt").write_text("raw samples")
            old_cwd = Path.cwd()
            try:
                os.chdir(repo)
                run, record = fixture()
                publish.save_history([record], [source], run, "owner/repo")
                first = git("--git-dir", str(remote), "rev-parse", "benchmarks").strip()
                publish.save_history([record], [source], run, "owner/repo")
                self.assertEqual(first, git("--git-dir", str(remote), "rev-parse", "benchmarks").strip())
                run["id"] = 43
                record["run_id"] = 43
                publish.save_history([record], [source], run, "owner/repo")
                history = json.loads(git("--git-dir", str(remote), "show", "benchmarks:data/index.json"))
                self.assertEqual([r["run_id"] for r in history], [42, 43])
                self.assertTrue((repo / "README").is_file())
            finally:
                os.chdir(old_cwd)


if __name__ == "__main__":
    unittest.main()
