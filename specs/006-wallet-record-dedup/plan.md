# Implementation Plan: Wallet Record Deduplication

**Branch**: `006-wallet-record-dedup` | **Date**: 2026-08-29 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/006-wallet-record-dedup/spec.md`

**Note**: Plan created by `/speckit-plan` command; executes specification into design phase.

## Summary

Build a CLI mechanism to detect, review, and safely deduplicate records in `data/wallet/records.json` (the mirrored Wallet API dataset). The feature identifies duplicate transactions by amount+date+counterparty, presents them to the user in tabular format, requires explicit confirmation before deletion, and executes atomic writes with automatic backups. Existing `detect-duplicates.go` detects CSV/state inconsistencies; this feature deduplicates the actual fetched records, a complementary but separate concern.

## Technical Context

**Language/Version**: Go 1.20+ (pure stdlib, no external dependencies)

**Primary Dependencies**: Standard library only (encoding/json, flag, fmt, os, sort, time, path/filepath)

**Storage**: JSON files in `data/wallet/` (records.json primary, state.json for dedup audit trail). All operations read-only on `packs/wallet/`, read-write on `data/wallet/`.

**Testing**: Go testing framework (`*_test.go`, `go test ./...`). Existing test files: config_test.go, state_test.go, wallet_test.go, sync_test.go, csvtxn_test.go

**Target Platform**: CLI (UNIX-like systems; access via `auto wallet dedup` or direct `go run . dedup`)

**Project Type**: CLI tool / command suite (multi-subcommand wallet pack)

**Performance Goals**: Handle 10,000+ records in <5 seconds (scan + confirmation + write). Memory footprint <500MB for typical use (current records.json: 6,329 records).

**Constraints**: Atomic writes (all-or-nothing dedup), data integrity (no corruption on partial failure), explicit user confirmation (irreversible data deletion), backup creation before modification, least-exposure logging (no PII in audit logs).

**Scale/Scope**: Single pack (packs/wallet/), single new subcommand (dedup), ~300-500 lines of Go code (detection + review + execution logic). Builds on existing config, state, and manifest infrastructure.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Answer each gate for THIS feature. `N/A` is a valid answer where a principle
does not apply — but state why in one line, do not leave it blank. Any `NO`
must appear in Complexity Tracking with the simpler alternative that was
rejected and why. See `.specify/memory/constitution.md` v1.0.0.

| # | Gate | Verdict |
|---|---|---|
| I | Does every value the pack needs (env, secrets, produced data) arrive via a `config.sample.yaml` declaration the workspace supplies — with no absolute path, workspace-relative path, or environment inspection in the pack? | **YES** — Dedup reads from `config.sample.yaml` declarations (same pattern as sync/fetch). Records and backups are declared in `data_files:` in config.sample.yaml; workspace supplies them via symlinks at runtime. No environment inspection in dedup logic. |
| II | Does the pack write **nothing** into `packs/` — every secret to `config/<pack>/`, every produced file to `data/<pack>/`, each reached through a declared symlink? | **YES** — All dedup operations read records.json from `data/wallet/` and write backups/audit logs to `data/wallet/`. Symlinks ensure the pack never touches `packs/wallet/` directly. |
| III | If this feature has a UI: is it a static artefact under `data/<pack>/`, declared in the manifest, opening correctly from disk, with no port bound and no route owned by the pack? | **N/A** — Dedup is a CLI tool, not a UI-generating pack. Output is text/JSON printed to stdout or written to audit logs in `data/wallet/`. No server binding, no routes, no static artefacts. |
| IV | Is every derived artefact regenerated from manifests/config on demand rather than stored, with one loader per fact and no registration step? | **YES** — Dedup report is ephemeral (run on demand, printed to stdout, not stored). Audit trail is append-only, not a regenerated cache. No hand-maintained derived lists or indexes. |
| V | Can a new instance of anything this feature handles (source, format, rule, category) be added as data, with one implementation of each shared computation and one contract covering all variants? | **YES** — Dedup key fields (amount, date, counterparty) are primary; optional extensions (category, tags, notes) are configured in `config.yaml`, not coded as variant branches. One dedup algorithm covers all comparison modes. New comparison rules added as config entries, not code paths. |
| VI | Is every boundary this feature relies on enforced by the sandbox, `auto doctor`, or repo access — not by documentation or convention? | **YES** — Sandbox enforces pack cannot write to `packs/`. Explicit user confirmation (interactive or manifest-driven) is the structural enforcement against silent data deletion. `auto doctor` verifies symlink integrity. No procedural trust in documentation. |
| VII | Does this feature bind only to localhost, render no secret values, and make any data leaving the machine an explicit configured act? | **YES** — All operations local to `data/wallet/`. No network calls, no server binding. Audit logs include record IDs and counts only, never PII or amounts. No data leaves the machine (dedup is local-only). Meets "local-first, least exposure" requirement. |

**Post-design re-check** (after Phase 1): All gates remain **PASS**. No design changes contradict Constitution principles. Dedup remains local-only, read-only on packs/, write-only to data/wallet/, and requires explicit user confirmation.

## Complexity Tracking

No Constitution violations. All principles satisfied without trade-offs. (Section omitted per template guidance.)

## Project Structure

### Documentation (this feature)

```text
specs/006-wallet-record-dedup/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output (none needed — no technical unknowns)
├── data-model.md        # Phase 1 output (/speckit-plan command)
├── quickstart.md        # Phase 1 output (/speckit-plan command)
├── contracts/
│   └── dedup-record.md  # Record dedup data contract + operation protocols
├── checklists/
│   └── requirements.md  # Quality validation checklist
└── tasks.md             # Phase 2 output (/speckit-tasks command)
```

### Source Code (repository root)

```text
packs/wallet/
├── main.go              # CLI entry point (add dedup subcommand case)
├── dedup.go             # NEW: Dedup detection, review, and execution logic
├── internal/
│   ├── wallet/
│   │   ├── wallet.go    # Existing Wallet API types and client
│   │   └── wallet_test.go
│   ├── state/
│   │   ├── state.go     # State.json audit trail tracking
│   │   └── state_test.go
│   ├── config/
│   │   ├── config.go    # Config injection (dedup config keys added here)
│   │   └── config_test.go
│   ├── csvtxn/
│   │   └── csvtxn.go    # CSV transaction types (reuse for dedup?)
│   └── sync/
│       └── sync.go      # Sync protocol (informational only)
├── dedup_test.go        # NEW: Unit tests for dedup logic
├── config.sample.yaml   # Updated: Add dedup config keys (comparisonFields, etc.)
├── pack.yaml            # Updated: Add wallet-dedup job manifest
├── jobs/
│   ├── wallet-sync/manifest.yaml
│   ├── wallet-fetch/manifest.yaml
│   └── wallet-dedup/manifest.yaml  # NEW: Dedup job declaration
├── RUNBOOK.md           # Updated with dedup usage examples
├── records.json         # Data file (read by dedup, backups written here)
└── state.json           # Data file (dedup audit trail appended here)
```

**Structure Decision**: 
- **Single CLI tool in one pack** (packs/wallet/): Dedup is a subcommand alongside `sync`, `fetch`, and `detect-duplicates`.
- **New file**: `dedup.go` contains dedup detection, review logic, and write execution. Follows existing pack patterns (config injection, state tracking).
- **Updated files**: `main.go` (add case for dedup subcommand), `config.sample.yaml` (declare dedup config keys), `jobs/wallet-dedup/manifest.yaml` (job declaration).
- **Testing**: `dedup_test.go` for unit tests (detection logic, grouping, edge cases). Existing `*_test.go` files serve as model.
- **Data integrity**: Backups written to `data/wallet/` via declared `data_files:` symlinks; audit trail appended to state.json.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |
