# ADR 0019 — `data_files:`: a pack's own produced data lives in data/, symlinked in, never inside packs/

**Status:** accepted — 2026-08-09

## Context

ADR 0011 decided the expenses pack's event registry (`config/events.json`)
and assignment ledger (`state.json`) would live **committed directly inside
`packs/expenses/`** — the registry because it's a versioned definitions
file, the ledger because it's local produced data "written directly into
the pack's own workdir." The wallet pack (ADR 0009) followed the same
pattern for its own `state.json` dedupe ledger. At the time this seemed
reasonable: `packs/expenses/` and `packs/wallet/` aren't git submodules
(unlike `packs/gmail/`) — they're plain directories inside the main
monorepo, so there was no cross-repo boundary making this look wrong.

Building the write-sandbox (ADR 0018) surfaced why it *is* wrong regardless
of repo boundaries: `packs/` is supposed to be the code a pack ships — read
this to know what a job does. Mixing in files a job also *writes* means
`packs/` stops being a clean, portable, shareable unit (the whole premise
of ADR 0002's parent/pack split and ADR 0004's shared-pack contribution
flow) and starts accumulating machine-specific runtime state. It's also
exactly the shape of the credentials-drift bug ADR 0007/0018 exist to
prevent, just for data instead of secrets: nothing stopped
`packs/expenses/state.json` from silently going stale, being lost on a pack
reset, or ending up committed by accident (it very nearly was — `.gitignore`
had to specifically exclude it).

Sumit's direction, stated directly: no pack registry should reside inside
the pack; packs are read-only as far as `auto` is concerned; all data and
cache belong in directories rooted at the workspace (`auto` itself), i.e.
`data/` (and `config/` for secrets) — never inside `packs/`.

## Decision

1. **New `data_files:` key in `config.sample.yaml`**, parallel to the
   existing `files:` (ADR 0007) but for produced data instead of secrets.
   Same mechanism, split by what the two workspace directories mean:
   `config/<pack>/` holds injected secrets/overrides, `data/<pack>/` holds
   whatever the pack itself produces and needs to persist across runs.

2. **`_link_pack_data_files()`**, mirroring `_link_pack_files()`: for each
   declared name, symlink it from `data/<pack>/<basename>` into the job's
   workdir at `<pack>/<name>`, creating the link's parent directory first
   (so a nested name like `config/events.json` still resolves where the
   app expects it — see decision 3). Wired into `execute_job()` right next
   to the existing secrets-linking call, so it runs on every `auto run` /
   orchestrator step, self-healing the same way secrets do.

3. **The declared name is the path relative to the pack's own workdir (what
   the app is hardcoded/flagged to open); the target under `data/<pack>/`
   is flattened to the basename.** `expenses/main.go`'s `--events` flag
   defaults to `config/events.json` (relative to its cwd, `packs/expenses/`)
   — that nesting is the pack's own internal layout convention and has no
   meaning outside it. So the link is created at
   `packs/expenses/config/events.json` (satisfying the app), pointing at
   `data/expenses/events.json` (flat — no reason to nest inside `data/`
   just because the pack nests it internally).

4. **No app code changes required.** `expenses/main.go` and `wallet/main.go`
   still open `config/events.json` / `state.json` at their usual relative
   paths — those paths are now symlinks. This is the same non-invasive
   property ADR 0007's secrets symlinking already has: the app doesn't know
   or care that the byte it's opening lives somewhere else.

5. **Physical relocation:** `packs/expenses/config/events.json` →
   `data/expenses/events.json` (still versioned — commit it); Path
   `packs/expenses/state.json` → `data/expenses/state.json` (still
   git-ignored); `packs/wallet/state.json` → `data/wallet/state.json`
   (still git-ignored). Each new `data/<pack>/` directory gets its own
   `.gitignore` (same convention `data/gmail/.gitignore` already
   established) rather than leaning on a global `data/config/` vs
   `data/state/` split — keeps the versioned/local distinction local to
   the pack that owns it, visible in one place.

6. **`auto doctor` gained the data-side twin of its existing secrets check:**
   every declared `data_files:` entry must be a symlink into
   `data/<pack>/`, never a real file sitting in the pack directory — same
   drift this ADR exists to prevent, checked on every `auto doctor` run,
   not just assumed.

7. **The write-sandbox (ADR 0018 Amendment 2) is what actually enforces
   this isn't optional.** Without the sandbox, a bug in a pack's own code
   (or a future edit that hardcodes a different path) could quietly
   recreate the exact problem this ADR fixes. With it, any write to
   `packs/<pack>/` that isn't going through one of these symlinks fails
   outright — `auto doctor` catches an already-drifted state, the sandbox
   prevents a new one from forming silently.

## Consequences

- `packs/expenses/` and `packs/wallet/` now contain only code + symlinks —
  no real data files. Verified: reading through both packs' `state.json`
  and `packs/expenses/config/events.json` after the move returns
  byte-identical content to before (477 expense assignments, 371
  wallet-pushed records, 4 events — nothing lost in the relocation).
- A probe against the real `expenses-update-event` job confirms the
  corrected model: writes through the two declared `data_files` symlinks
  succeed; a write to a *new*, undeclared file directly in
  `packs/expenses/` is denied by the sandbox.
- `packs/expenses/` and `packs/wallet/` are now meaningfully closer to
  shareable, portable units (ADR 0002/0004's premise) — nothing
  machine-specific has to be excluded or gitignored away, because nothing
  machine-specific is written there in the first place.
- Amends ADR 0011 decision 3 (event registry / ledger location) and ADR
  0009's equivalent decision for wallet's dedupe ledger — both now say
  "under data/<pack>/, symlinked in" rather than "committed/written inside
  the pack." Extends ADR 0007's mechanism (declare → symlink → write
  through) from secrets to produced data generally. Complements ADR 0018
  (the sandbox that makes this enforced, not just documented) and ADR 0005
  (versioned-vs-local provenance split, now applied via each `data/<pack>/`
  directory's own `.gitignore` rather than a single global split).
