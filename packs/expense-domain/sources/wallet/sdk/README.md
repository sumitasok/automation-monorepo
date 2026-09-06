# Wallet API SDKs

Two independent, dependency-free clients for the [BudgetBakers Wallet REST
API](https://rest.budgetbakers.com/wallet) — one in Go, one in Python — built
from the OpenAPI 3.1 specification, version 2.0.0, captured in this directory:

- [`wallet-api-openapi-reference.md`](wallet-api-openapi-reference.md) — the full reference
- [`wallet-by-budgetbaker-swagger-data.md`](wallet-by-budgetbaker-swagger-data.md) — Swagger UI capture with worked examples

```
sdk/
├── go/          module github.com/sumitasok/sa.automation.wallet/sdk/go
│   └── wallet/  package wallet
└── python/      package wallet_sdk
```

Both cover the same ground: **22 operations, 62 schemas, 168 fields, 90 enum
values, 5 response headers and 14 documented numeric limits.** Neither has a
runtime dependency outside its standard library, so both build and run offline.

## Coverage

| Tag | Operations |
|---|---|
| Banking | `getAccounts` `createAccount` `patchAccounts` `getRecords` `createRecords` `patchRecords` |
| Classification | `getCategories` `createCategory` `patchCategories` `getLabels` `createLabel` `patchLabels` `getRecordRules` |
| Planning | `getBudgets` `createBudget` `patchBudgets` `getGoals` `getStandingOrders` `getStandingOrderItems` |
| Data Management | `deleteByType` `getReferences` |
| Monitoring | `getApiUsageStats` |

## Quick start

**Go**

```go
import "github.com/sumitasok/sa.automation.wallet/sdk/go/wallet"

c, err := wallet.New(wallet.Config{Token: os.Getenv("WALLET_API_TOKEN")})
if err != nil { return err }

page, err := c.ListAccounts(ctx, &wallet.ListAccountsParams{
    Limit: wallet.Int(50),
    Name:  &[]wallet.Filter{wallet.ContainsI("checking")}[0],
})

err = c.AllRecords(ctx, &wallet.ListRecordsParams{
    RecordDate: wallet.Between(from, to),
}, func(r wallet.Record) bool {
    fmt.Println(r.RecordDate, r.Amount.Value, r.CounterParty)
    return true
})
```

**Python**

```python
from wallet_sdk import WalletClient, between, contains_i

client = WalletClient(token=os.environ["WALLET_API_TOKEN"])

page = client.list_accounts(limit=50, name=contains_i("checking"))

for record in client.iter_records(record_date=between("2024-01-01", "2024-04-01")):
    print(record.record_date, record.amount.value, record.counter_party)
```

## What both SDKs guarantee

**Every documented constraint is checked before a request is sent.** Page
limits, the 30/10/10 ID-filter ceilings, the 50-record and 10-item batch caps,
the 20-ID reference cap, string lengths counted in characters, the
`amount != 0` rule, the 24-hour future bound on `recordDate`, latitude and
longitude ranges, the mutual exclusions inside `limitOverrides`, the
`$clear`-versus-set and `reset`-versus-set conflicts, the pairing-mode
requirements on transfers, and the filter-prefix rules on every filterable
parameter. A violation names the exact field path (`records[2].amount.value`)
and every problem in the request is reported at once, so a 50-record batch
takes one round trip to fix rather than fifty.

**Batch endpoints return their envelope, not an error.** `POST /records` and
every `PATCH` and `DELETE` answer 200, 207, 400 and 500 with the same body.
Both SDKs decode all four and hand back `summary` and `results`; a 400 that is
a plain validation error is told apart by the shape of the body and raised
normally. Per-item results correlate to inputs through `inputIndex`.

**Every documented failure mode is a distinct, typed error** carrying the
status, the server's message, the raw body and the response metadata —
including the `text/plain` 403 that means the token belongs to the other app.

**Retries are safe by default.** Rate limits, 5xx and transport faults are
retried on `GET` and `DELETE` only, honouring `Retry-After` and otherwise
backing off exponentially with jitter. `POST` and `PATCH` need an explicit
opt-in, because a retried create duplicates records. A batch envelope is never
retried away: it carries per-item detail the caller needs.

**Decoding is forgiving.** Unknown fields are ignored and kept in `raw`,
unknown enum members round-trip verbatim, nulls where objects were expected
become absent values, and timestamps decode from every format the API emits.
A new server release does not break an old client.

**Pagination is a one-liner.** Every list endpoint has a walker that follows
`nextOffset` and stops rather than looping if the server repeats or rewinds it.

## Two API behaviours worth knowing

**The implicit record window.** `GET /records` with no `recordDate` bound
scopes results to the last three months and reports the filter it applied in
`appliedRecordDateFilters`. Any single bound overrides it; an ID lookup
bypasses it. Both SDKs surface the applied filter and a
`DefaultWindowApplied` / `default_window_applied` helper.

**`documentsWritten` is not `succeeded`.** On delete a repeated ID is
idempotent, so a retried delete of an already-gone document succeeds while
writing nothing. On create and patch the count can exceed `total`, because an
auto-created transfer mirror or a propagation to a mirror is a document
without being an item.

## Tests

Both suites drive the SDK against a real HTTP server over a socket — the whole
stack, not a stubbed transport, so a serialisation bug cannot hide behind a
mock.

```
cd go     && go test ./... -race     # 94% statement coverage
cd python && python3 -m pytest       # 694 tests, 99% statement coverage
```

Per-SDK detail is in [`go/README.md`](go/README.md) and
[`python/README.md`](python/README.md).

## Authentication

Both take a bearer JWT from
<https://web.budgetbakers.com/settings/rest-api> (Premium plan). Tokens carry
the flavour of the login that minted them: a Wallet token works on
`/wallet/...`, a Board token on `/board/...`. Using one on the other yields
403, and re-authenticating against the same app will not help.

Keep the token in the environment or in `~/automation-monorepo-config/`, never
in this repository.
