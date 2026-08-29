# ADR 0018 — Write-sandbox for job execution: jobs may only write to config/+data/

**Status:** accepted — 2026-08-09

## Context

`packs/gmail/token.json` and `packs/gmail/credentials.json` were found to be
real files sitting directly in the pack directory instead of symlinks into
`config/gmail/` — the contract ADR 0007 establishes (`config.sample.yaml`
declares `files:`, `auto run` symlinks them from the git-ignored
`config/<pack>/` into the job's workdir so writes persist there). The
`_link_pack_files()` linker has a safety guard that refuses to clobber an
existing real file, so once those files drifted into the pack directory
(most likely predating the symlink convention), every subsequent `auto run`
silently left them there — `config/gmail/` never actually held a canonical
`token.json`, and a refreshed OAuth token was never surviving in the place
the design assumed it would.

Nothing in `auto` enforced the ADR 0007 contract; it only *implemented* it
when starting from a clean state. Sumit asked for a check that guarantees
`auto` never reads/writes anywhere other than `config/` and `data/`.

Two things had to be scoped down from that literal statement, worked out via
clarifying questions before writing any code:

1. **`auto`'s own commands legitimately write elsewhere.** `auto catalog`
   writes `CATALOG.md` at the workspace root, `auto log` appends to
   `docs/worklog/`, `auto new` scaffolds a job folder under
   `packs/<pack>/jobs/`, and `auto schedule sync` writes
   `schedules/generated/` plus OS crontab/launchd entries. These are
   deliberate, ADR-established behaviors (0001, 0004), not the failure mode
   that prompted this — the failure mode was a *job's own process* (the
   `go run .` invocation `auto run` execs) writing secrets somewhere other
   than where the config contract says they live. Decision: sandbox only the
   child process `execute_job()` spawns for `auto run` / `auto orchestrate`
   steps; leave `auto`'s own meta-commands unwrapped and trusted.

2. **Reads can't be confined the same way.** A job's source code lives in
   `packs/<pack>/` and has to be *readable* to run at all — `go run .`
   reads `.go` files, `go.mod`, `go.sum`; the gmail pack reads `filters/`.
   Confining reads to `config/+data/` would mean copying each job's own code
   there before running it, which is a different and much larger redesign.
   Decision: writes only. Reads, network, and process execution stay
   unrestricted — only what a job can *persist to disk*, and *where*, is
   constrained.

## Decision

1. **New write-sandbox wraps the child process in `execute_job()`** — the
   function `cmd_run` and `cmd_orchestrate` both call to actually exec a
   job's `entrypoint`/`exec` command (see ADR-0001-era design note in
   `execute_job`'s own docstring: the two share this function so a step
   behaves byte-for-byte like `auto run`). Because both call sites share one
   function, both get the sandbox for free — no separate wiring for
   orchestrator steps.

2. **Allow-listed write roots: `config/` and `data/` at the workspace root,
   plus narrow, per-language toolchain-cache carve-outs** (`go env
   GOCACHE`/`GOMODCACHE`/`GOPATH` when the job's `language` is `go`; npm/pip
   cache dirs for `node`/`python`, unused by any job today but kept for
   parity since the framework advertises those runners). These caches are
   compiler/interpreter scratch space, not application data — without a
   writable `GOCACHE`, `go run .` doesn't run at all. Ephemeral OS temp dirs
   (`/tmp`, `/private/var/folders` on macOS) get the same narrow-carve-out
   treatment for the same reason.

3. **Per-OS enforcement, honestly non-uniform:**
   - **macOS — `sandbox-exec`** with a generated Seatbelt profile:
     `(allow default)` then `(deny file-write* (subpath "/"))` then
     `(allow file-write* (subpath <each allow-listed root>) ...)`. This is
     the primary, actually-used target (Sumit's machine) but the mechanism
     itself can't be exercised from the dev sandbox this was built in
     (Linux) — see `auto sandbox-check` below.
   - **Linux — `bwrap`** (bubblewrap) if installed: `--ro-bind / /` (reads
     everywhere) with `--bind <root> <root>` re-mounting each allow-listed
     root writable on top, plus fresh `--dev`/`--proc`/`--tmpfs /tmp`. Built
     and verified end-to-end in the dev sandbox (see Consequences).
   - **Windows / no backend found** — runs **unconfined**, and says so
     loudly on every single job run (`(sandbox) UNCONFINED — <reason>`) so
     an unenforced guarantee is never silent.

4. **Escape hatch: `auto run <id> --no-sandbox`** (or `AUTO_SANDBOX=off`).
   Extracted from argv the same way `--ai` is (`_extract_ai_flag` /
   `_extract_no_sandbox_flag`, ADR 0015's fix for `nargs='*'` positionals
   fighting named options in argparse), so it composes with `--ai` and a
   job's own passthrough args. Using it prints a loud warning — it's meant
   for a debugging session, not routine use.

5. **`auto sandbox-check`** — a self-test subcommand, because the actual
   enforcement mechanism is platform-specific and this was developed on a
   different OS (Linux) than where it needs to hold (macOS). It runs five
   probes through the same wrapping code path `execute_job` uses: write
   inside `data/` (must succeed), write inside `config/` (must succeed),
   write at the workspace root (must fail), write in `$HOME` (must fail),
   read from `packs/` (must succeed). Reports which backend is active and a
   pass/fail per probe — run it on any new machine before trusting the
   guarantee there.

## Consequences

- The credentials-drift bug that prompted this (real `token.json`/
  `credentials.json` sitting in `packs/gmail/` instead of symlinked from
  `config/gmail/`) is now something a job process *cannot silently repeat*:
  if a job's own code ever tries to write a secret next to itself instead of
  through the declared symlink, the sandboxed write fails outright instead
  of quietly succeeding in the wrong place.
- Verified in the (Linux) dev sandbox via `bwrap`: `auto sandbox-check`
  passes all 5 probes; `auto run hello-report` completes normally
  (sandboxed) and prints the loud unconfined warning under `--no-sandbox`;
  `auto doctor` / `auto serve` / `auto orchestrate` (list mode) all still
  work — nothing in `auto`'s own code path was touched.
- **Not yet verified on macOS** (`sandbox-exec`) — the profile syntax is
  correct per Apple's documented Seatbelt grammar and follows the same
  `(allow default)` / `(deny file-write* (subpath "/"))` /
  `(allow file-write* (subpath ...))` idiom used in widely-referenced public
  sandbox-exec examples, but this was written and tested on Linux. **Run
  `auto sandbox-check` on the real machine before relying on this.**
  `sandbox-exec` is also formally deprecated by Apple (still functional as
  of current macOS releases) — if it stops working in a future macOS
  version, `auto sandbox-check` will surface that immediately as
  `UNCONFINED`, rather than the failure being silent.
- Go is the only language any current job actually uses (`gmail`, `wallet`,
  `expenses`, `telegram` are all `go`; `hello-report`/`appdemo` are `bash`),
  so the Go toolchain-cache carve-out is exercised in practice; the
  node/python carve-outs are implemented for parity with the framework's
  advertised runners but untested against a real job.
- Complements ADR 0007 (pack-config injection — this sandbox enforces that
  contract's *write* side at runtime instead of only implementing it),
  ADR 0015 (`--ai` argv pre-extraction pattern, reused for `--no-sandbox`),
  and ADR 0001 (manifest-driven: nothing here is job-specific configuration,
  it's a property of `execute_job` itself).

## Amendment 1 (2026-08-09): a job's own pack directory is also a legitimate write target

The first version of the write-roots allow-list was `config/` + `data/` at
the workspace root — nothing else. The very first real job run against it,
`expenses-update-event --write-csv`, failed immediately:

```
error: saving registry: open config/events.json.tmp: operation not permitted
```

`packs/expenses/config/events.json` (the versioned event registry) and
`packs/expenses/state.json` (the git-ignored assignment ledger) are written
**directly inside the pack's own directory** — that's the documented design
in ADR 0011 decision 3, itself applying ADR 0005's versioned-vs-local
provenance split *at the pack level*, not just within the workspace's
top-level `data/`. This is a first-class, everyday pattern (every
app-backed pack — expenses, wallet — owns state files this way), not an
edge case, and the original allow-list didn't account for it at all.

**Corrected decision:** `_sandbox_write_roots(j)` now includes
`j["_packpath"]` (the invoking job's own pack root) alongside `config/`,
`data/`, and the toolchain caches. A job can write anywhere inside its own
pack, or the shared `config/`/`data/` areas — it still cannot touch another
pack, the workspace root, `docs/`, or `$HOME`. Verified with a probe against
the exact failing paths (`packs/expenses/config/events.json.tmp`,
`packs/expenses/state.json.tmp` → allow) alongside the original probes
(`packs/gmail/rogue.tmp` from an *expenses* job, workspace root, `$HOME` →
still deny).

**A discarded alternative:** keeping the narrow allow-list but adding a
literal-path *deny* for each pack's declared secret files (`credentials.json`,
`token.json`) layered on top of a general pack-dir allow, so the sandbox
itself would re-catch the original credentials-drift bug (ADR 0007) as well
as permit registries/ledgers. This is expressible on macOS (Seatbelt
resolves rule specificity: a `(deny file-write* (literal ...))` for one
exact path overrides a broader `(allow file-write* (subpath ...))` around
it) but not cleanly on Linux `bwrap`, which is mount-based: there's no
primitive for "this subtree is writable except this one filename," and the
closest approximation (an extra `--ro-bind` of the exact path layered after
the writable bind) risks freezing a legitimate symlink into a stale
read-only snapshot for the process's lifetime — breaking the exact
write-through-symlink behavior ADR 0007 depends on for a token refresh to
persist. Rather than ship an asymmetric guarantee (real protection on macOS,
a subtly broken one on Linux) or spend more time getting bwrap's mount
ordering exactly right under time pressure while a real job was failing,
the credentials-drift check was moved to `auto doctor` instead (see
`cmd_doctor`): a plain static check — for every pack, every file declared in
`config.sample.yaml`'s `files:` list must be a symlink, never a real file,
inside the pack directory. No mount-ordering subtlety, no per-OS asymmetry,
and it's the check that actually caught the original bug when written this
way. The sandbox's job is the broader, coarser guarantee (pack-scoped write
containment); precise per-file contract enforcement is `doctor`'s job.

## Amendment 2 (2026-08-09): reverted — packs/ is read-only, no exceptions

Amendment 1 was wrong. Sumit corrected it directly: no pack's registry
should live inside the pack; `packs/` is read-only as far as `auto` is
concerned; all data and cache belong in directories rooted at the workspace
(`data/`, `config/`), not inside a pack.

**Reverted:** `_sandbox_write_roots(j)` no longer includes
`j["_packpath"]`. The write-roots are back to exactly `config/` + `data/` +
toolchain caches — nothing else, no per-pack carve-out. A job cannot write
*anywhere* inside `packs/`, including its own pack's directory.

**What makes this work without breaking `expenses-update-event` again:**
the actual gap Amendment 1 was patching over wasn't "packs need to be
writable" — it was "`packs/expenses/config/events.json` and
`packs/expenses/state.json` had nowhere else to live." ADR 0019 (new)
gives them one: `data/expenses/`, reached from the pack's workdir through an
`auto`-managed symlink, the exact same mechanism ADR 0007 already uses for
secrets. The physical files moved: `packs/expenses/config/events.json` →
`data/expenses/events.json`, `packs/expenses/state.json` →
`data/expenses/state.json`, `packs/wallet/state.json` → `data/wallet/state.json`.
`packs/expenses/config.sample.yaml` / `packs/wallet/config.sample.yaml` now
declare these under a new `data_files:` key; `execute_job` symlinks them in
before every run, same as it already did for `files:`. The app code itself
(`main.go` in both packs) needed **no changes** — it still opens
`config/events.json` / `state.json` at its usual relative path; that path is
just a symlink now, and the write resolves through it into `data/`, which
*is* an allowed root.

Verified: a probe against the exact same job (`expenses-update-event`) now
shows writes through the two declared `data_files` symlinks succeeding,
while a write to a *new*, undeclared file directly in `packs/expenses/`
(e.g. `packs/expenses/rogue-new-file.txt`) is denied — packs are read-only
for anything not explicitly routed through `config/` or `data/`, including
the pack's own would-be state files. `auto doctor` now also checks
`data_files:` the same way it checks `files:`: every declared entry must be
a symlink into `data/<pack>/`, never a real file in the pack directory.

This supersedes Amendment 1's write-roots change (not its diagnosis of the
`expenses-update-event` failure, which was correct — only its fix, which was
too broad). See ADR 0019 for the full `data_files:` mechanism and the
ADR 0009/0011 amendments recording where wallet's and expenses' state
actually live now.

## Amendment 3 (2026-08-29): allow-listed roots must be `.resolve()`d — a symlinked `data/` silently denied every write under it

`gmail-extract` started failing on every scheduled run: `writing CSV: opening
CSV for write: open transactions.csv: operation not permitted`, plus a
`[WARN] saving forwarded-notes state: ... operation not permitted` on the
same run. `transactions.csv` is exactly the kind of write this ADR is
supposed to allow — a symlink from `packs/gmail/transactions.csv` through
`data/gmail/transactions.csv` — yet the sandbox was denying it.

**Root cause:** on this workspace, `data/` at the workspace root is *itself*
a symlink (`automation-monorepo/data -> /Users/sumitasok/data`, kept outside
the repo). `_sandbox_write_roots()` built its allow-list from
`DATA = WS / "data"` without ever resolving it, so the generated Seatbelt
profile allow-listed the literal, unresolved path
`.../automation-monorepo/data`. But `sandbox-exec` evaluates a
`(subpath ...)` rule against the resolved/canonical path a write actually
lands on, not the literal string handed to it when the profile was
generated — this is precisely why the base profile already needed both
`(subpath "/tmp")` *and* `(subpath "/private/var/folders")` (macOS's real
`/tmp` resolves into the latter). The real write target,
`/Users/sumitasok/data/gmail/transactions.csv`, is not a subpath of the
unresolved `.../automation-monorepo/data` string, so Seatbelt denied it —
silently, for every write under `data/`, on any machine where `data/` (or,
had it come up, `config/`) is a symlink outside the workspace. This is a
strictly more severe version of the exact bug this ADR already exists to
catch (Amendments 1/2): the *fix itself* had an unresolved-symlink gap.

**Fix:** resolve each allow-listed root once, at the point the workspace
computes it — `DATA = (WS / "data").resolve()` and a new
`CONFIG_ROOT = (WS / "config").resolve()` (replacing the five inline
`WS / "config"` call sites: `pack_config_dir`, `ai_profile_dir`,
`_sandbox_write_roots`, and both `sandbox-check` probe references) — rather
than resolving only inside `_macos_sandbox_profile`. This keeps `DATA`/
`CONFIG_ROOT` consistent everywhere they're used, including the
`AUTO_DATA_DIR` env var injected into every job's process (several jobs —
`wallet fetch --out`, `wallet detect-duplicates`, `gmail serve --data-dir`
— resolve paths from `$AUTO_DATA_DIR` themselves; they now get the real
path too, not just the sandbox).

**Verification:** `DATA` now resolves to `/Users/sumitasok/data` (confirmed
via `auto sandbox-check`, which tried to create `/Users/sumitasok/data/state`
— the correct real target — rather than the old in-repo symlink path).
Full `sandbox-exec` enforcement itself still needs to be re-verified with
`auto sandbox-check` run directly on the Mac (this fix was authored and
partially checked through a Linux-based remote bridge that has no
`sandbox-exec` binary at all, the same blind spot the original ADR called
out for its own initial verification).

**Not fixed by this amendment:** a separate, unrelated bug surfaced in the
same failing run — `filters/_forwarded-notes.yaml.state` (and, it turns out,
every per-bank `filters/<name>.yaml.state`) is a real file sitting directly
in `packs/gmail/filters/`, never migrated to the `data_files:` mechanism
(ADR 0019) the way it should have been alongside `transactions.csv`. The
sandbox is *correctly* denying that write per this ADR's own contract —
`packs/` is read-only. Left open for a follow-up ADR/fix.
