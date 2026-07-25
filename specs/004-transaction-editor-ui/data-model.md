# Data Model: Gmail Transactions Editor UI

## Entity: Transaction

Existing entity (`store.Record`, backed by `data/gmail/transactions.csv`) — this feature adds no new columns. It reclassifies the existing columns into read-only vs. user-editable for the purposes of this UI.

| Field | Editable via this UI? | Notes |
|---|---|---|
| MessageID | No | Stable identifier; link back to the source email. Used as the API's row key. |
| EmailDate | No | Extracted. |
| TxnDate | No | Extracted. Drives the newest-first ordering. |
| Type | No | Extracted. |
| Amount | No | Extracted. |
| Account | No | Extracted. |
| Merchant | No | Extracted. |
| Info | No | Extracted. |
| AvailableBalance | No | Extracted. |
| Subject | No | Extracted. |
| BankFrom | No | Extracted. |
| FilterName | No | Extracted. |
| FilterQuery | No | Extracted. |
| **Category** | **Yes** | Annotation. Editing it (or SubCategory/Labels) sets `Source = "user"`. |
| **SubCategory** | **Yes** | Annotation. Same `Source` rule as Category. |
| **Labels** | **Yes** | Annotation, semicolon-separated in the CSV cell (existing `labelsSep`). Same `Source` rule. |
| **Note** | **Yes** | Annotation (ADR 0013). Editing it does not touch `Source`/`CommentConsidered`. |
| Source | No (system-managed) | Read-only to the user, but written by the system: set to `"user"` as a side effect of a Category/SubCategory/Labels edit (see below). Never directly settable via the API. |
| **UserComment** | **Yes** | Annotation (spec 003). Editing it does not touch `Source`/`CommentConsidered` — the existing `NeedsReclassification()` dirty-check already reacts to a UserComment/CommentConsidered mismatch. |
| CommentConsidered | No (system-managed) | Written only by the `categorize` flow; untouched by this feature. |

## Validation rules (FR-005)

- **Category / SubCategory**: non-empty after trimming whitespace, when provided (a blank submission clears the field — see Assumptions; empty is a valid *value*, just not valid *whitespace-only* input).
- **Labels**: split on the existing `labelsSep` ("; ") once submitted as a single string from the UI; each individual label, after trimming, must be non-empty (no stray empty labels from e.g. a trailing separator).
- **Note / UserComment**: free text, no format constraint — any string, including empty (clearing the field is allowed).
- Any validation failure returns a 4xx JSON error with a field name and message; **no partial write occurs** — `SetAnnotation` is only called once all fields in a request pass validation.

## Store API addition

```go
// SetAnnotation writes the user-editable annotation fields (Category,
// SubCategory, Labels, Note, UserComment) onto the row at idx. If category,
// subCategory, or labels differ from the row's current values, Source is set
// to "user" (this call is now the decision-maker for classification); if none
// of the three changed, Source and CommentConsidered are left untouched so
// Record.NeedsReclassification()'s existing UserComment-dirty-check keeps
// working exactly as spec 003 defined it.
func (s *CSVStore) SetAnnotation(idx int, category, subCategory string, labels []string, note, userComment string) error
```

## Ordering

Transactions are sorted by `TxnDate` descending (string-lexicographic, valid because `parser.NormaliseDate` already normalises to `YYYY-MM-DD[ HH:MM:SS]`). A row with an empty `TxnDate` sorts after every row with a non-empty one, in original file order among themselves.

## API resource shape (see `contracts/` for the full contract)

```json
{
  "messageId": "…",
  "txnDate": "2026-07-20",
  "type": "debit",
  "amount": "450.00",
  "account": "…",
  "merchant": "…",
  "info": "…",
  "subject": "…",
  "bankFrom": "…",
  "category": "Groceries",
  "subCategory": "Supermarket",
  "labels": ["weekly", "essential"],
  "note": "",
  "source": "user",
  "userComment": "actually a gift, not groceries",
  "readOnly": ["messageId", "txnDate", "type", "amount", "account", "merchant", "info", "subject", "bankFrom", "source"]
}
```
