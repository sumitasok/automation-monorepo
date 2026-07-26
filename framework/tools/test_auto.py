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


def write_job(ws: Path, pack: str, job_id: str, **manifest):
    """Add a runnable script job to a temp workspace and mount its pack."""
    jd = ws / "packs" / pack / "jobs" / "misc" / job_id
    jd.mkdir(parents=True, exist_ok=True)
    (jd / "main.sh").write_text("#!/usr/bin/env bash\necho hi\n")
    m = {"id": job_id, "name": manifest.pop("name", job_id), "language": "bash",
         "entrypoint": "main.sh", "description": manifest.pop("description", "a test job"),
         "visibility": "private"}
    m.update(manifest)
    import yaml as _yaml
    (jd / "manifest.yaml").write_text(_yaml.safe_dump(m, sort_keys=False))
    (ws / "packs" / pack / "pack.yaml").write_text(_yaml.safe_dump(
        {"name": pack, "default_visibility": "private"}, sort_keys=False))
    (ws / "packs.yaml").write_text(_yaml.safe_dump(
        {"packs": [{"name": pack, "path": f"packs/{pack}", "writable": True}]}, sort_keys=False))
    return jd


class TestActionCatalog(unittest.TestCase):
    def setUp(self):
        self._ws = temp_workspace()
        self.ws = self._ws.__enter__()

    def tearDown(self):
        self._ws.__exit__(None, None, None)

    def test_jobs_accept_ai_and_are_listed(self):
        write_job(self.ws, "testpack", "job-a", description="does a thing")
        auto = load_auto(self.ws)
        acts = auto.list_actions()
        job = next(a for a in acts if a["id"] == "job-a")
        self.assertEqual(job["kind"], "job")
        self.assertEqual(job["description"], "does a thing")
        self.assertTrue(job["accepts_ai"], "jobs take --ai")
        self.assertFalse(job["danger"])
        self.assertTrue(job["available"])

    def test_orchestrations_do_not_accept_ai(self):
        """`auto orchestrate` has no --ai flag — each step names its own
        profile — so offering a dropdown would imply an override that does
        not exist (spec FR-010)."""
        write_job(self.ws, "testpack", "job-a")
        (self.ws / "orchestrator" / "pipe.yaml").write_text(
            "name: pipe\ndescription: a pipeline\nsteps:\n  - job: job-a\n")
        auto = load_auto(self.ws)
        orch = next(a for a in auto.list_actions() if a["kind"] == "orchestration")
        self.assertEqual(orch["id"], "pipe")
        self.assertFalse(orch["accepts_ai"])
        self.assertTrue(orch["available"])

    def test_orchestration_with_unknown_job_is_unavailable_with_reason(self):
        (self.ws / "orchestrator" / "broken.yaml").write_text(
            "name: broken\nsteps:\n  - job: nope-not-real\n")
        auto = load_auto(self.ws)
        orch = next(a for a in auto.list_actions() if a["id"] == "broken")
        self.assertFalse(orch["available"])
        self.assertIn("nope-not-real", orch["unavailable_reason"])

    def test_only_maintenance_commands_are_dangerous(self):
        auto = load_auto(self.ws)
        cmds = {a["id"]: a for a in auto.list_actions() if a["kind"] == "command"}
        self.assertTrue(cmds["schedule-sync"]["danger"])
        self.assertTrue(cmds["bootstrap"]["danger"])
        for safe in ("list", "packs", "doctor", "catalog"):
            self.assertFalse(cmds[safe]["danger"], f"{safe} must not be flagged dangerous")
        # Interactive / argument-taking commands aren't one-click actions.
        for excluded in ("new", "log", "share", "search"):
            self.assertNotIn(excluded, cmds)

    def test_machine_pinned_job_stays_runnable_with_a_note(self):
        """runs_on is a SCHEDULING constraint, not a manual-run permission.

        cmd_schedule uses job_runs_here to pick what to install on this
        machine; cmd_run never checks it, so `auto run gmail-extract` works by
        hand anywhere. The dashboard must not be stricter than the CLI it
        wraps — otherwise machine-pinned jobs (gmail-extract, wallet-sync,
        which are pinned to home-server) would be unclickable here.
        """
        write_job(self.ws, "testpack", "job-win",
                  runs_on={"os": ["windows"], "machines": ["some-other-box"]})
        auto = load_auto(self.ws)
        job = next(a for a in auto.list_actions() if a["id"] == "job-win")
        self.assertTrue(job["available"], "machine pinning must not block a manual run")
        self.assertIsNone(job["unavailable_reason"])
        self.assertIn("scheduled only on", job["note"])

    def test_job_with_missing_pack_dir_is_unavailable(self):
        write_job(self.ws, "testpack", "job-a")
        shutil.rmtree(self.ws / "packs" / "testpack" / "jobs")
        import yaml as _yaml
        (self.ws / "packs.yaml").write_text(_yaml.safe_dump(
            {"packs": [{"name": "gone", "path": "packs/gone", "writable": True}]}))
        auto = load_auto(self.ws)
        self.assertEqual([a for a in auto.list_actions() if a["kind"] == "job"], [])

    def test_action_argv_shapes(self):
        write_job(self.ws, "testpack", "job-a")
        auto = load_auto(self.ws)
        job = next(a for a in auto.list_actions() if a["id"] == "job-a")
        self.assertEqual(auto.action_argv(job), ["run", "job-a"])
        self.assertEqual(auto.action_argv(job, "deepseek"), ["run", "job-a", "--ai", "deepseek"])
        cmd = next(a for a in auto.list_actions() if a["id"] == "schedule-sync")
        self.assertEqual(auto.action_argv(cmd), ["schedule", "sync"])


class TestAiProfiles(unittest.TestCase):
    def setUp(self):
        self._ws = temp_workspace()
        self.ws = self._ws.__enter__()
        self.aidir = self.ws / "config" / "ai"

    def tearDown(self):
        self._ws.__exit__(None, None, None)

    def test_excludes_example_templates(self):
        (self.aidir / "deepseek.example.yaml").write_text(
            "provider: deepseek\napi_key: REPLACE_ME\n")
        (self.aidir / "real.yaml").write_text("provider: deepseek\napi_key: sk-abc123\n")
        auto = load_auto(self.ws)
        names = [p["name"] for p in auto.list_ai_profiles()]
        self.assertEqual(names, ["real"])

    def test_invalid_profile_reported_unusable_not_dropped(self):
        (self.aidir / "broken.yaml").write_text("provider: deepseek\n")  # no api_key
        auto = load_auto(self.ws)
        prof = next(p for p in auto.list_ai_profiles() if p["name"] == "broken")
        self.assertFalse(prof["usable"])

    def test_never_exposes_credentials(self):
        """SC-007: only the profile name/provider may leave the server."""
        secret = "sk-super-secret-value-000"
        (self.aidir / "real.yaml").write_text(f"provider: deepseek\napi_key: {secret}\n")
        auto = load_auto(self.ws)
        self.assertNotIn(secret, repr(auto.list_ai_profiles()))


@requires_tmux
class TestStartRun(unittest.TestCase):
    def setUp(self):
        self._ws = temp_workspace()
        self.ws = self._ws.__enter__()
        write_job(self.ws, "testpack", "job-a")
        self.auto = load_auto(self.ws)
        self.started = []

    def tearDown(self):
        for aid in self.started:
            self.auto.tmux_kill_session(self.auto.tmux_session_name(aid))
        self._ws.__exit__(None, None, None)

    def _wait_done(self, aid, timeout=30.0):
        deadline = time.time() + timeout
        while time.time() < deadline:
            r = self.auto.get_run(aid)
            if r and r["status"] != "running":
                return r
            time.sleep(0.2)
        return self.auto.get_run(aid)

    def test_launches_and_completes_with_audit_tagged_log(self):
        aid = self.auto.start_run("job", "job-a")
        self.started.append(aid)
        r = self._wait_done(aid)
        self.assertEqual(r["status"], "succeeded")
        self.assertEqual(r["rc"], 0)
        log = (self.ws / r["log_path"]).read_text()
        self.assertTrue(log.strip(), "log should not be empty")
        for line in log.splitlines():
            self.assertIn(aid, line, "every log line must carry the audit id")

    def test_refuses_duplicate_concurrent_run(self):
        write_job(self.ws, "testpack", "slow")
        (self.ws / "packs" / "testpack" / "jobs" / "misc" / "slow" / "main.sh").write_text(
            "#!/usr/bin/env bash\nsleep 20\n")
        auto = load_auto(self.ws)
        aid = auto.start_run("job", "slow")
        self.started.append(aid)
        with self.assertRaises(auto.RunRefused) as ctx:
            auto.start_run("job", "slow")
        self.assertEqual(ctx.exception.code, "already_running")
        auto.cancel_run(aid)

    def test_refuses_danger_action_without_confirm(self):
        with self.assertRaises(self.auto.RunRefused) as ctx:
            self.auto.start_run("command", "schedule-sync")
        self.assertEqual(ctx.exception.code, "confirm_required")

    def test_refuses_unknown_action(self):
        with self.assertRaises(self.auto.RunRefused) as ctx:
            self.auto.start_run("job", "no-such-job")
        self.assertEqual(ctx.exception.code, "unknown_action")

    def test_machine_pinned_job_can_still_be_launched(self):
        """Mirror of the catalog test: pinning must not refuse a manual run."""
        write_job(self.ws, "testpack", "pinned",
                  runs_on={"os": ["windows"], "machines": ["some-other-box"]})
        auto = load_auto(self.ws)
        aid = auto.start_run("job", "pinned")
        self.started.append(aid)
        self._wait_done(aid)

    def test_ai_profile_ignored_for_actions_that_cannot_take_one(self):
        aid = self.auto.start_run("command", "list")
        self.started.append(aid)
        self.assertIsNone(self.auto.get_run(aid)["ai_profile"])
        self._wait_done(aid)

    def test_cancel_marks_run_cancelled_and_kills_session(self):
        write_job(self.ws, "testpack", "slow2")
        (self.ws / "packs" / "testpack" / "jobs" / "misc" / "slow2" / "main.sh").write_text(
            "#!/usr/bin/env bash\nsleep 30\n")
        auto = load_auto(self.ws)
        aid = auto.start_run("job", "slow2")
        self.started.append(aid)
        time.sleep(1.0)
        auto.cancel_run(aid)
        self.assertEqual(auto.get_run(aid)["status"], "cancelled")
        self.assertFalse(auto.tmux_has_session(auto.tmux_session_name(aid)))
        with self.assertRaises(auto.RunRefused) as ctx:
            auto.cancel_run(aid)
        self.assertEqual(ctx.exception.code, "not_running")


@requires_tmux
class TestOrchestrationRun(unittest.TestCase):
    """End-to-end step progress, without touching the real gmail pipeline."""

    def setUp(self):
        self._ws = temp_workspace()
        self.ws = self._ws.__enter__()
        write_job(self.ws, "testpack", "step-one")
        write_job(self.ws, "testpack", "step-two")
        (self.ws / "orchestrator" / "pipe.yaml").write_text(
            "name: pipe\ndescription: two-step test pipeline\n"
            "steps:\n  - job: step-one\n  - job: step-two\n")
        self.auto = load_auto(self.ws)
        self.started = []

    def tearDown(self):
        for aid in self.started:
            self.auto.tmux_kill_session(self.auto.tmux_session_name(aid))
        self._ws.__exit__(None, None, None)

    def test_orchestration_records_step_progress(self):
        aid = self.auto.start_run("orchestration", "pipe")
        self.started.append(aid)
        deadline = time.time() + 40
        while time.time() < deadline:
            r = self.auto.get_run(aid)
            if r["status"] != "running":
                break
            time.sleep(0.3)
        self.assertEqual(r["status"], "succeeded", msg=(self.ws / r["log_path"]).read_text())

        steps = self.auto.get_run_steps(aid)
        self.assertEqual(len(steps), 2, f"expected 2 steps, got {steps}")
        self.assertEqual([s["job"] for s in steps], ["step-one", "step-two"])
        self.assertTrue(all(s["status"] == "succeeded" for s in steps))

        # Markers are control data, not output: they must not reach the log.
        log = (self.ws / r["log_path"]).read_text()
        self.assertNotIn("##AUTO-STEP##", log)


@requires_tmux
class TestParallelRunAudit(unittest.TestCase):
    """spec SC-004: with several runs in flight, every log line must be
    attributable to exactly one run."""

    def setUp(self):
        self._ws = temp_workspace()
        self.ws = self._ws.__enter__()
        for n in ("alpha", "beta", "gamma"):
            jd = write_job(self.ws, "testpack", n)
            # Chatty and staggered, so the three runs genuinely interleave.
            (jd / "main.sh").write_text(
                f"#!/usr/bin/env bash\nfor i in 1 2 3 4 5; do echo '{n} line '$i; sleep 0.2; done\n")
        self.auto = load_auto(self.ws)
        self.started = []

    def tearDown(self):
        for aid in self.started:
            self.auto.tmux_kill_session(self.auto.tmux_session_name(aid))
        self._ws.__exit__(None, None, None)

    def test_three_concurrent_runs_stay_attributable(self):
        ids = {n: self.auto.start_run("job", n) for n in ("alpha", "beta", "gamma")}
        self.started.extend(ids.values())

        deadline = time.time() + 60
        while time.time() < deadline:
            if all(self.auto.get_run(a)["status"] != "running" for a in ids.values()):
                break
            time.sleep(0.3)

        for name, aid in ids.items():
            r = self.auto.get_run(aid)
            self.assertEqual(r["status"], "succeeded", f"{name} did not succeed")
            lines = [l for l in (self.ws / r["log_path"]).read_text().splitlines() if l.strip()]
            self.assertTrue(lines)
            for line in lines:
                self.assertIn(aid, line, "every line must carry its own audit id")
                for other in ids.values():
                    if other != aid:
                        self.assertNotIn(other, line, "no line may carry a second run's id")
            self.assertTrue(any(f"{name} line 5" in l for l in lines),
                            f"{name}'s own output missing from its log")

        self.assertEqual(len({*ids.values()}), 3, "audit ids must be distinct")


class TestStepMarkers(unittest.TestCase):
    """Terminal runs must stay byte-for-byte identical (research §10)."""

    def setUp(self):
        self._ws = temp_workspace()
        self.ws = self._ws.__enter__()
        self.auto = load_auto(self.ws)

    def tearDown(self):
        os.environ.pop("AUTO_RUN_AUDIT_ID", None)
        self._ws.__exit__(None, None, None)

    def _capture(self):
        import io, contextlib as cl
        buf = io.StringIO()
        with cl.redirect_stdout(buf):
            self.auto._emit_step_marker("job-a", "start")
            self.auto._emit_step_marker("job-a", "end", 0)
        return buf.getvalue()

    def test_no_marker_without_audit_id(self):
        os.environ.pop("AUTO_RUN_AUDIT_ID", None)
        self.assertEqual(self._capture(), "",
                         "terminal runs must emit no step markers at all")

    def test_marker_emitted_under_dashboard_run(self):
        os.environ["AUTO_RUN_AUDIT_ID"] = "r-20260726-000000-abc123"
        out = self._capture()
        self.assertIn("##AUTO-STEP##", out)
        self.assertIn("job=job-a", out)
        self.assertIn("event=start", out)
        self.assertIn("rc=0", out)

    def test_marker_parsing_roundtrip(self):
        aid = "r-20260726-000000-parse1"
        self.auto.create_run(aid, "orchestration", "pipe", "", "s", f"logs/runs/{aid}.log")
        self.auto._handle_step_marker(aid, "##AUTO-STEP## idx=0 job=job-a event=start")
        self.auto._handle_step_marker(aid, "##AUTO-STEP## idx=0 job=job-a event=end rc=0")
        steps = self.auto.get_run_steps(aid)
        self.assertEqual(len(steps), 1)
        self.assertEqual(steps[0]["status"], "succeeded")

    def test_malformed_marker_never_raises(self):
        self.auto._handle_step_marker("r-x", "##AUTO-STEP## garbage")


if __name__ == "__main__":
    unittest.main()
