# Contract: Story 6 rule capture into `data/config/expense-rules.yaml`

This extends specs/002-expense-rules-engine's existing
`contracts/expense-rules.schema.md` contract — the file's *shape* is
unchanged; this document only contracts the new *writer* behavior this
feature adds (both packs' new `rulecapture.go`).

## Preconditions (checked in order; any failure aborts with no write)

1. The run is interactive (`isInteractive()` is true) — otherwise the prompt
   is never shown in the first place, so this writer is never invoked from a
   non-interactive run (FR-013/FR-017's spirit extended to Story 6).
2. The user answered the capture prompt affirmatively (`y`/`yes`,
   case-insensitive) — otherwise nothing happens (FR-022).
3. `git -C <workspace-root> status --porcelain -- data/config/expense-rules.yaml`
   returns empty output. A non-empty result (uncommitted changes already
   present on that file) prints a clear message identifying the file and
   aborts — the capture is **not** retried automatically; the user must
   commit or stash their own changes first, then re-run (FR-020).
4. The derived rule `name` does not collide with an existing rule's `name`
   after appending numeric suffixes up to a small bound (e.g. `-2`..`-20`);
   exhausting the bound aborts with a message rather than looping forever.

## Write sequence (only after all preconditions pass)

1. Load the current `ExpenseRules` (`LoadExpenseRules`, spec 002's existing
   loader — reused, not reimplemented).
2. Append one new `ExpenseRule` (data-model.md's "Rule capture write target"
   shape) to `Rules`.
3. Marshal back to YAML **preserving existing rule order and content** —
   only the new entry is added; nothing else in the file changes byte-for-byte
   beyond the append (verified by the unit test: round-tripping an existing
   fixture file through load→append→save changes only the new-rule bytes).
4. Write the file.
5. `git -C <workspace-root> add -- data/config/expense-rules.yaml`
6. `git -C <workspace-root> commit -m "<descriptive message>"` — message
   names the merchant and captured outcome, e.g. `"Capture rule: HungerBox ->
   Food & Drinks / Groceries [Work]"` (FR-021: "descriptive commit message,"
   SC-008: "always traceable to its own git commit").
7. Print confirmation including the commit's short hash (from `git commit`'s
   own stdout, or a follow-up `git rev-parse --short HEAD`) so the user sees
   proof of the commit without needing to run git themselves.

## Failure handling

- Any `git` invocation returning a non-zero exit code aborts the remaining
  sequence and surfaces the command's stderr to the user — the YAML file may
  already be written to disk by this point (step 4 precedes the git calls);
  in that case the message explicitly says so, since the working tree is now
  dirty and the user needs to know to inspect/commit/discard it themselves
  rather than assuming nothing happened.
- This flow never runs concurrently with itself in this single-user,
  single-process CLI, so no file-locking is needed beyond what
  `LoadExpenseRules`/YAML marshal-and-write already does.
