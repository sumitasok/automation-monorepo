"""Regression tests for the three bugs that let a broken sync look healthy.

Run: python3 -m pytest packs/expense-domain/sources/wallet/jobs/wallet-sync-unified/tests/

The failure they reproduce together: an AI-suggested format file was written
with an unquoted `subject: View:`, which is invalid YAML. load_formats() aborted
on it, so extract-engine.py exited 1 on every run; sync.py raised
CalledProcessError with the engine's stderr discarded; and because state was
only saved on the success path, last-sync.json kept reporting the last
successful run for the 30 runs that followed.
"""
import importlib.util
import json
import os
import pathlib
import sys

import pytest
import yaml

JOB_DIR = pathlib.Path(__file__).resolve().parent.parent


def _load(name, filename):
    """Import a hyphenated/standalone script as a module."""
    spec = importlib.util.spec_from_file_location(name, JOB_DIR / filename)
    mod = importlib.util.module_from_spec(spec)
    sys.modules[name] = mod
    spec.loader.exec_module(mod)
    return mod


@pytest.fixture
def engine():
    return _load("extract_engine_under_test", "extract-engine.py")


# ---------------------------------------------------------------------------
# 1. The generator must never write YAML it cannot read back
# ---------------------------------------------------------------------------

# The first is the exact subject that broke production: "View:" is a bare
# scalar containing a colon, which YAML rejects.
HOSTILE_SUBJECTS = [
    "View: Your HDFC AutoPay cancelled",
    "#hash leads the line",
    '"quoted" subject',
    "*star [bracket] {brace}",
    "- dash leads",
    "%percent leads",
    "&anchor *alias",
    "",
]


@pytest.mark.parametrize("subject", HOSTILE_SUBJECTS)
def test_generated_format_file_is_always_loadable(engine, tmp_path, subject):
    engine.FORMATS_DIR = tmp_path
    path = engine.create_format_file_from_suggestion(
        {
            "success": True,
            "format_name": "generated",
            "body_pattern": r"Rs\.(?P<amount>[\d,.]+)\s+(?P<merchant>.+?)\s+(?P<date>\d{2}-\d{2}-\d{2})",
            "reasoning": "a reason with: a colon and a #hash",
        },
        "HDFC Bank InstaAlerts <alerts@hdfcbank.bank.in>",
        subject,
    )
    assert path, f"refused to write a format for subject {subject!r}"

    docs = [d for d in yaml.safe_load_all(pathlib.Path(path).read_text()) if d]
    assert len(docs) == 1
    engine._validate(docs[0], "generated.yaml")

    # And the real loader must accept the directory it was written into.
    assert len(engine.load_formats(tmp_path, strict=True)) == 1


def test_generator_refuses_to_write_an_invalid_format(engine, tmp_path):
    """A pattern that will not compile must not reach disk."""
    engine.FORMATS_DIR = tmp_path
    path = engine.create_format_file_from_suggestion(
        {"success": True, "format_name": "bad-regex", "body_pattern": "(?P<amount>[unclosed"},
        "x@y.com",
        "Alert",
    )
    assert path is None
    assert list(tmp_path.glob("*.yaml")) == []


# ---------------------------------------------------------------------------
# 2. One bad format file must not stop every run
# ---------------------------------------------------------------------------

GOOD_FORMAT = """
name: good
source: gmail
priority: 10
match:
  sender: alerts@example\\.com
  subject: Alert
action: extract
fields:
  body: Rs\\.(?P<amount>[\\d.]+)
"""


def test_load_formats_skips_the_broken_file_and_keeps_the_rest(engine, tmp_path, capsys):
    (tmp_path / "a-good.yaml").write_text(GOOD_FORMAT)
    (tmp_path / "b-broken.yaml").write_text("name: x\nmatch:\n  subject: View:\naction: extract\n")

    fmts = engine.load_formats(tmp_path)

    assert [f["name"] for f in fmts] == ["good"]
    warning = capsys.readouterr().err
    assert "b-broken.yaml" in warning and "skipped 1" in warning


def test_load_formats_skips_a_file_that_parses_but_is_invalid(engine, tmp_path):
    (tmp_path / "a-good.yaml").write_text(GOOD_FORMAT)
    # Valid YAML, but 'extract' without 'fields' — caught by _validate.
    (tmp_path / "b-invalid.yaml").write_text("name: x\nmatch:\n  subject: Alert\naction: extract\n")
    # Valid YAML, but the regex will not compile.
    (tmp_path / "c-badregex.yaml").write_text(
        "name: y\nmatch:\n  subject: '(?P<unclosed'\naction: skip\n")

    fmts = engine.load_formats(tmp_path)
    assert [f["name"] for f in fmts] == ["good"]


def test_load_formats_is_strict_on_demand(engine, tmp_path):
    (tmp_path / "a-good.yaml").write_text(GOOD_FORMAT)
    (tmp_path / "b-broken.yaml").write_text("subject: View:\n")

    with pytest.raises(ValueError, match="unusable format file"):
        engine.load_formats(tmp_path, strict=True)


def test_load_formats_raises_when_nothing_usable_remains(engine, tmp_path):
    """An empty registry would match nothing and look like a clean run."""
    (tmp_path / "broken.yaml").write_text("subject: View:\n")

    with pytest.raises(ValueError, match="no usable format files"):
        engine.load_formats(tmp_path)


def test_load_formats_drops_a_whole_file_when_one_of_its_documents_is_bad(engine, tmp_path):
    """Half-applying a multi-document file would silently lose formats."""
    (tmp_path / "a-good.yaml").write_text(GOOD_FORMAT)
    (tmp_path / "b-multi.yaml").write_text(
        GOOD_FORMAT.replace("name: good", "name: sibling") + "\n---\nname: broken\naction: extract\n")

    fmts = engine.load_formats(tmp_path)
    assert [f["name"] for f in fmts] == ["good"], "the good sibling must not be applied alone"


# ---------------------------------------------------------------------------
# 3. last-sync.json must record failures
# ---------------------------------------------------------------------------

@pytest.fixture
def sync(tmp_path, monkeypatch):
    """Import sync.py with its paths pointed at a temp directory."""
    monkeypatch.setenv("CONFIG_PATH", str(tmp_path / "config"))
    monkeypatch.setenv("AUTO_DATA_DIR", str(tmp_path / "data"))
    monkeypatch.delenv("SA_VAULT", raising=False)
    (tmp_path / "data" / "logs").mkdir(parents=True)
    return _load("sync_under_test", "sync.py")


def _state(sync):
    return json.loads(sync.LAST_SYNC.read_text())


def _stats(**over):
    base = {"fetched": 0, "pushed": 0, "skipped": 0, "failed": 0, "unrouted": 0, "deduped": 0}
    base.update(over)
    return base


def test_state_records_a_failure_instead_of_keeping_the_last_success(sync, monkeypatch):
    sync.LAST_SYNC.write_text(json.dumps({
        "last_email_timestamp": "2026-09-04T00:00:00Z",
        "last_run_status": "success",
    }))

    def boom(args, state, log):
        raise RuntimeError("extract-engine.py failed (exit 1):\nScannerError: mapping values")

    monkeypatch.setattr(sync, "_run_sync", boom)
    monkeypatch.setattr(sys, "argv", ["sync.py"])

    with pytest.raises(RuntimeError):
        sync.main()

    s = _state(sync)
    assert s["last_run_status"] == "failed"
    assert "extract-engine.py failed" in s["last_run_error"]
    assert s["last_run_start"] and s["last_run_end"]
    # The cursor must not move: those emails were never processed.
    assert s["last_email_timestamp"] == "2026-09-04T00:00:00Z"


def test_a_failed_run_still_writes_its_own_log_file(sync, monkeypatch):
    monkeypatch.setattr(sync, "_run_sync",
                        lambda a, s, l: (_ for _ in ()).throw(RuntimeError("boom")))
    monkeypatch.setattr(sys, "argv", ["sync.py"])

    with pytest.raises(RuntimeError):
        sync.main()

    logs = list(sync.LOG_DIR.glob("sync-*.log"))
    assert logs, "a crashed run wrote no log of its own"
    assert "FAILED" in logs[0].read_text()


def test_sys_exit_inside_the_run_is_also_recorded(sync, monkeypatch):
    """Auth failures call sys.exit(), which is not an Exception."""
    monkeypatch.setattr(sync, "_run_sync",
                        lambda a, s, l: sys.exit("OAuth credentials not found"))
    monkeypatch.setattr(sys, "argv", ["sync.py"])

    with pytest.raises(SystemExit):
        sync.main()

    assert _state(sync)["last_run_status"] == "failed"


def test_state_records_success_and_advances_the_cursor(sync, monkeypatch):
    sync.LAST_SYNC.write_text(json.dumps({"last_email_timestamp": "2026-09-04T00:00:00Z"}))
    monkeypatch.setattr(sync, "_run_sync",
                        lambda a, s, l: (2, 1, "2026-09-06T00:00:00Z", "2026-09-06T00:00:00Z",
                                         {"fetched": 9, "pushed": 2, "skipped": 1,
                                          "failed": 4, "unrouted": 2, "deduped": 0}))
    monkeypatch.setattr(sys, "argv", ["sync.py"])

    sync.main()

    s = _state(sync)
    assert s["last_run_status"] == "success"
    assert s["last_email_timestamp"] == "2026-09-06T00:00:00Z"
    assert s["last_run_summary"]["created"] == 3
    # A completed run that could not extract 4 documents must say so.
    assert s["last_run_summary"]["failed"] == 4
    assert s["last_run_summary"]["unrouted"] == 2
    assert s["last_run_summary"]["total_fetched"] == 9
    assert s["records_pushed_total"] == 3
    assert "last_run_error" not in s


def test_a_run_in_progress_is_visible_before_it_finishes(sync, monkeypatch):
    """A hard kill leaves 'running', not the previous run's 'success'."""
    seen = {}

    def capture(args, state, log):
        seen["status"] = _state(sync)["last_run_status"]
        return 0, 0, "", "", _stats()

    monkeypatch.setattr(sync, "_run_sync", capture)
    monkeypatch.setattr(sys, "argv", ["sync.py"])
    sync.main()

    assert seen["status"] == "running"


def test_dry_run_never_writes_state(sync, monkeypatch):
    monkeypatch.setattr(sync, "_run_sync", lambda a, s, l: (1, 1, "ts", "ts", _stats()))
    monkeypatch.setattr(sys, "argv", ["sync.py", "--dry-run"])

    sync.main()

    assert not sync.LAST_SYNC.exists()


# ---------------------------------------------------------------------------
# 4. The engine's own error must reach the log
# ---------------------------------------------------------------------------

def test_run_engine_surfaces_the_engine_stderr(sync, monkeypatch):
    """The old code raised CalledProcessError with stderr thrown away."""
    class Result:
        returncode = 1
        stdout = ""
        stderr = "yaml.scanner.ScannerError: mapping values are not allowed here"

    monkeypatch.setattr(sync.subprocess, "run", lambda *a, **k: Result())

    with pytest.raises(RuntimeError, match="ScannerError"):
        sync.run_engine([{"id": "x", "body": "b"}], lambda m: None)


def test_run_engine_logs_warnings_from_a_successful_engine_run(sync, monkeypatch):
    class Result:
        returncode = 0
        stdout = '{"matched": false}\n'
        stderr = "WARNING: skipped 1 unusable format file(s)"

    monkeypatch.setattr(sync.subprocess, "run", lambda *a, **k: Result())
    lines = []

    out = sync.run_engine([{"id": "x", "body": "b"}], lines.append)

    assert out == [{"matched": False}]
    assert any("skipped 1 unusable format file" in line for line in lines)


# ---------------------------------------------------------------------------
# 5. Dedup against real Wallet API record shapes
# ---------------------------------------------------------------------------

def test_record_amount_reads_the_api_amount_object(sync):
    """GET /records returns amount as {"value": ..., "currencyCode": ...}.

    Treating it as a scalar raised `TypeError: unsupported operand type(s)
    for +: 'dict' and 'float'` and aborted the run before anything was written.
    """
    assert sync.record_amount({"amount": {"value": -868.76, "currencyCode": "INR"}}) == -868.76
    assert sync.record_amount({"amount": -500.0}) == -500.0      # legacy scalar shape
    assert sync.record_amount({}) == 0.0
    assert sync.record_amount({"amount": None}) == 0.0
    assert sync.record_amount({"amount": {"currencyCode": "INR"}}) == 0.0


def test_fuzzy_duplicate_survives_the_amount_object(sync):
    existing = [{"amount": {"value": -2500.0, "currencyCode": "INR"},
                 "recordDate": "2026-09-05T12:00:00.000Z",
                 "counterParty": "NASAREEN", "note": "manual entry"}]

    assert sync.fuzzy_duplicate(2500.0, "6307781870@ybl (NASAREEN)", "2026-09-05", existing)


def test_fuzzy_duplicate_matches_a_upi_handle_against_a_plain_name(sync):
    """The alert says "6307781870@ybl (NASAREEN)"; the existing record says
    "NASAREEN". The old first-six-characters test compared "630778" and missed
    it, so the run would have created a second copy."""
    existing = [{"amount": {"value": -2500.0}, "recordDate": "2026-09-05",
                 "counterParty": "NASAREEN", "note": ""}]

    assert sync.fuzzy_duplicate(2500.0, "6307781870@ybl (NASAREEN)", "2026-09-05", existing)


def test_fuzzy_duplicate_ignores_records_this_pipeline_wrote(sync):
    """gm-tagged records are already_in_wallet()'s job, not fuzzy matching."""
    existing = [{"amount": {"value": -2500.0}, "recordDate": "2026-09-05",
                 "counterParty": "NASAREEN", "note": "NASAREEN | gm:abc123"}]

    assert not sync.fuzzy_duplicate(2500.0, "NASAREEN", "2026-09-05", existing)


def test_fuzzy_duplicate_does_not_match_a_different_merchant(sync):
    existing = [{"amount": {"value": -2500.0}, "recordDate": "2026-09-05",
                 "counterParty": "Blinkit", "note": ""}]

    assert not sync.fuzzy_duplicate(2500.0, "Zomato", "2026-09-05", existing)


def test_fuzzy_duplicate_does_not_match_a_different_day_or_amount(sync):
    base = {"counterParty": "NASAREEN", "note": ""}
    assert not sync.fuzzy_duplicate(
        2500.0, "NASAREEN", "2026-09-05",
        [{**base, "amount": {"value": -2500.0}, "recordDate": "2026-09-04"}])
    assert not sync.fuzzy_duplicate(
        2500.0, "NASAREEN", "2026-09-05",
        [{**base, "amount": {"value": -2400.0}, "recordDate": "2026-09-05"}])


def test_already_in_wallet_finds_the_gm_tag(sync):
    existing = [{"note": "HUNGERBOX | via HDFC CC | gm:1a06bb742c271f68 | proc:x:create"}]
    assert sync.already_in_wallet("1a06bb742c271f68", existing)
    assert not sync.already_in_wallet("someotherid", existing)
    assert not sync.already_in_wallet("1a06bb742c271f68", [{"note": None}])


def test_a_failed_run_does_not_leave_the_previous_summary_behind(sync, monkeypatch):
    """A stale summary under a "failed" status reads as this run's numbers."""
    sync.LAST_SYNC.write_text(json.dumps({
        "last_run_status": "success",
        "last_run_summary": {"total_fetched": 19, "created": 5, "skipped": 3, "failed": 0},
    }))
    monkeypatch.setattr(sync, "_run_sync",
                        lambda a, s, l: (_ for _ in ()).throw(RuntimeError("boom")))
    monkeypatch.setattr(sys, "argv", ["sync.py"])

    with pytest.raises(RuntimeError):
        sync.main()

    assert _state(sync)["last_run_summary"] is None
