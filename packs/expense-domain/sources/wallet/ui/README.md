# Wallet UI

A read-only browser dashboard over whatever the wallet pack has written to the
external config data directory — by convention
`~/automation-monorepo-config/data/expense-domain/wallet/`.

```bash
# from packs/expense-domain/sources/wallet/
go run . ui --open
```

Then <http://127.0.0.1:8787>. Nothing is ever written back to the data
directory; every endpoint is GET-only and a write method returns 405.

## What it shows

| Tab | Reads | Shows |
|---|---|---|
| **Overview** | records | totals, spend by category, daily flow, top merchants, which files were read, plus warnings for a failed run, undecided duplicates and uncategorised records |
| **Records** | `records*.jsonl`, `records*.json`, `records/*` | sortable, filterable table; click a row for the full record including its stored JSON |
| **Duplicates** | `dedup-scan-*.json` joined to `dedup-decisions-*.json` and `*-dedup-results.json` | each duplicate group with its keep/delete decision, or marked undecided |
| **Sync & Logs** | `last-sync.json`, `logs/*.log` | last sync state, every run log classified pass/fail, with the tail of the selected run |

The filter bar applies to Overview and Records together, so the tiles and charts
always describe exactly the rows in the table. `#overview`, `#records`,
`#duplicates` and `#runs` are linkable.

## Flags

```
wallet ui [flags]
  --config-path PATH  config root (default $CONFIG_PATH, else ~/automation-monorepo-config)
  --data-dir PATH     wallet data dir (default <config-path>/data/expense-domain/wallet)
  --addr HOST:PORT    listen address (default 127.0.0.1:8787)
  --open              open the dashboard in the default browser
  --dev               serve ui/ from disk instead of the embedded copy
  --quiet             suppress per-request logging
```

Keep `--addr` on loopback. The pages carry personal finance detail and there is
no authentication in front of them.

## API

The front end is plain HTML/CSS/JS with no dependencies and no build step. It
talks to five endpoints, all GET, all `Cache-Control: no-store`:

| Endpoint | Returns |
|---|---|
| `/api/meta` | resolved data directory and endpoint list |
| `/api/records` | `{records, files, issues}` — records flattened for display, each with its `raw` payload and the file it came from |
| `/api/duplicates` | `{groups, files, issues}` — scan groups with decisions attached |
| `/api/sync` | `last-sync.json` passed through, plus its mtime |
| `/api/logs`, `/api/logs/{name}?tail=N` | run log list and one log's tail |

Records are decoded server-side with the SDK's own `wallet.Record` type
(`../sdk/go/wallet`), so the dashboard and the API client cannot drift apart on
field names. Both the JSONL and JSON-array file shapes are read; a record id
appearing in more than one file resolves to the copy with the newest
`updatedAt`, and lines that fail to parse are reported in the "Files read" panel
rather than silently dropped.

## Editing the front end

`ui/index.html`, `ui/app.js` and `ui/styles.css` are embedded into the binary at
build time. Use `--dev` (run from this pack's directory) to serve them off disk
and just reload the page; drop the flag and rebuild to bake the changes in.

Colours come from a validated palette. Money-in blue `#2a78d6` and money-out red
`#e34948` (dark: `#3987e5` / `#e66767`) clear the lightness band, chroma floor,
colour-vision-deficiency separation and 3:1 contrast checks in both modes, and
in/out is also carried by position and the legend, never by hue alone.

## Relationship to the other UI

`../index.html` is the older viewer: it pulls `records.jsonl` from a GitHub
repository using a personal access token. It is untouched and still works. This
one reads the local data directory instead and needs no credentials.
