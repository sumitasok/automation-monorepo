# Phase 0 Research: User Comments Inform Transaction Classification

No `[NEEDS CLARIFICATION]` markers remain in `spec.md`. This phase investigates
*how* to realize the spec's assumptions against the actual, current state of
the two consumer packs — both already carry spec 002's rules engine and
`Source` decision-tracking, which this feature builds directly on top of.

## Decision 1: Where the comment column lives, and its name

**Decision**: A new `UserComment` column in `data/gmail/transactions.csv`,
appended after the existing `Source` column (the current last column per
`store/csv.go`'s `csvHeader`).

**Rationale**: FR-007 requires the comment to be "an additional column...
consistent with how existing enrichment columns... are already stored."
`transactions.csv` already grew additively three times (Category/SubCategory/
Labels in ADR 0010, `Note` in ADR 0013, `Source` in ADR 0016) with the same
pattern: append at the end, pad short legacy rows, never renumber existing
columns. Naming it `UserComment` (not `Comment`) makes FR-008's distinction
from the existing `Note` column impossible to confuse at a glance — `Note`
already means "extracted from a manually-forwarded email's preamble" (ADR
0013); `UserComment` means "typed directly into this CSV cell." Both are
free text and both may be non-empty on the same row (FR-008/FR-009), so a
name collision (even conceptually, e.g. calling this new column `Note2`)
would be actively confusing.

**Alternatives considered**: Reusing `Note` for this purpose was rejected
outright — the spec (FR-008) explicitly forbids conflating the two, since
they have different origins (direct edit vs. forwarded email) and a user
might reasonably want both filled on one row.

## Decision 2: Detecting "comment added or changed" vs. "comment unchanged" (FR-010/FR-011)

**Decision**: A second new, system-written column, `CommentConsidered`,
snapshotting the exact `UserComment` value that was in effect the last time
this row was classified (gmail side) or matched (expenses side, as a new
`Comment` field on `state.json`'s `AssignmentEntry` — expenses cannot write
`transactions.csv`, ADR 0011 decision 2, so it needs its own copy in the file
it already owns). A row is "comment-eligible" (must be (re)considered) when
`strings.TrimSpace(UserComment) != strings.TrimSpace(CommentConsidered)`;
once classified, `CommentConsidered` is set equal to the comment that was
used, so the next run with no further edits sees them equal and leaves the
row alone.

**Rationale**: This is a dirty-tracking pattern already implicit elsewhere in
this codebase — `NeedsCategory()` is exactly this idea applied to "has an
outcome at all." A plain boolean ("was a comment ever considered") cannot
distinguish FR-011's "unchanged since last classification" from FR-010's "a
comment was edited" — both would read as "yes, a comment was considered" —
so the actual *value* must be snapshotted, not just its presence. Comparing
trimmed values (rather than a hash) keeps the mechanism transparent: opening
the CSV/`state.json` by hand (which the spec's Assumptions section says is
how comments are authored in the first place) shows exactly what will
trigger reprocessing, with no hidden hash to reason about.

**Alternatives considered**: A hash (e.g. a short checksum of the comment) was
considered to keep the new column narrow, but rejected — this workspace has
no existing precedent for hashing a human-editable field, the comment values
here are short (personal notes, not documents), and a plaintext snapshot is
strictly more debuggable for a single user hand-inspecting the file. A
separate `CommentUpdatedAt` timestamp (rather than a value snapshot) was also
rejected — it would require trusting that every edit path updates it (the
Assumption is that the user edits the raw CSV/state directly with no tool in
between), which a manual text-editor save cannot be relied on to do.

## Decision 3: How a present comment overrides a would-be rule match (FR-012)

**Decision**: In each pack's `Run()`, the `rules.Match(...)` call is skipped
entirely for a row whose current `UserComment` (gmail) / `Comment` (expenses
join, via `csvtxn.Txn.UserComment`) is non-empty after trimming — such a row
goes straight into the AI-bound `items`/batch with its comment attached,
regardless of whether a rule would otherwise have matched it. A row with an
empty comment continues through the existing rules-then-AI path, byte-for-byte
unchanged (SC-003).

**Rationale**: FR-012 states the comment takes precedence "for as long as
that comment remains present" — combined with Decision 2's dirty-tracking,
this composes cleanly: the row only re-enters the selection loop at all when
its comment is new/changed (or it has no outcome yet), and while it's in that
eligible state, a non-empty comment always steers it to the AI. Once
classified, `CommentConsidered` catches up to `UserComment` and the row drops
out of the eligible set — so the "for as long as present" language is
satisfied without needing to re-derive rule-vs-comment precedence on every
single run for rows that haven't changed. This is a strict superset of
spec 002's existing precedence rule (rule-before-AI); it doesn't change that
rule's behavior for comment-free rows at all.

**Alternatives considered**: Running the rule check anyway and only
overriding its result if a comment is present (rather than skipping the call)
was considered and rejected as pointless extra work with an extra failure
mode (a rule's outcome could still get logged/warned about even though it's
about to be discarded) — skipping the call outright is simpler and cheaper.

## Decision 4: Comment as AI input — shape and defensive framing (FR-003/FR-004/FR-006)

**Decision**: Both `categorize.Item` (gmail) and `event.Item` (expenses) gain
one new field, `Comment string` (`json:"comment,omitempty"`), populated only
when non-empty. Both packs' `buildPrompt` functions render it as a clearly
labelled, per-transaction field — not concatenated into the instruction text —
and both `systemPrompt` constants gain one new sentence instructing the model
that any `comment` field is user-authored *descriptive context about that one
transaction*, never an instruction, and that the taxonomy/allowed-values list
and output JSON schema remain fixed regardless of what a comment says.

**Rationale**: `omitempty` means a comment-free row's JSON payload is
byte-identical to today's (SC-003) — no `"comment": ""` clutter that could
itself become something a model over-indexes on. Per-transaction-object
placement (not prompt-level) keeps the defensive framing local to a single
system-prompt sentence rather than needing per-batch pre/post-processing;
this mirrors how ADR 0010/0011 already defend the *output* side (validate
against taxonomy/registry, never trust blindly) — here the same discipline is
applied to the *input* side, one added sentence, no new validation machinery
needed since the taxonomy/registry validation that already runs on every
response is what actually enforces "the comment cannot expand what's valid."

**Alternatives considered**: A separate API call or separate prompt section
purely for "read this comment and decide if it changes anything" was
considered and rejected as needless complexity — the existing per-transaction
JSON-object shape already has room for one more field, and the model is
already asked to reason over merchant/info/subject text for every row; a
comment is just one more input signal of the same kind.

## Decision 5: Recording that a comment shaped the outcome (Story 3 / FR-C.f. spec Key Entities)

**Decision**: The existing `Source` value (`"ai:<provider>"` from spec 002)
gains a `+comment` suffix — `"ai:<provider>+comment"` — written whenever the
row that was just AI-classified/matched had a non-empty comment considered in
that same call. Rule-decided rows are unaffected (`"rule:<name>"`, unchanged)
since Decision 3 guarantees a rule only ever decides a comment-free row.

**Rationale**: SC-004 requires determining comment-influence "without reading
source code or raw logs" — the `Source` column/field is exactly the place
Sumit already looks (per spec 002's own quickstart) to see rule-vs-AI
provenance, so extending its vocabulary is strictly cheaper than adding a new
column/field, and `"ai:<provider>+comment"` still starts with `"ai:"` so any
existing simple prefix check (`strings.HasPrefix(source, "ai:")`) keeps
working unmodified.

**Alternatives considered**: A separate boolean column (`CommentInfluenced`)
was considered and rejected as redundant with `CommentConsidered` (Decision
2) being non-empty on that row — the information is already present, so a
purpose-built `Source` suffix is the minimal addition that also satisfies
SC-004's "without reading source code" bar (a suffix reads naturally; a
second boolean column requires knowing to cross-reference it).

## Decision 6: Interactive vs. scheduled/cron detection (FR-013)

**Decision**: `os.Stdin.Stat()`'s `os.ModeCharDevice` bit — pure stdlib, no
new dependency. A small `isInteractive() bool` helper, duplicated per pack
(same duplication rationale as spec 002 Decision 3), returns true only when
stdin is a real terminal.

**Rationale**: This workspace's job runner (`framework/tools/auto`) has no
existing `AUTO_INTERACTIVE`-style env var — `auto run` (a human at a
terminal) and a cron-installed `auto run <job-id>` (via `_sync_cron` in
`framework/tools/auto`) invoke the exact same code path with no distinguishing
signal passed in today (confirmed by reading `execute_job`). A TTY check on
stdin is the standard, dependency-free Go idiom for this exact distinction,
and it fails safe: a cron-triggered process's stdin is not a terminal (it's
typically `/dev/null` or unset), so `isInteractive()` returns false there
even if `--suggest-similar` were accidentally left on in a cron config —
directly satisfying FR-017's "regardless of configuration."

**Alternatives considered**: Adding a `golang.org/x/term` dependency for a
more robust `IsTerminal` check was considered and rejected — it would be
`packs/expenses`'s second external dependency for a check the stdlib already
covers on macOS/Linux (this workspace's only `runs_on.os` targets), so the
extra dependency buys nothing here. A new `AUTO_INTERACTIVE` env var wired
through `framework/tools/auto` was also considered and rejected as
out-of-scope surgery on a shared tool for a single feature's benefit, when
the TTY check already gives the correct answer without touching `auto` at
all.

## Decision 7: The opt-in flag and its scope (FR-014/FR-018)

**Decision**: Both `categorize` and `update-event` gain one new bool flag,
`--suggest-similar` (default `false`). The retroactive-suggestion flow
(Story 5) runs only when `isInteractive() && *suggestSimilar`; either
condition alone is insufficient (FR-013's AND, Decision 6).

**Rationale**: A single, symmetrically-named flag on both subcommands
matches the spec's own Assumptions ("each gains its own opt-in
retroactive-suggestion parameter for its own domain"). Requiring both the
flag AND a TTY (rather than the flag alone) is a deliberate belt-and-braces
reading of FR-017 — "never... regardless of configuration" is a strong
guarantee, and a config-only gate is one `cron -e` typo away from violating
it, whereas a TTY is not something a crontab entry can accidentally provide.

## Decision 8: What "similar, already-decided" means for a Story 5 candidate

**Decision**: Given a just-corrected row (its new Category/SubCategory, on
the gmail side, or its new EventID, on the expenses side), candidates are
other rows/entries that (a) are already decided (gmail:
`!NeedsReclassification()`; expenses: `st.Has(id)`), (b) are not the
just-corrected row itself, (c) share the same `Merchant` (case-insensitive
exact match — the same field `merchant_contains` rule conditions already key
on), and (d) currently hold a *different* outcome than the one just produced
(different Category/SubCategory pair, or a different EventID) — an identical
outcome is not a correction candidate, it's already right. Candidates are
presented oldest-`TxnDate`-first.

**Rationale**: The spec's own Assumptions section names "the same signals the
expense-rules engine already matches on — primarily the same merchant"" as
the intended basis, explicitly leaving exact matching logic to planning.
Merchant equality reuses `containsAnyFold`-style case-insensitive comparison
already present in both packs' `rules.go` — no new matching primitive. Oldest-
first ordering matches the user story's own framing ("several other, older
transactions") and gives a stable, reproducible walk order for the
approve/skip session (SC-006's per-row approval guarantee doesn't depend on
order, but a stable order makes a session resumable-in-spirit and testable).

**Alternatives considered**: Reusing the just-corrected row's *previous*
`rule:<name>` Source tag (when present) as an additional/alternative
candidate filter — "other rows this same rule decided" — was considered, since
the Assumptions text also names it ("and/or the same rule that would have
applied"). It is folded in as a *secondary* signal, not a replacement: when
the corrected row's prior Source was `rule:<name>`, candidates are the union
of (same-merchant) and (prior `Source == "rule:<name>"`), still excluding
identical-outcome rows. This costs nothing extra to compute (Source is
already loaded) and covers cases where a rule's merchant list is broader or
narrower than a single literal merchant string.

## Decision 9: The approve/skip interaction shape

**Decision**: A simple `bufio.Scanner`-over-`os.Stdin` read loop, one
candidate at a time: print the transaction's key fields plus the proposed new
outcome, read a line, treat `y`/`yes` (case-insensitive) as approve and
anything else (including empty input) as skip, then move to the next
candidate. No new terminal UI dependency.

**Rationale**: No existing interactive-prompt code exists anywhere in either
pack to mirror (confirmed by search) — this is the first such UI in either
codebase — so the simplest possible primitive (`bufio.Scanner`, stdlib) that
satisfies FR-015/FR-016 (present each candidate individually, wait for an
explicit decision, never touch a skipped row) is the right level of
investment for a personal, single-user CLI tool with no other interactive
flows to stay consistent with.

## Decision 10: Recording an approved Story 5 suggestion's provenance

**Decision**: `Source` (or `AssignmentEntry.Source`) for a row updated via an
approved suggestion is written as `"suggested:<original-source>"` — e.g.
`"suggested:ai:deepseek+comment"` or `"suggested:rule:hungerbox-workplace-food"`
— capturing both "this came from an approved suggestion, not a fresh AI call
or rule match" (spec's Key Entities section, verbatim) and which mechanism
originally produced the correction it's copying.

**Rationale**: The spec's Key Entities section explicitly calls for "a new
'approved suggestion' case... rather than a fresh AI call or a rule match" —
a distinct, greppable prefix (`suggested:`) satisfies that without losing the
traceability a bare `"suggested"` string would lose (which correction this
one is derived from).

## Decision 11: Rule capture — locating the workspace root and the git binary

**Decision**: Reuse the already-resolved `--rules-file` path (Decision inherited
from spec 002 Decision 4: `$AUTO_DATA_DIR/config/expense-rules.yaml` under
`auto`, else `../../data/config/expense-rules.yaml` for local `go run .`) to
derive the workspace root as `filepath.Dir(filepath.Dir(filepath.Dir(rulesFilePath)))`
(strip `expense-rules.yaml`, `config`, `data`). Git commands
(`git status --porcelain --`, `git add --`, `git commit -m`) run via
`os/exec.Command("git", ...)` with `Dir` set to that resolved workspace root —
never the pack's own (different) git repository.

**Rationale**: No new flag is needed — the rules file's own location already
identifies the workspace root unambiguously, and this derivation works
identically whether the pack is running under `auto` (env-injected
`AUTO_DATA_DIR`) or via local `go run .` (relative-path fallback), exactly
like every other `AUTO_DATA_DIR`-aware default in both `main.go` files. Both
`packs/gmail` and `packs/expenses` are independently-versioned git
repositories in their own right (a submodule and a separate module
respectively) — running `git` with the *workspace* root as `Dir` (not the
pack's own root, which `os.Getwd()` would return under `auto`'s `workdir`
convention) is essential, since `data/config/expense-rules.yaml` is versioned
in the parent monorepo, not in either pack's own repo.

**Alternatives considered**: Requiring the user to commit rule-capture changes
themselves (dropping FR-021's "without requiring the user to run git commands
manually") was considered as a way to avoid shelling out to `git` at all, but
rejected — it's a direct, explicit requirement (FR-021, SC-008) and this
workspace already assumes `git` on `PATH` universally (the root `CLAUDE.md`'s
global auto-commit convention). A Go git library (`go-git`) was considered
and rejected as a needless dependency for three simple, already-scriptable
git invocations.

## Decision 12: When rule capture is offered (Story 6 trigger point)

**Decision**: Immediately after each row is corrected via Story 4's
comment-driven reclassification, or immediately after each Story 5 candidate
is approved — but only when `isInteractive()` is true (Decision 6). A
non-interactive/cron run never prompts, consistent with "no one present to
approve anything" applying equally to this offer.

**Rationale**: The spec's own framing places rule capture "right after
approving a correction" (Story 6's narrative) — tying the prompt to the
correction event itself (rather than, say, a separate end-of-run summary)
keeps the merchant/outcome context fresh in the same terminal interaction,
and reuses Decision 9's interaction primitive (one more `y`/`yes` read).
