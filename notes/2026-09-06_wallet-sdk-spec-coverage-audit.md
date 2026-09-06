# Wallet SDK — spec coverage audit

**Date**: 2026-09-06
**Subject**: `packs/expense-domain/sources/wallet/sdk/{go,python}` against
`wallet-api-openapi-reference.md` (OpenAPI 3.1, Wallet API 2.0.0)

## Objective

Establish, mechanically rather than by reading, that the two new SDKs cover
every operation, parameter, schema field, enum value, response header and
numeric limit the specification states — and that the two agree with each
other on the wire.

## Approach

Four passes, each a script that parses the captured specification and greps
the SDK sources, plus a differential that runs both SDKs on the same inputs
and compares their output. Grepping for the wire name (`"recordDate"`) rather
than the language-specific identifier keeps the audit independent of naming
convention: Go's `RecordDate` and Python's `record_date` both carry the same
JSON tag, and that tag is what the API sees.

## Inputs

- `wallet-api-openapi-reference.md` — 4,975 lines, the full reference.
- `wallet-by-budgetbaker-swagger-data.md` — 2,919 lines, Swagger UI capture
  with worked request/response examples. Used to confirm envelope key names
  and the `{type}` path enumerations; it is a subset of the reference.
- `go/wallet/*.go` and `python/wallet_sdk/*.py`, excluding tests.

## Steps and findings

### Pass 1 — operations and parameters

Extracted every `**Operation ID:**` and the `| name | query|path |` rows that
follow it, then checked each SDK for the operation id and each parameter's
wire name.

| | Count | Go | Python |
|---|---|---|---|
| Operations | 22 | 22 | 22 |
| Query/path parameters | 147 | 147 | 147 |

Largest surfaces: `getRecords` (21 parameters), `getBudgets` (17),
`getLabels` (14), `getAccounts` and `getCategories` (13 each).

Confirmed absences, checked deliberately because a missing parameter and an
absent one look identical from the SDK side: `getBudgets`, `getGoals`,
`getRecordRules`, `getStandingOrders` and `getStandingOrderItems` have **no**
`sortBy`; `getStandingOrderItems` has **no** `id` filter. Neither SDK invents
one.

### Pass 2 — schemas and fields

Parsed each `### \`Name\`` JSON block under *Components: schemas* and walked it
recursively, collecting every `properties` key including those nested inside
`items`, `allOf` and `anyOf`.

- 62 parseable schemas, **168 distinct properties**.
- **Zero unmapped** in either SDK.

### Pass 3 — enums, headers, limits

- **90 distinct enum values** across the document. One flagged in Go:
  `"false"`, from the `X-Sync-In-Progress` header's `["true","false"]`. Go
  parses that header with a case-insensitive compare against `"true"`, so
  `false` is handled without the literal appearing — not a gap.
- **5 documented headers** (`X-Last-Data-Change-At`, `X-Last-Data-Change-Rev`,
  `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-Sync-In-Progress`), all
  parsed into `ResponseMeta` by both.
- **14 numeric limits** stated in prose rather than schema (page limit 1–200;
  id/accountId/categoryId filter caps 30/10/10; batch caps 50 create, 10
  patch, 10 delete, 20 references, 50 limitOverrides; periodCount 1–120; name
  80 and note 255 characters; 2 range conditions; usage period
  366days/53weeks/13months) — all enforced client-side by both.

### Pass 4 — cross-SDK differential

Ran both SDKs over the same inputs and diffed the JSON: filter rendering,
amount formatting across nine magnitudes, sort directives, the range helpers,
and four request bodies (`createAccount`, `createRecords` with a
cross-currency transfer, `patchRecords` with `$clear` and an `{add, remove}`
delta, `patchBudgets` with both `IdSet` shapes and two override forms).

Result: **identical output in every group.** The only diff the first run
reported was in the harness — the snapshot keyed amounts by each language's
own `repr` of the input float, so Go's `100` and Python's `100.0` looked like
different keys for the same rendered value.

## Results

Three defects surfaced, all found by the passes rather than by the feature
tests, and all fixed:

1. **`parse_retry_after` could raise** (Python). `email.utils.parsedate_to_datetime`
   raises `ValueError` on unparseable input rather than returning `None`, so a
   garbage `Retry-After` header escaped as an exception from a header parser.
2. **`format_amount` emitted exponent notation** (Python). `repr(1e20)` is
   `'1e+20'`, which the `amount` filter cannot read. The first fix,
   `format(value, "f")`, was worse — it defaults to six decimal places and
   silently rendered `1e-7` as `0`. The shipped fix routes through `Decimal`,
   which reproduces the value exactly in plain notation.
3. **`truncate` could split a rune** (Go). It sliced by byte, so capping an
   error body at 8 KiB could leave invalid UTF-8 in a user-visible message.

Final state: Go `go vet` clean, `go test -race` passing, 94% statement
coverage. Python 699 tests passing, 99% statement coverage.

## Interpretation

Coverage of the *stated* surface is complete and now mechanically re-checkable
— the four scripts can be re-run against a newer spec capture to find drift.

Worth noting about what the passes do and do not prove. Pass 2 checks that a
field is *mentioned*, not that its type is right; type correctness rests on the
feature tests, which decode a realistic payload per schema and assert on the
values. Pass 4 compares the two SDKs to each other, so a shared
misunderstanding of the spec would pass — it catches divergence, not error.
The mock server replays what the specification says the API returns, which is
not the same as what it does return.

## Caveats

- Two documented constraints cannot be checked client-side, because they
  depend on state the SDK has not fetched: that `startDate`/`endDate`/
  `limitOverrides` suit a budget's type, and that an override `period` is not
  in the past. Both are documented on the fields and enforced by the API.
- No live API call was made at any point. Everything ran against a local mock.
- The spec capture is a markdown rendering, not the machine-readable document.
  Anything the renderer dropped is invisible to this audit.
