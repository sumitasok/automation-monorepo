# Contract: command surface

The pack's interface to its operator and to `auto`. Three jobs, each a manifest wrapping one
subcommand (the app-backed pattern of ADR 0006, as `packs/expenses` uses).

## Invocation

```
auto run portfolio-import  -- [args]
auto run portfolio-report  -- [args]
auto run portfolio-share   -- [args]
```

Directly during development: `cd packs/portfolio && python3 main.py <subcommand> [args]`.

## Global conventions

- Exit `0` success · `1` usage or validation failure · `2` refused (corporate action suspected, derivability check failed).
- Human-readable output on stdout; every diagnostic naming a file, a location and an expectation.
- `--json` prints machine-readable output instead, for a future `tax` pack or for scripting.
- **Validation of every input file happens before any figure is computed** (FR-052); a failure prints *all* problems, not the first (FR-054).
- No subcommand writes anywhere except through declared `data_files:` symlinks.

## `import` — ingest a broker export

```
main.py import --file <path> --profile <name> [--dry-run] [--allow-duplicate-rows]
```

| Flag | Default | Meaning |
|---|---|---|
| `--file` | required | Broker export to ingest |
| `--profile` | required | `profiles/<name>.yaml` |
| `--dry-run` | off | Print every lot it would create and disposal it would match; write nothing (FR-027) |
| `--allow-duplicate-rows` | off | Treat identical rows within one file as genuinely distinct fills |

**Output**: the ImportBatch summary — created, matched, skipped-as-seen, unrecognised, ignored
with reasons — and the row-count identity assertion (SC-007). Warns on any disposal that landed
shortly before its lot would have matured (FR-028).

**Always `--dry-run` first.** Import is idempotent by `src` fingerprint (FR-026), so a re-run of
the same file is a no-op, but the dry run is where an unrecognised action or a wrong profile
shows up before the register moves.

## `report` — recompute and regenerate

```
main.py report [--ticker T] [--as-of DATE] [--spot P] [--fx R] [--format page|json|text]
```

| Flag | Default | Meaning |
|---|---|---|
| `--ticker` | all | Scope to one instrument (FR-014); omit for the whole portfolio |
| `--as-of` | today | Valuation date — roll the clock forward to test maturity |
| `--spot` / `--fx` | from register | Override price / reporting-currency rate for a what-if |
| `--format` | `page` | `page` writes the declared artefact; `json` writes the document only; `text` prints to stdout |

**Writes** (through declared symlinks): `explorer.html` — the declared artefact, embedded
variant, full disclosure — and `explorer-document.json`.

`--spot`/`--fx` overrides are a what-if: they change the generated figures but MUST NOT be
written back into the register.

## `share` — produce a redacted copy

```
main.py share --profile <name> [--out <path>] [--preview]
```

| Flag | Default | Meaning |
|---|---|---|
| `--profile` | required | A disclosure profile name |
| `--out` | `shared/<profile>-<date>.html` | Destination, under `data/portfolio/shared/` |
| `--preview` | off | Open the produced copy exactly as its recipient would see it (FR-072) |

**Behaviour**: runs the derivability check first and refuses on failure (FR-049, exit 2).
Redaction deletes withheld fields from the document before the page is built (FR-047). A
full-disclosure profile requires an interactive confirmation naming what is about to leave the
machine (FR-050); it is refused on a non-interactive run rather than defaulting to yes.

Output goes to `shared/`, never over the declared artefact (FR-071), and is never declared or
served (FR-070).

## `validate` — check every data file

```
main.py validate [--strict]
```

Validates the register, rate table, rules, every broker profile and the disclosure profiles
against their schemas, plus the beyond-schema rules in data-model.md. Reports every problem at
once (FR-054). `--strict` additionally fails on any unverified-value flag, which is the check to
run before trusting figures for filing.

## Contract stability

`import`, `report` and `validate` flag names are part of this contract — the manifests and
RUNBOOK reference them. Adding a flag is compatible; renaming or removing one is a breaking
change requiring a manifest update and a RUNBOOK note.

## Not in this contract

Serving. The pack binds no port and owns no route (FR-063). `auto serve` reads the declared
artefact from disk; the pack's only contribution to being served is the declaration itself —
which is blocked on the workspace UI declaration contract (FR-062, FR-064; see plan.md
Complexity Tracking).
