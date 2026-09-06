# Wallet API — Go SDK

A complete, dependency-free client for the BudgetBakers Wallet REST API
(OpenAPI 3.1, version 2.0.0). Standard library only: it builds offline and
carries no `go.sum`.

```
module github.com/sumitasok/sa.automation.wallet/sdk/go
import  "github.com/sumitasok/sa.automation.wallet/sdk/go/wallet"
```

## Install

The module lives inside this repository, so a consumer in the same tree wires
it up with a `replace`:

```
require github.com/sumitasok/sa.automation.wallet/sdk/go v0.0.0
replace github.com/sumitasok/sa.automation.wallet/sdk/go => ../sdk/go
```

## Client

```go
c, err := wallet.New(wallet.Config{
    Token:   os.Getenv("WALLET_API_TOKEN"),
    Timeout: 30 * time.Second,
    Retry:   wallet.RetryConfig{MaxRetries: 3},
})
```

`Config` also takes `BaseURL` (a path component is preserved, so a proxy
mounted at `/api/wallet` works), `HTTPClient`, `UserAgent`, extra `Header`s,
and `SkipValidation` to hand every decision to the server. `MustNew` panics
instead of returning an error, for package-level initialisation.

Every method takes a `context.Context` as its first argument. A cancelled
context short-circuits before a request is sent; a deadline and a network
fault are reported distinctly, and both stay reachable through `errors.Is`.

## Operations

| Method | Operation |
|---|---|
| `ListAccounts` `CreateAccount` `PatchAccounts` | `getAccounts` `createAccount` `patchAccounts` |
| `ListRecords` `CreateRecords` `CreateRecord` `PatchRecords` | `getRecords` `createRecords` `patchRecords` |
| `ListBudgets` `CreateBudget` `PatchBudgets` | `getBudgets` `createBudget` `patchBudgets` |
| `ListCategories` `CreateSubcategory` `PatchCategories` | `getCategories` `createCategory` `patchCategories` |
| `ListLabels` `CreateLabel` `PatchLabels` | `getLabels` `createLabel` `patchLabels` |
| `ListGoals` `ListRecordRules` `ListStandingOrders` `ListStandingOrderItems` | `getGoals` `getRecordRules` `getStandingOrders` `getStandingOrderItems` |
| `Delete` `References` `DeleteUnreferenced` | `deleteByType` `getReferences` |
| `APIUsage` | `getApiUsageStats` |

`CreateRecord` is the single-record form of `CreateRecords`; a failed item is
data, not an error, so check `result.Success` either way.
`DeleteUnreferenced` is the safe delete the API documentation asks for: it
checks references first and deletes only what nothing points at.

## Parameters and filters

List parameters are structs of pointers, so an unset field is never sent:

```go
page, err := c.ListRecords(ctx, &wallet.ListRecordsParams{
    AccountIDs:   []string{"acc1", "acc2"},
    RecordDate:   wallet.Between(from, to),           // gte. + lt.
    Amount:       []wallet.Filter{wallet.AmountFilter(wallet.OpLte, -100.50)},
    CounterParty: &counterParty,                       // wallet.Contains("Bistro")
    RecordStates: []wallet.RecordState{wallet.RecordStateReconciled},
    SortBy:       &sort,                               // wallet.Desc("recordDate")
    Limit:        wallet.Int(200),
})
```

Filters carry the prefix the API demands: `Eq` `Gt` `Gte` `Lt` `Lte`
`Contains` `ContainsI`, plus `Between`, `BetweenDates`, `OnDate` and the typed
`TimeFilter` / `DateFilter` / `AmountFilter`. `ParseFilter` reads a wire-form
string back. `Int`, `Bool`, `Float` and `String` build the pointers.

Each params struct has a `Validate()` you can call yourself to pre-flight a
payload without issuing the call.

## Pagination

```go
err := c.AllRecords(ctx, params, func(r wallet.Record) bool {
    process(r)
    return true // false stops the walk
})
```

There is an `All*` walker for every list endpoint, plus `CollectRecords` and
`CollectAccounts` for the eager case. A walk works on a copy of the params, so
the caller's struct is never mutated, and it stops rather than looping if the
server repeats or rewinds `nextOffset`.

## Batch results

```go
resp, err := c.PatchRecords(ctx, items, nil)
if err != nil { return err }          // 401, 403, 429, or a plain 400

switch {
case resp.Summary.AllSucceeded():
case resp.Summary.Partial():
    for _, r := range resp.Results {
        if !r.Success {
            log.Printf("item %d (%s): %s [%s]", r.InputIndex, r.ID, r.Error, r.ErrorType)
        }
    }
}
```

200, 207, 400 and 500 all decode into the envelope; `Meta.StatusCode` tells
them apart. `Summary` offers `Failed`, `AllSucceeded`, `AllFailed` and
`Partial`; each result offers `IsClientError` and `IsServerError`.

## Errors

```go
var apiErr *wallet.Error
switch {
case errors.Is(err, wallet.ErrRateLimited):
    time.Sleep(apiErr.RetryAfter)
case errors.Is(err, wallet.ErrForbidden):
    // The token belongs to the other app.
case errors.Is(err, wallet.ErrInvalidRequest):
    var problems wallet.InvalidRequestErrors
    errors.As(err, &problems)   // every field path that failed
}
```

Sentinels: `ErrValidation` `ErrUnauthorized` `ErrForbidden` `ErrNotFound`
`ErrRateLimited` `ErrServer` `ErrUnexpectedStatus` `ErrDecode` `ErrTransport`
`ErrInvalidRequest`. Wire failures are a `*Error` carrying `Op`, `StatusCode`,
`Message`, `RawBody`, `RetryAfter` and `Meta`; `Retryable()` classifies them.

## Response metadata

Every envelope embeds `Meta`:

```go
if page.Meta.RateLimitExhausted() { … }
if page.Meta.SyncInProgress != nil && *page.Meta.SyncInProgress { … }
rev := page.Meta.LastDataChangeRev   // compare between requests
```

`Client.LastMeta()` returns the most recent response's metadata, errors
included.

## Types

`Time` and `Date` decode leniently — null, empty, RFC 3339 at any precision,
any offset, or a bare date — and encode canonically as UTC. `Valid()` reports
whether a value was actually present. Enums are string types that accept any
value the server sends; `Valid()` checks one against the members this SDK
knows, and the request validators do exactly that before sending.

## Tests

```
go test ./... -race        # 94% of statements
```

The suite drives the client against `httptest` servers — the whole stack, not
a stubbed transport. It covers every operation's query serialisation and
decoding, every documented status including the `text/plain` 403, batch
outcomes on all four statuses, retry behaviour and backoff arithmetic,
pagination and its loop guards, context cancellation and deadlines, malformed
and empty bodies, unknown fields and unknown enum members, and the full
client-side validation matrix.
