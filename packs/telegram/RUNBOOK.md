# RUNBOOK — telegram pack

How to set up the Telegram daily-digest service from scratch. This is a
**one-time interactive setup**; after it, the `telegram-summary` job runs
unattended on a schedule.

Access model and secret handling are documented in the workspace at
`docs/adr/0008-telegram-pack.md`. Read that first if you want the "why".

---

## 0. Prerequisites

- Go 1.22+ installed on the machine that will run the job (the always-on
  `home-server` in `machines.yaml`).
- Network access on that machine (Telegram's data centres + `api.anthropic.com`).
- Your phone with the Telegram app installed and logged in (to receive the
  one-time login code).
- A Claude API key.

---

## 1. Get your Telegram API ID and API hash (my.telegram.org)

These identify the **client library**, not your account. They are required for
any MTProto client and are distinct from a bot token.

1. Go to **https://my.telegram.org** and log in with your phone number (you'll
   get a code in the Telegram app).
2. Click **API development tools**.
3. Fill the short form:
   - **App title:** e.g. `sa-automation-telegram`
   - **Short name:** e.g. `sa-tg`
   - **Platform:** Desktop
   - URL/description can be left blank.
4. Click **Create application**.
5. Copy the **`App api_id`** (an integer) and **`App api_hash`** (a 32-char hex
   string). Treat the hash like a secret.

> You do this once. The same api_id/api_hash are reused forever.

---

## 2. Scaffold and fill the workspace config

From the workspace root:

```bash
auto config init telegram
```

This creates `config/telegram/config.yaml` (git-ignored) from
`packs/telegram/config.sample.yaml`. Open it and fill in:

```yaml
env:
  TELEGRAM_API_ID: "1234567"                 # from step 1
  TELEGRAM_API_HASH: "0123456789abcdef0123456789abcdef"
  ANTHROPIC_API_KEY: "sk-ant-..."
  TELEGRAM_DIGEST_CHAT_ID: ""                # set in step 5, or leave empty for Saved Messages
  TELEGRAM_FIRST_RUN_LOOKBACK_HOURS: "24"
  TELEGRAM_EXCLUDE_CHAT_IDS: ""              # optional, e.g. "777000,42777"
```

Check what's set vs. missing at any time:

```bash
auto config telegram
```

---

## 3. Build dependencies (populate go.sum)

`go.mod` pins `gotd/td`, but `go.sum` and the indirect dependency graph must be
generated on a machine with network access:

```bash
cd packs/telegram
go mod tidy
go build ./...        # sanity check that it compiles
```

Commit the resulting `go.sum` (it contains no secrets).

---

## 4. First interactive login (creates session.json)

This is the step that turns your account credentials into a reusable session
file. It must be run **interactively** once — it prompts for a code Telegram
sends to your phone.

```bash
# still in packs/telegram
go run . login
```

You'll be prompted for:

1. **Phone number** — international format, e.g. `+919812345678`.
2. **Login code** — Telegram sends this to your Telegram app (not SMS, usually).
   Enter it.
3. **2FA password** — only if you have two-step verification enabled. Leave blank
   if not.

On success it prints your name and account id and writes **`session.json`** in
the current directory.

### Put the session where `auto run` expects it

`auto run` injects config by symlinking declared secret files from
`config/telegram/` into the job workdir (ADR 0007). Move the session there so it
survives submodule re-clones and is picked up on scheduled runs:

```bash
mv session.json ../../config/telegram/session.json
```

> **Security:** `session.json` is effectively full access to your Telegram
> account. It is git-ignored in both the pack and the workspace and must never be
> committed or shared. You can revoke it any time from
> **Telegram → Settings → Devices → Active sessions** (kill the session named
> after your app), then re-run `go run . login` to make a new one.

---

## 5. (Optional) Choose a dedicated digest chat

You chose to deliver the digest to a **dedicated chat/channel** rather than Saved
Messages. To set that up:

1. In Telegram, create a private channel (or group) for your digests — e.g.
   "My Telegram Digest". Make sure your account is a member.
2. Find its chat id. Easiest: forward one of its messages to `@userinfobot`, or
   `@RawDataBot`, which reports the chat id. For a channel it looks like
   `-1001234567890`.
3. Put that id in `config/telegram/config.yaml` as `TELEGRAM_DIGEST_CHAT_ID`.
   The pack accepts the `-100…` form directly.

Leave `TELEGRAM_DIGEST_CHAT_ID` empty to send to your own **Saved Messages**
instead.

The destination chat must appear in your recent dialog list (it will, once it has
any message and you're a member) so the sender can resolve it.

---

## 6. Test run

```bash
cd packs/telegram
go run . summary
```

Expected: it logs how many new messages it found across how many chats, then
sends one digest message to your destination. The first run summarizes the last
24h (per `TELEGRAM_FIRST_RUN_LOOKBACK_HOURS`) and writes `state.json` with per-chat
checkpoints. Subsequent runs only cover new messages.

Move the state file alongside the session so scheduled runs keep it:

```bash
# state.json is written in the workdir; it's local runtime state (ADR 0005).
# On the home-server it will simply persist in the pack dir between runs.
```

Run it through the framework instead of raw `go run` to get logging/timeout/
history:

```bash
auto run telegram-summary
```

---

## 7. Enable the schedule

The job manifest (`jobs/telegram-summary/manifest.yaml`) ships with:

```yaml
schedule:
  cron: "30 7 * * *"
  timezone: Asia/Kolkata
  enabled: false
```

Flip `enabled: true`, then sync schedules to the OS (cron/launchd/Task Scheduler):

```bash
auto schedule sync
```

The job is pinned to `machines: [home-server]` because it must not miss a run and
holds the session. Confirm it lands in the catalog:

```bash
auto catalog
auto doctor          # should pass: exec-based app job, private visibility in a private pack
```

---

## 8. Day-to-day operations

| Task | Command |
|------|---------|
| Run the digest now | `auto run telegram-summary` |
| See config status | `auto config telegram` |
| Re-summarize a wider first window | set `TELEGRAM_FIRST_RUN_LOOKBACK_HOURS`, delete `state.json`, run |
| Reset all checkpoints | delete `packs/telegram/state.json` (next run treats every chat as first-run) |
| Exclude noisy chats | add ids to `TELEGRAM_EXCLUDE_CHAT_IDS` |
| Rotate/revoke access | kill the session in Telegram → Devices, then `go run . login` again |

---

## 9. Troubleshooting

- **`not authorized — run go run . login`** — the session is missing or expired
  (or wasn't symlinked into the workdir). Re-run login and ensure
  `config/telegram/session.json` exists; `auto run` symlinks it in.
- **`TELEGRAM_API_ID not set` / `...HASH not set`** — config not filled or not
  injected. Run `auto config telegram` to see what's missing.
- **`digest chat_id … not found in your dialogs`** — the destination chat isn't
  in your recent dialogs, or the id is wrong. Verify with `@userinfobot`, ensure
  you're a member, or leave the id empty to use Saved Messages.
- **Digest split into `(1/2)`, `(2/2)`** — normal; content exceeded 4096 chars.
- **LLM fallback message ("automatic summary unavailable")** — the Claude call
  failed (bad key, rate limit, network). The run still delivered per-chat counts
  and advanced checkpoints. Check `ANTHROPIC_API_KEY` and re-run.
- **`FLOOD_WAIT_x` from Telegram** — you hit a rate limit; wait the indicated
  seconds. The daily cadence should not trigger this in normal use.

---

## 10. What is and isn't stored

- **Stored (local, git-ignored):** `session.json` (credential) and `state.json`
  (per-chat last-seen message ids).
- **Not stored:** raw message contents. They're held in memory for the run, sent
  to the Claude API to write the digest, and discarded. Only the checkpoints and
  the digest message you received remain. (ADR 0008 §4.)
