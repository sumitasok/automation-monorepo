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
