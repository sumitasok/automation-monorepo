# Contract: CLI surface changes

## `gmail-categorize` (`packs/gmail`, `go run . categorize`)

New flag, added alongside the existing `categorize` flags in `main.go`:

| Flag | Default | Description |
|---|---|---|
| `--rules-file` | `$AUTO_DATA_DIR/config/expense-rules.yaml` if `AUTO_DATA_DIR` is set, else `../../data/config/expense-rules.yaml` | YAML file of expense classification rules evaluated before the AI provider (see `contracts/expense-rules.schema.md`). |

Existing flags (`--csv`, `--taxonomy`, `--ai-provider`, `--ai-model`,
`--batch-size`, `--limit`, `--dry-run`, `--ai`, `--ai-profiles-dir`) are
unchanged. `--dry-run` previews both rule-matched and AI-matched rows,
tagging each with its source in the printed output (mirrors the existing
dry-run print format, extended with the source tag).

**Unchanged behavior**: with an absent or empty rules file, `categorize`'s
observable behavior is byte-for-byte identical to today (spec SC-005).

## `expenses-update-event` (`packs/expenses`, `go run . update-event`)

New flag, added alongside the existing `update-event` flags in `main.go`:

| Flag | Default | Description |
|---|---|---|
| `--rules-file` | `$AUTO_DATA_DIR/config/expense-rules.yaml` if `AUTO_DATA_DIR` is set, else `../../data/config/expense-rules.yaml` | Same shared file as gmail's `--rules-file`. |

Existing flags (`--csv`, `--events`, `--state`, `--ai-provider`, `--ai-model`,
`--threshold`, `--batch-size`, `--limit`, `--dry-run`, `--write-csv`) are
unchanged.

**Unchanged behavior**: with an absent or empty rules file, `update-event`'s
observable behavior is byte-for-byte identical to today (spec SC-005).

## Manifests

`packs/gmail/jobs/gmail-categorize/manifest.yaml` and
`packs/expenses/jobs/expenses-update-event/manifest.yaml` each gain the
shared rules file in their `data.reads` list, documenting the new read-only
dependency (no `writes` change — neither job writes the rules file):

```yaml
data:
  reads: [transactions.csv, config/taxonomy.yaml, ../../data/config/expense-rules.yaml]
```

(path shown relative to each pack's own workdir, matching the existing
`data.reads` convention seen in both manifests today).

## Output / observability contract

- Every log line already printed per assignment (`categorize`'s dry-run
  print, `update-event`'s dry-run print) gains a `[rule:<name>]` or `[ai]`
  tag, so a `--dry-run` preview alone is enough to audit which mechanism
  would decide each row — satisfying spec User Story 4 / FR-010 without
  requiring the user to open the CSV or `state.json` directly (though those
  remain the durable, post-run record).
