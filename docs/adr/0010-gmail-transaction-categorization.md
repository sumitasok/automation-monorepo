# ADR 0010 — Gmail pack: AI categorisation of transactions (DeepSeek + committed taxonomy)

**Status:** accepted — 2026-07-15

## Context

The gmail pack extracts bank-alert transactions into `data/gmail/transactions.csv`
(ADR 0006). Those rows carry the money-movement fields (amount, type, merchant,
info) but no *expense* classification. The finance app (Wallet, ADR 0009) already
owns the taxonomy of expense categories: a category **group** (e.g. "Food &
Drinks"), the category names within it (e.g. "Groceries"), and free-form
**labels** (171 of them). We want each newly-fetched transaction tagged with that
taxonomy so the CSV can feed the finance app without hand-classification.

Two facts shape the decision:

- Classification is a judgement task over unstructured merchant/info text — a good
  fit for the LLM the pack already calls for parse-failure recovery (ADR-less
  aiassist package, DeepSeek default).
- The taxonomy lives in the finance app behind an MCP server. The Go pack is a
  headless CLI with no MCP client and runs on the scheduler path, so it cannot
  fetch the taxonomy live at run time.

## Decision

1. **New `categorize` subcommand** in the gmail app, alongside `discover` /
   `recategorize`. It reads `transactions.csv`, selects rows missing any of
   Category / SubCategory / Labels, classifies them, and writes the values back
   in place. Idempotent: a re-run only touches still-unfilled rows, and a later
   `fetch` re-run preserves enrichment columns rather than overwriting them.

2. **Committed taxonomy YAML** (`config/taxonomy.yaml`), generated from the
   finance app's `get_categories` + `get_labels`. Category = group name,
   SubCategory = category name within the group; labels are a flat list. The file
   is regenerated when the finance taxonomy changes. Rejected: a live MCP fetch
   (no MCP client on the scheduler path) and hard-coding in Go (needs a recompile
   to change).

3. **DeepSeek, reusing the existing provider convention.** The `categorize`
   package has its own small DeepSeek client rather than depending on `aiassist`,
   whose Recognizer is specialised to regex generation. Same env contract
   (`DEEPSEEK_API_KEY`, `DEEPSEEK_MODEL`) and `--ai-provider` flag. Only DeepSeek
   is implemented today; the `Assigner` Strategy interface leaves room for others.

4. **Single API call per invocation by default.** All unfilled rows for a run go
   to DeepSeek in one request (`--batch-size 0`). The full taxonomy is repeated in
   every request, so one call minimises cost and latency and keeps assignments
   consistent. `--batch-size N` splits very large backlogs to stay within
   output-token limits.

5. **Validate every assignment against the taxonomy before writing.** The model's
   category/subcategory pair is resolved to canonical spelling (case-insensitive;
   a valid sub-category corrects a wrong group); labels are filtered to known
   values, de-duplicated, and capped at 5. Unresolvable rows are skipped and
   logged, never written partially.

## Consequences

- `transactions.csv` gains three columns (Category, SubCategory, Labels). The
  first `categorize` run migrates legacy 13-column rows to 16 columns.
- The taxonomy can drift from the finance app between regenerations; that is an
  accepted, documented trade-off for headless operation.
- Categorisation quality depends on DeepSeek; `--dry-run` previews assignments
  before any write.
