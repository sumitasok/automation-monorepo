# telegram pack

Reads all of your Telegram chats (read-only) via a user-account MTProto client,
summarizes the messages that arrived since the last run with the Claude API, and
sends you one digest message. Modeled on the `gmail` pack; see the workspace's
`docs/adr/0008-telegram-pack.md` for the access-model decision.

## What it does

- Authenticates as **your Telegram account** (not a bot) with `gotd/td`, so it
  can see every DM, group, and channel you're in.
- Fetches only messages **newer than the last checkpoint** per chat, so the
  digest never repeats content. The first run is bounded to a lookback window
  (default 24h).
- Groups the digest **DMs first, then groups, then channels**, and asks Claude
  to write a single concise summary.
- Sends the digest to a **configured chat** (`TELEGRAM_DIGEST_CHAT_ID`), or to
  your **Saved Messages** if none is set. Splits into multiple messages if it
  exceeds Telegram's 4096-char limit.

## Scope / non-goals

Strictly **read + one outbound digest**. The code calls only `messages.getDialogs`
and `messages.getHistory` (read) and one `messages.sendMessage` (the digest). No
editing, deleting, reacting, joining, leaving, or muting. Media is described by
type (photo, voice note, sticker…), never transcribed. No raw-message archive —
only per-chat checkpoints are persisted.

## Job

| job | command | schedule |
|-----|---------|----------|
| `telegram-summary` | `go run . summary` | daily 07:30 (disabled until configured) |

Run on demand: `auto run telegram-summary`.

## Setup

See **`RUNBOOK.md`** for the full, one-time interactive setup (API credentials
from my.telegram.org, first login, config). Quick version:

```bash
auto config init telegram          # scaffold config/telegram/config.yaml
# fill in TELEGRAM_API_ID, TELEGRAM_API_HASH, ANTHROPIC_API_KEY,
# and (optionally) TELEGRAM_DIGEST_CHAT_ID in config/telegram/config.yaml
cd packs/telegram && go mod tidy   # populate go.sum (needs network)
go run . login                     # one-time: phone + code (+ 2FA); writes session.json
go run . summary                   # test run
```

## Files

```
auth/        MTProto client + interactive login
fetch/       read-only dialog + per-chat history since checkpoint
summarize/   orders chats, calls the Claude API, writes the digest
send/        resolves destination, splits, sends the one digest message
config/      config.go (env), state.go (per-chat checkpoints)
model/       plain data types shared across packages
jobs/telegram-summary/manifest.yaml
config.sample.yaml
session.json     (git-ignored; created by `go run . login`)
state.json       (git-ignored; per-chat checkpoints)
```

## Config

Declared in `config.sample.yaml`, real values in the workspace at
`config/telegram/` (git-ignored, injected by `auto run` — ADR 0007):

| key | required | meaning |
|-----|----------|---------|
| `TELEGRAM_API_ID` | yes | integer app id from my.telegram.org |
| `TELEGRAM_API_HASH` | yes | app hash from my.telegram.org |
| `ANTHROPIC_API_KEY` | yes | Claude API key for the summary |
| `TELEGRAM_DIGEST_CHAT_ID` | no | destination chat id; empty → Saved Messages |
| `TELEGRAM_FIRST_RUN_LOOKBACK_HOURS` | no | first-run window (default 24) |
| `TELEGRAM_EXCLUDE_CHAT_IDS` | no | comma-separated chat ids to always skip |
| `TELEGRAM_SUMMARY_MODEL` | no | override the Claude model |
| `session.json` (file) | yes | MTProto session, created by `go run . login` |
