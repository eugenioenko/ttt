import copy
import importlib.util
import io
import json
import unittest
import zipfile
from pathlib import Path
from unittest.mock import patch


spec = importlib.util.spec_from_file_location("publisher", Path(__file__).with_name("publish-pr-reports.py"))
publisher = importlib.util.module_from_spec(spec)
spec.loader.exec_module(publisher)


class PublishReportsTest(unittest.TestCase):
    def setUp(self):
        self.run = {
            "id": 42, "run_attempt": 2, "event": "pull_request", "conclusion": "failure",
            "head_sha": "a" * 40, "head_branch": "feature",
            "head_repository": {"full_name": "contributor/ttt", "owner": {"login": "contributor"}},
            "pull_requests": [],
        }
        self.pr = {
            "number": 7, "state": "open", "base": {"ref": "main"},
            "head": {"sha": "a" * 40, "ref": "feature", "repo": {"full_name": "contributor/ttt"}},
        }
        self.artifacts = [
            {"id": index, "name": f"{name}-2", "expired": False, "size_in_bytes": 100}
            for index, (name, _) in enumerate(publisher.REPORTS, 1)
        ]
        self.comments = []
        self.current_pr = self.pr
        self.attempt = 2

    def api(self, path, *args):
        if path.endswith("/actions/runs/42"):
            return {"run_attempt": self.attempt}
        if "/pulls?" in path:
            self.assertIn(f"head={self.run['head_repository']['owner']['login']}%3Afeature", path)
            return [[self.pr]]
        if path.endswith("/pulls/7"):
            return self.current_pr
        if "/artifacts?" in path:
            return [{"artifacts": self.artifacts}]
        if "/comments?" in path:
            return [[], self.comments]
        self.fail(f"Unexpected API call: {path}")

    def publish(self):
        with patch.object(publisher, "api", side_effect=self.api), \
             patch.object(publisher, "read_report", side_effect=lambda repo, artifact, filename, marker: marker + "\nreport"), \
             patch.object(publisher.subprocess, "run") as write:
            publisher.publish("eugenioenko/ttt", self.run)
            return write.call_args_list

    def test_fork_failed_ci_publishes_both_reports(self):
        calls = self.publish()
        self.assertEqual(len(calls), 2)
        for call in calls:
            self.assertIn("repos/eugenioenko/ttt/issues/7/comments", call.args[0])
            self.assertIn("POST", call.args[0])
            body = json.loads(call.kwargs["input"])["body"]
            self.assertIn("`aaaaaaaaaaaa`", body)
            self.assertIn("/actions/runs/42", body)

    def test_update_only_actions_bot_comment(self):
        marker = publisher.REPORTS[0][1]
        self.comments = [
            {"id": 10, "user": {"login": "contributor"}, "body": marker},
            {"id": 11, "user": {"login": "github-actions[bot]"}, "body": marker},
        ]
        calls = self.publish()
        self.assertIn("repos/eugenioenko/ttt/issues/comments/11", calls[0].args[0])
        self.assertIn("PATCH", calls[0].args[0])
        self.assertIn("POST", calls[1].args[0])

    def test_same_repository_pr_still_publishes(self):
        self.run["head_repository"] = {"full_name": "eugenioenko/ttt", "owner": {"login": "eugenioenko"}}
        self.pr["head"]["repo"]["full_name"] = "eugenioenko/ttt"
        self.run["conclusion"] = "success"
        self.assertEqual(len(self.publish()), 2)

    def test_stale_closed_or_wrong_repository_pr_is_ignored(self):
        for change in ("sha", "repo", "state", "base"):
            with self.subTest(change=change):
                pr = copy.deepcopy(self.pr)
                if change == "sha":
                    pr["head"]["sha"] = "b" * 40
                elif change == "repo":
                    pr["head"]["repo"]["full_name"] = "someone-else/ttt"
                elif change == "state":
                    pr["state"] = "closed"
                else:
                    pr["base"]["ref"] = "other"
                with patch.object(self, "pr", pr):
                    self.assertEqual(self.publish(), [])

    def test_head_changed_during_download_is_ignored(self):
        self.current_pr = copy.deepcopy(self.pr)
        self.current_pr["head"]["sha"] = "b" * 40
        self.assertEqual(self.publish(), [])

    def test_old_attempt_is_ignored(self):
        self.attempt = 3
        self.assertEqual(self.publish(), [])

    def test_missing_expired_and_old_attempt_reports_are_skipped(self):
        self.artifacts[0]["expired"] = True
        self.artifacts[1]["name"] = "size-report-1"
        self.assertEqual(self.publish(), [])
        self.artifacts = self.artifacts[:1]
        self.artifacts[0]["expired"] = False
        self.assertEqual(len(self.publish()), 1)

    def test_non_pr_and_unfinished_runs_are_ignored(self):
        for event, conclusion in (("push", "success"), ("pull_request", "action_required"), ("pull_request", "cancelled")):
            with self.subTest(event=event, conclusion=conclusion):
                self.run.update(event=event, conclusion=conclusion)
                self.assertEqual(self.publish(), [])

    def test_archive_is_read_as_text_without_extraction(self):
        marker = publisher.REPORTS[0][1]
        body = marker + "\n$(touch /tmp/should-not-run) `uname`"
        cases = [
            ({"editor-bench.md": body}, True),
            ({"../editor-bench.md": body}, False),
            ({"editor-bench.md": body, "script.sh": "exit 1"}, False),
            ({"editor-bench.md": "missing marker"}, False),
            ({"editor-bench.md": marker + "\n" + "x" * publisher.MAX_REPORT_BYTES}, False),
        ]
        for entries, valid in cases:
            with self.subTest(entries=list(entries), valid=valid):
                data = io.BytesIO()
                with zipfile.ZipFile(data, "w", zipfile.ZIP_DEFLATED) as archive:
                    for name, text in entries.items():
                        archive.writestr(name, text)
                with patch.object(publisher.subprocess, "check_output", return_value=data.getvalue()):
                    if valid:
                        self.assertEqual(publisher.read_report("owner/repo", self.artifacts[0], "editor-bench.md", marker), body)
                    else:
                        with self.assertRaises(ValueError):
                            publisher.read_report("owner/repo", self.artifacts[0], "editor-bench.md", marker)


if __name__ == "__main__":
    unittest.main()
