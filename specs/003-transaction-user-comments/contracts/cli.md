# Contract: CLI surface changes

Both jobs are invoked as `go run . <subcommand> [flags]` (local dev) or via
their `manifest.yaml`'s `exec` (under `auto`). This feature adds no new
subcommand — only new flags on the two existing ones already extended by
spec 002.

## `gmail-categorize` (`packs/gmail`, `go run . categorize`)

New flag:

```
--suggest-similar   after processing, if running interactively (a real TTY on
                     stdin) and at least one row was corrected via a comment
                     this run, offer retroactive suggestions for other
                     already-decided rows resembling each correction
                     (Story 5). Default: false. Has no effect on a
                     non-interactive run (e.g. under cron) regardless of
                     this flag's value (FR-017).
```

No existing flag's default or meaning changes. `--dry-run` continues to
suppress all writes; when combined with `--suggest-similar`, suggestions are
still presented and can be "approved" for display purposes, but nothing is
written to disk (mirrors every other `--dry-run` guarantee already in this
job).

## `expenses-update-event` (`packs/expenses`, `go run . update-event`)

New flag, same contract as above, expenses-domain-scoped:

```
--suggest-similar   same semantics as gmail-categorize's flag, for event
                     assignments instead of category assignments.
```

## Interactive rule-capture prompt (Story 6)

Not a flag — an automatic prompt (`y`/`yes` to accept, anything else to
decline) shown after each comment-driven correction (direct, Story 4) or each
approved Story 5 suggestion, **only** when the run is interactive
(`isInteractive()`, research.md Decision 6). No opt-out flag is introduced;
declining the prompt each time it appears is the opt-out (FR-022 — declining
leaves the rules file untouched).

## Behavioral contract: comment precedence over a matching rule

For both jobs, per eligible row (research.md Decision 3):

```
if trim(row.currentComment) != "":
    # comment present — never consult expense-rules.yaml for this row
    send to AI with comment attached
    record outcome; Source = "ai:<provider>+comment"
else:
    # unchanged from spec 002
    if a rule matches: apply it; Source = "rule:<name>"; no AI call
    else: send to AI (no comment field in the payload); Source = "ai:<provider>"
```

## Behavioral contract: row eligibility (replaces spec 002's bare "needs an outcome")

```
gmail:    eligible := row.NeedsCategory() OR
                       (trim(row.UserComment) != "" AND
                        trim(row.UserComment) != trim(row.CommentConsidered))

expenses: eligible := NOT state.Has(id) OR
                       (trim(currentComment) != "" AND
                        trim(currentComment) != trim(entry.Comment))
```

A row/entry that is not eligible is left completely untouched — no rule
check, no AI call, no write (FR-011, SC-002 combined: comments are never
lost, and untouched rows produce byte-identical output to a run that never
saw this feature).
