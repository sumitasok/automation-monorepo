# Contract: Transactions UI HTTP API

Local-only HTTP API served by the new `packs/gmail serve` subcommand (default `http://localhost:8090`, overridable via a flag — see `quickstart.md`). No authentication (single-user local tool, per spec Assumptions).

## `GET /`

Serves the tab-shell HTML page (server-rendered initial Transactions table via `html/template`, then client-side JS takes over for editing/filtering).

## `GET /api/transactions`

Lists all transactions, sorted by `TxnDate` descending (see `research.md` §6), optionally filtered.

**Query parameters** (all optional, combinable):
- `merchant` — case-insensitive substring match against `Merchant`.
- `category` — case-insensitive substring match against `Category`.
- `from`, `to` — `YYYY-MM-DD`, inclusive bounds on `TxnDate`.

**Response** `200 OK`:
```json
{
  "loadedAt": "2026-07-25T10:00:00Z",
  "transactions": [ /* Transaction resource objects, see data-model.md */ ]
}
```
`loadedAt` is an opaque token the client must echo back on `PATCH` (FR-010 staleness check) — see below. An empty `transactions` array (no data, or a filter with no matches) is a normal `200`, not an error (FR-009).

## `PATCH /api/transactions/{messageId}`

Edits the annotation fields of one transaction.

**Request body**:
```json
{
  "loadedAt": "2026-07-25T10:00:00Z",
  "category": "Groceries",
  "subCategory": "Supermarket",
  "labels": ["weekly", "essential"],
  "note": "",
  "userComment": "actually a gift, not groceries"
}
```
All five fields are required in the body (the client always submits the full annotation set for that row, not a partial patch), matching `SetAnnotation`'s signature.

**Responses**:
- `200 OK` — `{ "transaction": { /* updated resource */ }, "loadedAt": "<new token>" }`. The new `loadedAt` must be used for the row's next edit.
- `409 Conflict` — the file on disk changed since `loadedAt` was issued (FR-010). Body: `{ "error": "stale", "message": "Transaction data changed on disk — refresh and try again." }`. No write is performed.
- `422 Unprocessable Entity` — validation failed (FR-005). Body: `{ "error": "validation", "field": "labels", "message": "…" }`. No write is performed.
- `404 Not Found` — no transaction with that `MessageID`.

## Staleness token (`loadedAt`)

Implemented as the `transactions.csv` file's modification time (RFC3339), captured when the server last loaded the file into its in-memory `CSVStore`. A `PATCH` is only applied if the server's current in-memory `loadedAt` still matches the value the client last saw; otherwise the server first reloads from disk and returns `409` so the client can refetch and retry. This satisfies FR-010 without any new on-disk locking.
