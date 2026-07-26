"""Tests for the `auto` CLI (spec 005-job-runner-ui).

`auto` is an extensionless executable script, so it can't be imported by name —
it's loaded by path via importlib below. Every test that touches workspace
state runs against a throwaway temp workspace (see `temp_workspace`), never the
real one, so running the suite can never write to the operator's own
data/state/runs.sqlite or logs/.
"""
from __future__ import annotations

import contextlib
import importlib.machinery
import importlib.util
import os
import shutil
import subprocess
import sys
import tempfile
import time
import unittest
from pathlib import Path

TOOLS_DIR = Path(__file__).resolve().parent
AUTO_PATH = TOOLS_DIR / "auto"


def load_auto(workspace: Path | None = None):
    """Import the `auto` script fresh, optionally rooted at a given workspace.

    `auto` resolves WS/DATA at import time from AUTO_WORKSPACE, so the env var
    must be set before the module executes — hence a fresh load per workspace
    rather than a module-level import.
    """
    env_before = os.environ.get("AUTO_WORKSPACE")
    if workspace is not None:
        os.environ["AUTO_WORKSPACE"] = str(workspace)
    try:
        # An explicit SourceFileLoader is required: `auto` has no .py suffix, so
        # spec_from_file_location can't infer a loader and returns None.
        loader = importlib.machinery.SourceFileLoader("auto_cli", str(AUTO_PATH))
        spec = importlib.util.spec_from_loader(loader.name, loader)
        mod = importlib.util.module_from_spec(spec)
        loader.exec_module(mod)
        return mod
    finally:
        if workspace is not None:
            if env_before is None:
                os.environ.pop("AUTO_WORKSPACE", None)
            else:
                os.environ["AUTO_WORKSPACE"] = env_before


@contextlib.contextmanager
def temp_workspace():
    """A minimal but valid workspace: packs.yaml + data/ + config/ai/."""
    d = Path(tempfile.mkdtemp(prefix="auto-test-ws-"))
    try:
        (d / "packs.yaml").write_text("packs: []\n")
        (d / "data" / "state").mkdir(parents=True)
        (d / "config" / "ai").mkdir(parents=True)
        (d / "orchestrator").mkdir()
        yield d
    finally:
        shutil.rmtree(d, ignore_errors=True)


def tmux_present() -> bool:
    return shutil.which("tmux") is not None


requires_tmux = unittest.skipUnless(tmux_present(), "tmux not installed")


class TestModuleLoads(unittest.TestCase):
    def test_auto_script_imports(self):
        with temp_workspace() as ws:
            mod = load_auto(ws)
            self.assertEqual(mod.WS, ws)
            self.assertEqual(mod.DATA, ws / "data")


class TestAuditId(unittest.TestCase):
    def setUp(self):
        self._ws = temp_workspace()
        self.ws = self._ws.__enter__()
        self.auto = load_auto(self.ws)

    def tearDown(self):
        self._ws.__exit__(None, None, None)

    def test_format(self):
        aid = self.auto.new_audit_id()
        self.assertRegex(aid, r"^r-\d{8}-\d{6}-[0-9a-f]{6}$")

    def test_unique_across_rapid_calls(self):
        ids = {self.auto.new_audit_id() for _ in range(500)}
        self.assertEqual(len(ids), 500, "audit ids collided within the same second")

    def test_sorts_chronologically_as_plain_string(self):
        # The format exists so string sort == time sort; assert that directly.
        early = "r-20260101-000000-aaaaaa"
        later = "r-20260726-153012-000000"
        self.assertLess(early, later)

    def test_status_for_rc(self):
        self.assertEqual(self.auto.status_for_rc(0), "succeeded")
        self.assertEqual(self.auto.status_for_rc(1), "failed")
        # 124 is execute_job's timeout convention, not a generic failure.
        self.assertEqual(self.auto.status_for_rc(124), "timed_out")


class TestRunStore(unittest.TestCase):
    def setUp(self):
        self._ws = temp_workspace()
        self.ws = self._ws.__enter__()
        self.auto = load_auto(self.ws)

    def tearDown(self):
        self._ws.__exit__(None, None, None)

    def _make(self, audit_id="r-20260726-000000-abc123", kind="job", action_id="hello-report"):
        self.auto.create_run(audit_id, kind, action_id, "",
                             self.auto.tmux_session_name(audit_id),
                             f"logs/runs/{audit_id}.log")
        return audit_id

    def test_schema_is_idempotent_and_preserves_existing_runs_table(self):
        import sqlite3
        db = self.auto._runs_db()
        c = sqlite3.connect(db)
        c.execute("CREATE TABLE IF NOT EXISTS runs(ts TEXT, job TEXT, host TEXT, rc INTEGER, dur REAL)")
        c.execute("INSERT INTO runs VALUES ('t','j','h',0,1.0)")
        c.commit(); c.close()

        c = self.auto._ui_runs_connect(); self.auto._ui_runs_schema(c); c.close()
        c = self.auto._ui_runs_connect(); self.auto._ui_runs_schema(c)  # twice = safe
        rows = c.execute("SELECT COUNT(*) FROM runs").fetchone()[0]
        c.close()
        self.assertEqual(rows, 1, "pre-existing runs table must be left intact")

    def test_create_and_get(self):
        aid = self._make()
        r = self.auto.get_run(aid)
        self.assertEqual(r["status"], "running")
        self.assertIsNone(r["rc"])
        self.assertIsNone(r["ended_at"])
        self.assertEqual(r["action_id"], "hello-report")

    def test_finish_run_sets_status_from_rc(self):
        aid = self._make()
        self.auto.finish_run(aid, 0)
        self.assertEqual(self.auto.get_run(aid)["status"], "succeeded")

    def test_finish_run_maps_timeout(self):
        aid = self._make()
        self.auto.finish_run(aid, 124)
        self.assertEqual(self.auto.get_run(aid)["status"], "timed_out")

    def test_list_runs_newest_first(self):
        self._make("r-20260101-000000-aaaaaa")
        self._make("r-20260726-120000-bbbbbb")
        ids = [r["audit_id"] for r in self.auto.list_runs()]
        self.assertEqual(ids[0], "r-20260726-120000-bbbbbb")

    def test_unknown_run_returns_none(self):
        self.assertIsNone(self.auto.get_run("r-does-not-exist"))

    def test_reconcile_moves_dead_running_run_off_running(self):
        # Session name that certainly does not exist -> must be reconciled.
        aid = self._make("r-20260726-000000-dead01")
        fixed = self.auto.reconcile_running_runs()
        self.assertEqual(fixed, 1)
        r = self.auto.get_run(aid)
        self.assertNotEqual(r["status"], "running")
        self.assertEqual(r["status"], "failed")
        self.assertIn("abnormally", (r["note"] or ""))

    def test_reconcile_leaves_finished_runs_alone(self):
        aid = self._make()
        self.auto.finish_run(aid, 0)
        self.auto.reconcile_running_runs()
        self.assertEqual(self.auto.get_run(aid)["status"], "succeeded")

    def test_steps_roundtrip(self):
        aid = self._make(kind="orchestration", action_id="gmail-wallet-sync")
        self.auto.record_step(aid, 0, "gmail-extract", "start")
        self.auto.record_step(aid, 0, "gmail-extract", "end", rc=0)
        self.auto.record_step(aid, 1, "gmail-categorize", "start")
        steps = self.auto.get_run_steps(aid)
        self.assertEqual(len(steps), 2)
        self.assertEqual(steps[0]["status"], "succeeded")
        self.assertEqual(steps[1]["status"], "running")


class TestRunLogs(unittest.TestCase):
    def setUp(self):
        self._ws = temp_workspace()
        self.ws = self._ws.__enter__()
        self.auto = load_auto(self.ws)

    def tearDown(self):
        self._ws.__exit__(None, None, None)

    def test_format_log_line_carries_audit_id(self):
        line = self.auto.format_log_line("r-20260726-000000-abc123", "hello world")
        self.assertIn("r-20260726-000000-abc123", line)
        self.assertTrue(line.endswith("\n"))
        self.assertRegex(line, r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z r-\S+ hello world\n$")

    def test_format_log_line_does_not_double_newline(self):
        line = self.auto.format_log_line("r-x", "text\n")
        self.assertEqual(line.count("\n"), 1)

    def test_read_log_slice_missing_file_is_safe(self):
        text, nxt, size = self.auto.read_log_slice(self.ws / "nope.log", 0)
        self.assertEqual((text, nxt, size), ("", 0, 0))

    def test_read_log_slice_resumes_from_offset(self):
        p = self.ws / "a.log"
        p.write_text("line1\n")
        text, nxt, _ = self.auto.read_log_slice(p, 0)
        self.assertEqual(text, "line1\n")
        # No new data yet: same offset back, empty text.
        text2, nxt2, _ = self.auto.read_log_slice(p, nxt)
        self.assertEqual(text2, "")
        self.assertEqual(nxt2, nxt)
        # Append, then only the new bytes come back.
        with open(p, "a") as f:
            f.write("line2\n")
        text3, _, _ = self.auto.read_log_slice(p, nxt)
        self.assertEqual(text3, "line2\n")

    def test_read_log_slice_offset_past_eof(self):
        p = self.ws / "b.log"
        p.write_text("xy")
        text, nxt, size = self.auto.read_log_slice(p, 999)
        self.assertEqual(text, "")
        self.assertEqual(size, 2)


@requires_tmux
class TestTmuxRunner(unittest.TestCase):
    def setUp(self):
        self._ws = temp_workspace()
        self.ws = self._ws.__enter__()
        self.auto = load_auto(self.ws)
        self.sessions = []

    def tearDown(self):
        for s in self.sessions:
            self.auto.tmux_kill_session(s)
        self._ws.__exit__(None, None, None)

    def _session(self, suffix):
        name = f"auto-test-{os.getpid()}-{suffix}"
        self.sessions.append(name)
        return name

    def _wait_gone(self, name, timeout=10.0):
        deadline = time.time() + timeout
        while time.time() < deadline:
            if not self.auto.tmux_has_session(name):
                return True
            time.sleep(0.1)
        return False

    def test_available(self):
        self.assertTrue(self.auto.tmux_available())

    def test_session_self_destructs_after_command_exits(self):
        name = self._session("selfdestruct")
        out = self.ws / "out.txt"
        self.auto.tmux_launch(name, ["sh", "-c", f"echo done > {out}"], self.ws)
        self.assertTrue(self._wait_gone(name),
                        "tmux session should disappear on its own once the command exits")
        self.assertEqual(out.read_text().strip(), "done")

    def test_kill_session_stops_a_long_run(self):
        name = self._session("longrun")
        self.auto.tmux_launch(name, ["sleep", "60"], self.ws)
        time.sleep(0.5)
        self.assertTrue(self.auto.tmux_has_session(name))
        self.auto.tmux_kill_session(name)
        self.assertTrue(self._wait_gone(name))

    def test_launched_command_sees_non_tty_stdin(self):
        """Regression guard for spec FR-015.

        tmux gives each pane a real PTY, so without the `< /dev/null` redirect
        in tmux_launch the packs' isInteractive() would be true and jobs would
        block on prompts nobody can answer. If this test fails, dashboard runs
        will hang.
        """
        name = self._session("tty")
        out = self.ws / "tty.txt"
        probe = "if [ -t 0 ]; then echo TTY > %s; else echo NOTTY > %s; fi" % (out, out)
        self.auto.tmux_launch(name, ["sh", "-c", probe], self.ws)
        self.assertTrue(self._wait_gone(name))
        self.assertEqual(out.read_text().strip(), "NOTTY",
                         "stdin must NOT be a tty inside a dashboard run (FR-015)")


if __name__ == "__main__":
    unittest.main()
