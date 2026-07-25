# Quickstart: Gmail Transactions Editor UI

## Prerequisites

- Go 1.22+ installed.
- A populated `data/gmail/transactions.csv` (this workspace already has one, ~480 rows, submodule `data/gmail`).
- From the repo root: `packs/gmail` submodule checked out (`git submodule update --init packs/gmail` if not already).

## Run it

```sh
cd packs/gmail
go run . serve
```

Expected: a log line with the URL, e.g. `serving Transactions UI at http://localhost:8090`. Open that URL in a browser.

## Validation scenarios

1. **Newest-first ordering** (User Story 1 / SC-001): On load, the top row's `TxnDate` is the most recent date present in `transactions.csv`; scrolling down, dates only decrease (or stay equal, then decrease).
2. **Edit and persist** (User Story 2 / SC-002, SC-003): Click a Category cell on any row, change its value, save. Reload the page (or `curl localhost:8090/api/transactions | grep <MessageID>`) — the new value is present. Independently, run `git -C ../../data/gmail diff transactions.csv` (or open the file) and confirm only that row's Category/Source columns changed.
3. **Reject invalid input** (SC-004): Attempt to save a Labels value that is only whitespace/separators (e.g. `"; ;"`) — expect a `422` and the stored value unchanged.
4. **Cancel discards** (FR-007): Start editing a row, cancel — value in the table and in `transactions.csv` is unchanged.
5. **Filter** (User Story 3): Enter a merchant name that matches a handful of rows — only those rows show, still newest-first. Enter one that matches nothing — an empty-state message appears, no error.
6. **Staleness** (FR-010): While the UI is open, run `packs/gmail`'s `categorize` (or manually touch `transactions.csv`'s mtime) against the same file, then try to save an edit already open in the browser — expect a `409`/refresh prompt rather than a silent overwrite.

## Contracts

See `contracts/transactions-api.md` for the full `GET /api/transactions` / `PATCH /api/transactions/{messageId}` request/response shapes.
