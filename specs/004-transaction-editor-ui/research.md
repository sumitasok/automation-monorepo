# Phase 0 Research: Gmail Transactions Editor UI

## 1. Data access: extend `store.CSVStore` vs. a new data layer

- **Decision**: Extend the existing `packs/gmail/store.CSVStore` with one new method for annotation-field edits, rather than building a parallel CSV reader/writer for the UI.
- **Rationale**: `CSVStore` already owns the read-modify-write contract for `transactions.csv`, including the header-merge logic that fixed a real mislabeling bug (docs/adr/0017). A second parser for the same file would risk reintroducing that class of bug and violates the "reuse existing code" convention this repo already follows (`categorize/*.go` reuses `CSVStore` rather than re-reading the file).
- **Alternatives considered**: A separate SQLite/embedded DB — rejected, adds an infra dependency and a sync problem against the CSV that the `discover`/`categorize` CLI commands still write directly, for no benefit at ~500 rows. A read-only view with edits queued elsewhere — rejected, the spec (FR-006) requires edits to be immediately visible to other processes reading the same file.

## 2. New store method shape and its effect on `Source`

- **Decision**: Add `SetAnnotation(idx int, category, subCategory string, labels []string, note, userComment string) error` to `CSVStore`. It always writes Category/SubCategory/Labels/Note/UserComment. It sets `Source` to `"user"` **only when** the incoming Category/SubCategory/Labels differ from the row's current values; editing only Note or UserComment leaves `Source` and `CommentConsidered` untouched.
- **Rationale**: `Source` (ADR 0016) already carries a controlled vocabulary — `rule:<name>`, `ai:<provider>`, `ai:<provider>+comment` — recording *what decided* the classification. A direct UI edit to Category/SubCategory/Labels is a new, distinct decision-maker and must say so (`"user"`), or a future `categorize` run's `NeedsCategory()` check would look at a fully-populated row and never know a human overrode it. Editing only UserComment must **not** touch `Source`/`CommentConsidered`, because `Record.NeedsReclassification()` (spec 003) already uses a UserComment-vs-CommentConsidered mismatch as its own signal — touching `Source` here would just be redundant with that existing mechanism, and touching `CommentConsidered` would wrongly mark the new comment as already considered.
- **Alternatives considered**: Reusing `SetEnrichment` as-is for UI edits — rejected, it always overwrites `CommentConsidered`, which would incorrectly mark a freshly-typed comment as already factored into classification and suppress the reclassification spec 003 relies on. Adding a generic `SetField(idx, name, value)` — rejected, it would let callers bypass the Source-vocabulary rule above; explicit methods keep that invariant enforced in one place.

## 3. HTTP layer

- **Decision**: Go 1.22 stdlib `net/http` with the enhanced `http.ServeMux` (method+path patterns, e.g. `"GET /api/transactions"`, `"PATCH /api/transactions/{id}"`), `html/template` for the initial page, `encoding/json` for the API.
- **Rationale**: Matches this repo's stdlib-first precedent (`categorize/interactive.go`'s stdlib-only TTY detection) and adds zero new `go.mod` entries for a single-user local tool with a handful of routes.
- **Alternatives considered**: A router library (chi/gin/echo) — rejected as unneeded weight; Go 1.22's stdlib mux already does method+path-parameter routing.

## 4. Frontend approach

- **Decision**: Server-rendered `html/template` page for the initial Transactions table plus vanilla JS (`fetch`) for inline editing, filtering, and a client-side tab shell (one tab — Transactions — active now, structured so a second tab can be added later per the spec's own assumption).
- **Rationale**: No frontend toolchain (Node/npm/a bundler) exists anywhere in this repo today. Introducing one for a single local page is new, ongoing-maintenance infrastructure this personal-use tool doesn't need; a static Go binary serving HTML/JS/CSS keeps the whole feature inside `packs/gmail`'s existing build (`go build`).
- **Alternatives considered**: A React/Vue SPA — rejected for the same reason; would require adding and maintaining a JS build pipeline with no other consumer in the repo.

## 5. Row identity in the API

- **Decision**: The API addresses a transaction by `MessageID` (already `CSVStore`'s own unique key, `rowMap[MessageID] → index`), not by its in-memory `Index`.
- **Rationale**: `MessageID` is the stable, semantically meaningful identifier; `Index` is an implementation detail of the current in-memory snapshot and isn't guaranteed stable across a reload if rows are ever removed or reordered by another process.

## 6. Ordering: "latest event first"

- **Decision**: Sort by `TxnDate` descending; a row with an empty/unparseable `TxnDate` sorts to the end, not interleaved. `TxnDate` values are already normalised to `"YYYY-MM-DD"` or `"YYYY-MM-DD HH:MM:SS"` by `parser.NormaliseDate`, so a lexicographic string sort is equivalent to a chronological one.
- **Rationale**: Directly satisfies the spec's Assumption ("latest event" = transaction date, not email date) and Edge Case (missing date must not break ordering), using data already in its final normalised form — no new date-parsing code needed.
- **Alternatives considered**: Sorting by `EmailDate` — rejected per the spec's own assumption, since a forwarded/delayed email's `EmailDate` can differ from when the transaction actually happened.

## 7. Concurrency & staleness (FR-010)

- **Decision**: The `webui` server holds one `*store.CSVStore` behind a `sync.Mutex`, reloaded from disk lazily by comparing `transactions.csv`'s `os.Stat` mtime against the mtime recorded at last load; a save request against a stale in-memory copy is rejected with a "data changed, please refresh" response instead of silently overwriting.
- **Rationale**: `discover`/`categorize` are separate CLI processes that can rewrite the same file while the server is running; an mtime check is a cheap, stdlib-only way to detect that without building a full transaction/locking system, and directly satisfies FR-010 and the spec's "external modification" edge case.
- **Alternatives considered**: OS-level file locking (`flock`) — rejected as unnecessary complexity for a single-user tool where "detect and ask to refresh" is an explicitly acceptable behavior (FR-010), not "prevent at the OS level".
