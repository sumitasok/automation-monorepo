# ADR 0008 — Telegram pack: user-account MTProto access + session-credential handling

**Status:** accepted — 2026-07-05

## Context

Mounting the `telegram` pack introduces an access pattern the workspace has not
used before. The gmail pack (ADR 0006) authenticates with Google OAuth against a
first-party API with a narrowly-scoped, revocable token. Telegram is different:
to read *all* of Sumit's chats (1:1 DMs, groups, channels) we cannot use the Bot
API — a bot only sees messages sent to it or in groups it joins, so it can never
cover personal DMs. Reading everything requires authenticating **as Sumit's own
user account** over MTProto (via `gotd/td`), which:

- produces a **long-lived session credential** (`session.json`) that is
  effectively full access to the account — far broader than an OAuth scope;
- is created by an **interactive login** (phone number + one-time code, plus 2FA
  password if set), not a headless key exchange;
- has **no server-enforced read-only scope**: the same session that reads could,
  in principle, also send, edit, delete, or leave chats.

Per the workspace's ADR-driven model (every decision must be documented), a new
access pattern that holds a live account credential gets its own ADR before the
pack is used.

## Decision

1. **User-account MTProto, via `gotd/td` (pure Go).** The pack stays a
   single-language Go app (consistent with gmail), no Python/telethon helper.
   Rejected alternative: a `telethon` subprocess — more mature MTProto but makes
   this a mixed-language pack with a second runtime to manage.

2. **Least privilege enforced in code, not by the API.** Because MTProto has no
   read-only scope, the restriction is a code-level invariant: the pack calls
   **only** `messages.getDialogs` and `messages.getHistory` (read) and exactly
   one `messages.sendMessage` (the digest). No edit/delete/react, no
   join/leave/mute, no settings access. This invariant is stated in package docs
   (`fetch`, `send`) and must be preserved in review — adding any write call
   beyond the single digest send requires a follow-up ADR.

3. **The session file is a secret handled like gmail's `token.json` (ADR 0007).**
   - Declared in `config.sample.yaml` under `files:` as `session.json`.
   - `.gitignore`d in the pack repo so it is never committed upstream.
   - The real file lives only in the workspace at `config/telegram/` (git-ignored)
     and is symlinked into the app workdir by `auto run` at runtime.
   - It is produced once by `go run . login` (interactive) and reused thereafter.
   - The API ID/hash (from my.telegram.org) and the Claude API key are injected
     as env vars, same contract as every other pack.

4. **No raw-message archive by default (ADR 0005 alignment).** The pack persists
   only per-chat checkpoints (`state.json`, git-ignored, local) — the highest
   message ID already summarized per chat. Fetched message bodies are held in
   memory for the run, sent to the Claude API for summarization, and discarded.
   Nothing but checkpoints and the LLM's summary survives a run.

5. **Incremental, bounded scope.** Only messages newer than each chat's
   checkpoint are fetched (`min_id`). The first run (no checkpoint) is bounded to
   a configurable lookback window (default 24h) so it can never pull full history.

## Settings chosen for the first instance

- **Digest destination:** a dedicated chat/channel (`TELEGRAM_DIGEST_CHAT_ID`);
  falls back to Saved Messages when unset.
- **First-run lookback:** 24 hours.
- **Ordering:** grouped by chat, DMs first, then groups, then channels.
- **Exclusions:** none by default; `TELEGRAM_EXCLUDE_CHAT_IDS` supports opting
  specific chats out.
- **Retention:** raw messages discarded after summarization; only checkpoints
  kept.

## Consequences

- The workspace now holds (in `config/telegram/`, git-ignored) a credential that
  is broader than any prior pack's. Losing control of `session.json` means
  account access; it must be protected like a password and can be revoked from
  Telegram → Settings → Devices (Active Sessions).
- Read-only-ness is a **discipline enforced in review**, not a platform
  guarantee. The single-`sendMessage` rule is the line; crossing it needs a new
  ADR.
- Visibility is `private` for this instance (holds a live session + personal
  message content). The *code* could later be shared/public like gmail, with
  each user supplying their own session and config locally (ADR 0006/0007).
- Complements ADR 0005 (produced data local), 0006 (apps as packs), 0007 (config
  injection). Follows the same shape as gmail; the delta captured here is the
  account-level credential and the code-enforced least-privilege scope.
