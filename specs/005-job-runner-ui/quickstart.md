# Quickstart: One-Click Job Runner UI

## Prerequisites

- `tmux` installed (**hard requirement, no fallback** — FR-017). Verify: `tmux -V`. Install: `brew install tmux`.
- Python 3 with PyYAML (already required by `auto`).
- Run from the workspace root.

## Run it

```sh
./auto serve          # or: make serve   (PORT=… to override 4321)
```

Open `http://127.0.0.1:4321`.

## Validation scenarios

Each maps to the user stories and success criteria in `spec.md`.

1. **Actions are discoverable and described** (US1 / SC-002): the Actions section lists all 11 jobs, the `gmail-wallet-sync` orchestration, and the command buttons — each with its description. Nothing requires knowing a CLI name.
2. **One-click launch** (US1 / SC-001): click a fast job (`hello-report`); a run appears in Runs within a second with status `running`, and reaches `succeeded`.
3. **Live logs** (US2 / SC-003): launch a longer job, open its run, and confirm output appears progressively without reloading the page.
4. **AI profile dropdown** (US3): `deepseek` is offered for jobs; the `*.example.yaml` templates are **not** (FR-008). Orchestrations show **no** dropdown (FR-010). Launch `gmail-categorize` with `deepseek` selected and confirm the run's log shows the profile being applied and the run record names it.
5. **Audit IDs under parallelism** (US4 / SC-004): start three runs at once, then
   `cat logs/runs/*.log | sort` — every line carries exactly one audit ID and no line is ambiguous.
6. **No credential leakage** (SC-007): `grep -ri "$(grep api_key config/ai/deepseek.yaml | cut -d: -f2- | tr -d ' \"')" logs/` returns nothing; the key never appears in the page source either.
7. **Non-interactive guarantee** (FR-015 / SC-009): launch `gmail-categorize` — it must complete rather than hang, with the interactive suggest/rule-capture prompts skipped exactly as on a cron run.
8. **Duplicate guard** (FR-006): launch the same job twice quickly — the second attempt is refused with `409` naming the in-flight run.
9. **Confirmation gate** (FR-005 / SC-008): the machine-altering buttons (`Install scheduled tasks`, `Bootstrap workspace`) are visually distinguished and require confirming. Posting without `confirm: true` returns `412`.
10. **Cancellation** (FR-014): start a long run, cancel it, confirm status becomes `cancelled` and the tmux session is gone (`tmux ls`).
11. **Session hygiene** (FR-013 / SC-006): after runs finish, `tmux ls` shows no leftover `auto-*` sessions.
12. **Reconciliation** (FR-016 / SC-005): start a run, `tmux kill-session -t auto-<audit_id>` behind the dashboard's back, reload Runs — the run must move off `running` to a definite status rather than hanging forever.
13. **Missing tmux** (FR-017): temporarily make tmux unreachable (`PATH= ./auto serve`) and confirm the dashboard reports it clearly instead of offering buttons that silently fail.
14. **GET is side-effect free** (FR-029): reloading any page or replaying a `GET` never starts a run.

## Tests

```sh
python3 -m unittest discover -s framework/tools -p 'test_*.py' -v
```

## Contracts

Full request/response shapes: `contracts/dashboard-api.md`.

---

# Quickstart — Revision 2 (2026-07-27)

## Prerequisites (changed)

`tmux` as before, **plus** the two directories must now be supplied explicitly. Either export them once:

```sh
export AUTO_DATA_DIR="$PWD/data"
export AUTO_CONFIG_DIR="$PWD/config"
./auto serve
```

…or pass them per invocation:

```sh
./auto serve --data-dir ./data --config-dir ./config
./auto run hello-report --data-dir ./data --config-dir ./config
```

> **Upgrade note**: bare `./auto run …` no longer works, by design. Re-run `auto schedule sync` after upgrading so the generated cron/launchd entries carry the directories (research §16) — otherwise every scheduled job will start failing.

## Validation scenarios (rev. 2 adds 15–22)

Scenarios 1–14 from rev. 1 still apply, with the directory flags added to each invocation.

15. **Missing directory is refused** (US5 / SC-010): `env -u AUTO_DATA_DIR -u AUTO_CONFIG_DIR ./auto run hello-report` → non-zero exit, message naming `--data-dir` and `AUTO_DATA_DIR`. Confirm nothing was written: `git status` clean, no new row in `ui_runs`.
16. **Nonexistent path is refused** (SC-010): `--data-dir /nope/nothing` → refused, message names the path.
17. **Structurally wrong path is refused** (SC-010): point `--data-dir` at a non-empty but wrong directory (e.g. `$HOME`) → refused, naming the expected `state/`+`config/` children. This is the case a bare non-empty check would have let through.
18. **Env var is a complete substitute** (US5 scenario 2): `AUTO_DATA_DIR=./data AUTO_CONFIG_DIR=./config ./auto run hello-report` → succeeds with no flags.
19. **Flag beats env** (US5 scenario 4 / FR-016): set both to different valid directories, pass the flag, and confirm the run record's `data_dir` shows the flag's value.
20. **Inspection commands still work without dirs** (SC-012): with both unset, `./auto list`, `./auto packs`, `./auto doctor`, `./auto catalog` all still succeed — this is what lets you diagnose the misconfiguration.
21. **Dashboard refuses to start** (FR-020): with both unset, `./auto serve` exits non-zero with the same message and never binds the port (`curl` to it fails).
22. **Session-wide AI profile, with per-step override** (US3 / SC-013):
    - `PUT /api/session/ai-profile {"ai_profile":"deepseek"}` → `200`.
    - Launch a job → its run record shows `ai_profile: deepseek`, and the log header names it.
    - Launch `gmail-wallet-sync` (whose steps declare `ai: deepseek` themselves) → the steps use their own profile; the run still records the session default. Confirm no step failed for credentials.
    - `PUT` with `{"ai_profile": null}` → subsequent runs record `null` and behave as before the control existed.
    - Confirm there is exactly **one** profile control in the page (`grep -c '<select' ` on the served HTML returns 1).
23. **Selected profile deleted before launch** (FR-012): select a profile, move `config/ai/<name>.yaml` aside, then launch → `422 unknown_profile`, no run started.
24. **Run records carry the directories** (SC-014): after any run, `GET /api/runs/<id>` shows the `data_dir`/`config_dir` it operated on.

## Tests

```sh
python3 -m unittest discover -s framework/tools -p 'test_*.py' -v
```
