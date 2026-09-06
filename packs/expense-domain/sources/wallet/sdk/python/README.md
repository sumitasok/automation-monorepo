# Wallet API — Python SDK

A complete, dependency-free client for the BudgetBakers Wallet REST API
(OpenAPI 3.1, version 2.0.0). Standard library only — `urllib.request` under a
pluggable transport — so it installs and runs offline.

Requires Python 3.9 or newer.

## Install

```bash
pip install -e python/            # from the repository
pip install -e 'python/[dev]'     # with pytest
```

## Client

```python
from wallet_sdk import RetryConfig, WalletClient

client = WalletClient(
    token=os.environ["WALLET_API_TOKEN"],
    timeout=30.0,
    retry=RetryConfig(max_retries=3),
)
```

It also takes `base_url` (a path component is preserved, so a proxy mounted at
`/api/wallet` works), `transport`, `user_agent`, extra `headers`, and
`validate=False` to hand every decision to the server. It works as a context
manager, and `close()` is forwarded to a transport that wants it.

## Operations

| Method | Operation |
|---|---|
| `list_accounts` `create_account` `patch_accounts` | `getAccounts` `createAccount` `patchAccounts` |
| `list_records` `create_records` `create_record` `patch_records` | `getRecords` `createRecords` `patchRecords` |
| `list_budgets` `create_budget` `patch_budgets` | `getBudgets` `createBudget` `patchBudgets` |
| `list_categories` `create_subcategory` `patch_categories` | `getCategories` `createCategory` `patchCategories` |
| `list_labels` `create_label` `patch_labels` | `getLabels` `createLabel` `patchLabels` |
| `list_goals` `list_record_rules` `list_standing_orders` `list_standing_order_items` | `getGoals` `getRecordRules` `getStandingOrders` `getStandingOrderItems` |
| `delete` `references` `delete_unreferenced` | `deleteByType` `getReferences` |
| `api_usage` | `getApiUsageStats` |

`create_record` is the single-record form of `create_records`, returning a
`(result, response)` pair; a failed item is data, not an exception, so check
`result.success` either way. `delete_unreferenced` is the safe delete the API
documentation asks for: it checks references first and deletes only what
nothing points at, returning `(response, references, skipped)`.

## Parameters and filters

Every list method takes keyword arguments; an omitted one is never sent:

```python
from wallet_sdk import FilterOp, amount_filter, between, contains, desc

page = client.list_records(
    account_ids=["acc1", "acc2"],
    record_date=between("2024-01-01T00:00:00Z", "2024-04-01T00:00:00Z"),
    amount=[amount_filter(FilterOp.LTE, -100.50)],
    counter_party=contains("Bistro"),
    record_states=[RecordState.RECONCILED],
    sort_by=desc("recordDate"),
    limit=200,
)
```

Filters carry the prefix the API demands: `eq` `gt` `gte` `lt` `lte`
`contains` `contains_i`, plus `between`, `between_dates`, `on_date` and the
typed `time_filter` / `date_filter` / `amount_filter`. `parse_filter` reads a
wire-form string back. Date arguments accept a `datetime`, a `date` or a
string — a string passes through verbatim, so a bare date stays a bare date.

## Pagination

```python
for record in client.iter_records(record_date=between(start, end)):
    process(record)
```

There is an `iter_*` generator for every list endpoint. Iteration is lazy —
nothing is fetched until the first item is pulled — so `itertools.islice` stops
the walk early. A walk works on a copy of the arguments and stops rather than
looping if the server repeats or rewinds `nextOffset`.

## Batch results

```python
response = client.patch_records(items)   # raises only on 401/403/429/plain-400

if response.summary.partial:
    for result in response.failed:
        print(result.input_index, result.id, result.error, result.error_type)
```

200, 207, 400 and 500 all decode into the envelope; `response.meta.status_code`
tells them apart. `summary` offers `failed`, `all_succeeded`, `all_failed` and
`partial`; the envelope offers `succeeded`, `failed`, `client_errors`,
`server_errors` and `result_for(input_index)`.

## Errors

```python
from wallet_sdk import (
    APIError, ForbiddenError, InvalidRequestError, RateLimitError, WalletError,
)

try:
    client.list_records(limit=500)
except InvalidRequestError as err:      # nothing was sent
    for problem in err.problems:
        print(problem.field, problem.reason)
except RateLimitError as err:
    time.sleep(err.retry_after or 60)
except ForbiddenError:
    ...                                  # the token belongs to the other app
except APIError as err:
    print(err.operation, err.status_code, err.message, err.body)
```

`WalletError` is the root. `APIError` covers everything that reached the
server: `ValidationError` `UnauthorizedError` `ForbiddenError` `NotFoundError`
`RateLimitError` `ServerError` `UnexpectedStatusError` `DecodeError`
`TransportError`, each with a `retryable` flag. `InvalidRequestError` is
deliberately *not* an `APIError` — nothing was sent — and subclasses
`ValueError` so existing argument guards keep working.

## Response metadata

Every envelope carries `meta`:

```python
if page.meta.rate_limit_exhausted: ...
if page.meta.sync_in_progress: ...
rev = page.meta.last_data_change_rev   # compare between requests
```

`client.last_meta` returns the most recent response's metadata, errors
included.

## Models

Models are dataclasses with `from_dict`, and each keeps the payload it was
built from in `raw`, so a field this SDK does not model yet is still reachable.
Enums subclass `str`, so a member compares equal to its wire form; an unknown
member round-trips verbatim rather than raising, and `is_valid()` checks a
value against what this SDK knows.

## Custom transport

```python
class MyTransport(Transport):
    def send(self, request: HTTPRequest) -> HTTPResponse:
        ...
client = WalletClient(token=..., transport=MyTransport())
```

Anything with a `send` method works — a `requests` session wrapper, a
recording proxy, or a stub in a test.

## Tests

```bash
python3 -m pytest              # 694 tests
python3 -m coverage run -m pytest && python3 -m coverage report
```

The suite drives the client against a real `ThreadingHTTPServer` over a socket
— the whole stack, not a stubbed transport. It covers every operation's query
serialisation and decoding, every documented status including the `text/plain`
403, batch outcomes on all four statuses, retry behaviour and backoff
arithmetic, lazy pagination and its loop guards, timeouts and transport
faults, malformed and empty bodies, unknown fields and unknown enum members,
type confusion in every argument, and the full client-side validation matrix.
Statement coverage is 99%.
