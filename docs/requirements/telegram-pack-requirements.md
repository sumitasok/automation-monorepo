# Requirements: `telegram` pack — daily message digest

## 1. Purpose

A new pack, `telegram`, that reads all of Sumit's Telegram chats (read-only),
generates a single LLM-written summary of new messages since the last run,
and delivers that summary as one Telegram message back to Sumit.

Modeled on the existing `gmail` pack: an app-backed Go pack mounted as its own
submodule, exposing one subcommand as a job.

## 2. Access model

- **Client type**: user-account login via MTProto (not the Bot API). The Bot
  API only sees messages sent directly to a bot or in groups the bot joins,
  which cannot cover all personal DMs. A user-account client (e.g. Go's
  `gotd/td`, or calling out to a `telethon`-based helper) authenticates as
  Sumit's real Telegram account and can read every chat, group, and channel
  he's a member of.
- **Auth**: phone number + one-time login code (and 2FA password if enabled),
  performed once interactively to produce a session file. The session file
  is the long-lived credential; treat it like `packs/gmail/token.json` —
  git-ignored, never committed, never shared.
- **Permission scope**: strictly read (fetch messages) + send (the one digest
  message). No editing, deleting, or reacting to any message; no joining or
  leaving chats; no reading/writing chat settings.
- **Delivery of the digest**: the same authenticated client sends itself
  (Saved Messages) or a designated chat_id one summary message per run.
  Needs confirmation: send to "Saved Messages" vs. a specific chat/bot —
  see Open Questions.

## 3. Scope of messages summarized

- All chats visible to the account: 1:1 DMs, groups, channels — no allowlist,
  no exclusions by default.
- Only messages received since the last successful run are summarized (see
  state tracking below), so the digest never repeats content.
- Media messages (photos, voice notes, stickers, files) are represented by
  type + sender + chat in the summary, not transcribed — text/caption content
  is summarized normally.

## 4. State tracking

- Per chat, persist the last-seen message ID (or timestamp) so each run only
  fetches messages newer than the last checkpoint — same pattern as gmail's
  `discoverstate.go` / `config/state.go`.
- State lives under `data/state/` (the workspace's existing git-synced SQLite
  store) or a pack-local state file, consistent with `packs/gmail/config/state.go`.
- First run needs a defined lookback window (e.g. last 24h) since there's no
  prior checkpoint — see Open Questions.

## 5. Summarization

- New messages collected since last checkpoint are batched and sent to an
  LLM (Claude API) to produce one concise natural-language digest, grouped
  by chat, in the order: unread-priority chats first (DMs), then groups,
  then channels — or simply chronological, TBD.
- Digest should stay within a single Telegram message length limit (4096
  chars) — if content exceeds that, the job should split into multiple
  messages or truncate with a "+N more" note.

## 6. Trigger / scheduling

- Runs on a schedule (default: once every morning) via the workspace's
  existing `schedules/` + `auto schedule sync` mechanism.
- Also runnable on-demand via `auto run telegram-summary`.

## 7. Pack structure (proposed)

```
packs/telegram/
  pack.yaml                # name, description, default_visibility: private
  go.mod / go.sum
  auth/                    # MTProto login flow, session file handling
  fetch/                   # per-chat message fetch since checkpoint
  summarize/               # batches messages, calls LLM API
  send/                    # sends the digest message
  config/
    config.go              # loads secrets (API ID/hash, session path, LLM key)
    state.go                # per-chat checkpoint persistence
  config.sample.yaml
  jobs/
    telegram-summary/
      manifest.yaml        # wraps the Go binary as a job, sets schedule
  session.json             # git-ignored, produced by interactive login
  README.md
  RUNBOOK.md               # how to do the first interactive login
```

Registered in the workspace's `packs.yaml` as its own submodule (`writable:
true`, `default_visibility: private`), same as `gmail`.

## 8. Secrets / config needed

- Telegram API ID + API hash (from my.telegram.org, required for MTProto
  client libraries — distinct from a bot token).
- Session file path (produced by first interactive login).
- LLM API key (Claude API) for summarization.
- Target chat for the digest (Saved Messages or specific chat_id).

All via `auto config init telegram` / `config/` (git-ignored), matching the
gmail pack's config pattern.

## 9. Non-goals

- No writing/editing/deleting of any message.
- No joining, leaving, or muting chats.
- No two-way chat automation (replying to people, bots commands, etc.) —
  this pack only reads and sends one outbound digest.
- No permanent raw-message archive by default (only checkpoints + whatever
  the LLM summary retains) — unless Sumit wants a searchable history later.

## 10. Governance

- Per the workspace's ADR-driven model, mounting a new pack that introduces
  a new access pattern (MTProto user-account login, holding a live session
  credential) should get its own ADR, similar to `docs/adr/0006-applications-as-packs.md`
  for the gmail pack. Draft an ADR (e.g. `0008-telegram-pack.md`) covering
  the access-model decision and secret handling before implementation.

## 11. Open questions (need Sumit's decision before build)

1. **Digest destination**: send to "Saved Messages", or a dedicated
   chat/channel created for digests?
2. **First-run lookback window**: how far back should the very first run
   summarize (e.g. last 24h, last 7 days, or nothing until the first
   checkpoint is set)?
3. **Chat exclusions**: any chats that should always be excluded (e.g. spam
   channels, bot chats) even though default scope is "all chats"?
4. **Digest ordering/grouping**: chronological vs. grouped-by-chat vs.
   priority (DMs first)?
5. **Retention**: should raw fetched messages be discarded immediately after
   summarization, or kept for some period for debugging/audit?
6. **MTProto library choice**: pure-Go (`gotd/td`) vs. shelling out to a
   Python `telethon` helper process — affects whether this stays a pure Go
   pack or becomes a mixed-language pack.
