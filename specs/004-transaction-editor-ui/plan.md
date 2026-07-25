# Implementation Plan: Gmail Transactions Editor UI

**Branch**: `004-transaction-editor-ui` | **Date**: 2026-07-25 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/004-transaction-editor-ui/spec.md`

## Summary

Add a local, single-user web UI to the existing `packs/gmail` Go CLI: a new `serve` subcommand that starts an HTTP server showing a Transactions tab backed by `data/gmail/transactions.csv`, newest transaction date first, with inline editing of the annotation fields (Category, SubCategory, Labels, Note, UserComment) — reusing the existing `store.CSVStore` read-modify-write logic rather than building a parallel data layer.

## Technical Context

**Language/Version**: Go 1.22 (existing `packs/gmail` module — `github.com/sumitasok/sa.automation.gmail`)

**Primary Dependencies**: Go stdlib only — `net/http` (Go 1.22 `ServeMux` method+path patterns), `html/template`, `encoding/json`. Vanilla JS/CSS served as static assets for the browser side — no npm/Node toolchain, no new `go.mod` dependency.

**Storage**: `data/gmail/transactions.csv`, read/written through the existing `store.CSVStore` (extended with one new method for annotation-field edits; see `data-model.md`).

**Testing**: `go test` — table-driven tests, matching the existing convention in `store/csv_test.go` and `categorize/*_test.go`.

**Target Platform**: Local machine (macOS/Linux), single user, accessed via `http://localhost:<port>` in a browser.

**Project Type**: Single project — the UI is a new package (`webui`) inside the existing `packs/gmail` module, wired in as a CLI subcommand, not a separate application or repo.

**Performance Goals**: Interactive local use — list/edit round-trips well under 1s for hundreds to low-thousands of rows (no database needed at this scale; matches SC-001's 2-second load target with large margin).

**Constraints**: No new runtime dependencies beyond the module's existing ones; edits must never corrupt `transactions.csv`'s structure; single-writer safety within the server process (an in-process mutex serializes reads/edits against the shared `CSVStore`, since the `discover`/`categorize` CLI commands can also write this file from a separate process — see FR-010 staleness handling in `research.md`).

**Scale/Scope**: Hundreds to low thousands of transaction rows (current file: ~480); one concurrent user.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

`.specify/memory/constitution.md` is still the unfilled template (no ratified principles) — no project-specific gates apply. Falling back to this repo's established, observable conventions as the bar: reuse existing code before adding new (`store.CSVStore`, not a parallel CSV layer), stdlib-first (no new dependency for something the standard library already does, per `categorize/interactive.go`'s stdlib-only TTY check precedent), and additive schema changes only (never rename/remove existing CSV columns, per every prior `csv.go` change). This plan satisfies all three. **PASS.**

## Project Structure

### Documentation (this feature)

```text
specs/004-transaction-editor-ui/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md         # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
└── tasks.md             # Phase 2 output (/speckit-tasks — not created here)
```

### Source Code (repository root)

```text
packs/gmail/                      # existing Go module — this feature extends it
├── main.go                       # add "serve" subcommand dispatch (alongside discover/categorize)
├── store/
│   ├── csv.go                    # add SetAnnotation(idx, category, subCategory, labels, note, userComment)
│   └── csv_test.go               # add coverage for the new method
└── webui/                        # NEW package
    ├── server.go                 # HTTP handlers: list/sort/filter, edit, static/template serving
    ├── server_test.go            # handler tests (httptest), including staleness/validation paths
    ├── templates/
    │   └── index.html            # tab shell + Transactions table (server-rendered initial state)
    └── static/
        ├── app.js                # inline-edit UI, fetch calls to the JSON endpoints, client-side filter
        └── app.css
```

**Structure Decision**: Single project (Option 1) — this is an addition to the existing `packs/gmail` Go module, following the same "extend, don't replace" pattern spec 003 used for `categorize/`. No `frontend/`/`backend/` split: the whole feature is one Go binary serving both the JSON API and the static/template assets, since there is no existing frontend toolchain in this repo and introducing one (Node/npm/a bundler) would be new infrastructure this single-user local tool doesn't need.

## Complexity Tracking

*No constitution violations — table omitted.*
