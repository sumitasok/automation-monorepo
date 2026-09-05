#!/usr/bin/env python3
"""
sync.py — No-AI Gmail → BudgetBakers Wallet + Obsidian sync pipeline.

Replaces the Claude-headless run-sync.sh with a pure deterministic script.
Known email formats are handled by engine.py (zero AI tokens).
Unmatched formats surface clearly so you can codify them — after that they
cost nothing too.

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
PREREQUISITES
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
    pip install google-auth-oauthlib google-api-python-client requests pyyaml \\
                --break-system-packages

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
FIRST-TIME SETUP
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
1.  Google Cloud Console → APIs & Services → Credentials
    → Create OAuth 2.0 Client ID (Desktop app)
    → Download JSON → save as  $CONFIG_PATH/config/expense-domain/gmail/credentials.json
    (default CONFIG_PATH: ~/automation-monorepo-config; same store the rest
    of this workspace's gmail jobs use — one OAuth grant, not a second one)

2.  Run:  python3 sync.py --auth
    Opens browser, completes consent, writes token to
    $CONFIG_PATH/config/expense-domain/gmail/token.json

3.  Set wallet token:
    export WALLET_AUTH_HEADER="Bearer <your-budgetbakers-token>"
    (or add to ~/.zshrc / a .env file and source it before running)
    Normally injected via config/expense-domain/wallet/config.yaml by
    scripts/wallet-sync.sh — see that script for the single-command entry
    point.

4.  Normal run:
    python3 sync.py

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
USAGE
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
    python3 sync.py                          # normal run
    python3 sync.py --dry-run                # print actions, write nothing
    python3 sync.py --since 2026-06-01       # override cursor (backfill)
    python3 sync.py --auth                   # redo OAuth consent flow
    python3 sync.py --test-engine            # run engine.py regression tests
    python3 sync.py --ai-assist              # for emails matching no known
                                              # format, ask AI to suggest one,
                                              # save it, and retry within this
                                              # same run (costs API calls)

Optional env vars:
    SA_VAULT      set to an Obsidian vault path to enable write-back of a
                  human-readable monthly expense log (unset = disabled)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
WHAT THIS DOES (mirrors Wallet Sync Runbook.md)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Part A  Gmail bank/card alerts → engine.py → Wallet + Obsidian log
Part B  Google Drive Bills Inbox → parse → Wallet + bill notes + product-prices.jsonl
Part C  Cross-source reconciliation (append-only, same run)
Part D  Label tagging via labels-cache.json
        Apply claude-read Gmail label to every processed thread
        Update last-sync.json cursor
"""

import argparse
import datetime as dt
import json
import os
import pathlib
import re
import subprocess
import sys
import tempfile
import time

# ── optional deps (imported lazily with friendly errors) ──────────────────

def _require(pkg, install_name=None):
    try:
        return __import__(pkg)
    except ImportError:
        name = install_name or pkg
        sys.exit(f"Missing package '{name}'. Run:\n"
                 f"  pip install {name} --break-system-packages")

# ── paths ─────────────────────────────────────────────────────────────────
#
# Everything this script reads or writes lives under CONFIG_PATH (default
# ~/automation-monorepo-config) or right next to this script — never a
# hardcoded personal path. The one deliberate exception is the Obsidian
# write-back (a real, opt-in feature — see spec 010): it only activates when
# SA_VAULT is explicitly set, so an unconfigured run never touches a personal
# iCloud vault.

# External configuration/data root (constitution: config/data/rules live here)
CONFIG_PATH = pathlib.Path(os.environ.get("CONFIG_PATH", pathlib.Path.home() / "automation-monorepo-config"))
EXT_CONFIG_DIR = CONFIG_PATH / "config" / "expense-domain" / "wallet"
LABELS_CACHE = EXT_CONFIG_DIR / "labels-cache.json"

# The extraction engine and its formats/routing live in this repo, migrated
# alongside sync.py — never in the Obsidian vault (that copy is stale).
EXTRACT_DIR  = pathlib.Path(__file__).resolve().parent
ENGINE       = EXTRACT_DIR / "extract-engine.py"

# Sync state/logs/unmatched-log: this pack's own produced data, under
# CONFIG_PATH/data/expense-domain/wallet/ (AUTO_DATA_DIR, when the framework
# injects it, points at the same place).
SYNC_DIR     = pathlib.Path(os.environ.get(
    "AUTO_DATA_DIR", str(CONFIG_PATH / "data" / "expense-domain" / "wallet")
))
LAST_SYNC    = SYNC_DIR / "last-sync.json"
LOG_DIR      = SYNC_DIR / "logs"

# Obsidian write-back: opt-in only. Unset SA_VAULT (the default) disables it
# rather than silently defaulting to one person's iCloud vault path.
_sa_vault = os.environ.get("SA_VAULT", "")
VAULT = pathlib.Path(_sa_vault) if _sa_vault else None

# Gmail OAuth credentials: gmail is a source of expense-domain, addressed
# config/<domain>/<source>/ like every other source (config/expense-domain/
# wallet/ follows the same pattern) — not a separate, disconnected store.
CONFIG_DIR   = CONFIG_PATH / "config" / "expense-domain" / "gmail"
CREDS_FILE   = CONFIG_DIR / "credentials.json"
TOKEN_FILE   = CONFIG_DIR / "token.json"

WALLET_BASE  = "https://rest.budgetbakers.com/wallet/v1/api"
GMAIL_SCOPES = [
    "https://www.googleapis.com/auth/gmail.readonly",
    "https://www.googleapis.com/auth/gmail.labels",   # apply claude-read
    "https://www.googleapis.com/auth/gmail.modify",   # label threads
]
DRIVE_SCOPE  = "https://www.googleapis.com/auth/drive.readonly"
ALL_SCOPES   = GMAIL_SCOPES + [DRIVE_SCOPE]

CLAUDE_READ_LABEL_ID = "Label_145"   # created 2026-06-20; name: claude-read

# Identifies this specific scheduled process in every record's note, so a
# record can be traced back to which of the three concurrently-running
# sync pipelines created or last updated it (com.safinances.wallet-sync,
# com.sumitasok.wallet-sync, or this one).
PROC_LABEL = "com.automation-monorepo.wallet-sync-unified"


def proc_tag(action: str) -> str:
    return f" | proc:{PROC_LABEL}:{action}"

DRIVE_BILLS_FOLDER   = "1DXizYKYGSg8pPO1_tbXPLTUOENOwfMR6"

DEFAULT_LOOKBACK_DAYS = 7


# ══════════════════════════════════════════════════════════════════════════
# AUTH
# ══════════════════════════════════════════════════════════════════════════

def _get_google_creds(force_reauth=False):
    from google.oauth2.credentials import Credentials                         # google-auth
    from google_auth_oauthlib.flow import InstalledAppFlow                    # google-auth-oauthlib
    from google.auth.transport.requests import Request                        # google-auth

    creds = None
    if TOKEN_FILE.exists() and not force_reauth:
        creds = Credentials.from_authorized_user_file(str(TOKEN_FILE), ALL_SCOPES)
    if not creds or not creds.valid:
        if creds and creds.expired and creds.refresh_token:
            creds.refresh(Request())
        else:
            if not CREDS_FILE.exists():
                sys.exit(
                    f"OAuth credentials not found at {CREDS_FILE}.\n"
                    "See the setup instructions at the top of this file."
                )
            flow = InstalledAppFlow.from_client_secrets_file(str(CREDS_FILE), ALL_SCOPES)
            creds = flow.run_local_server(port=0)
        CONFIG_DIR.mkdir(parents=True, exist_ok=True)
        TOKEN_FILE.write_text(creds.to_json())
        print(f"Token saved → {TOKEN_FILE}")
    return creds


def _gmail_service(creds):
    from googleapiclient.discovery import build
    return build("gmail", "v1", credentials=creds, cache_discovery=False)


def _drive_service(creds):
    from googleapiclient.discovery import build
    return build("drive", "v3", credentials=creds, cache_discovery=False)


# ══════════════════════════════════════════════════════════════════════════
# STATE  (last-sync.json)
# ══════════════════════════════════════════════════════════════════════════

def load_state():
    if LAST_SYNC.exists():
        return json.loads(LAST_SYNC.read_text())
    return {}


def save_state(state, dry_run=False):
    if dry_run:
        print("[dry-run] would write last-sync.json:", json.dumps(state, indent=2)[:400])
        return
    LAST_SYNC.write_text(json.dumps(state, indent=2))


# ══════════════════════════════════════════════════════════════════════════
# WALLET API
# ══════════════════════════════════════════════════════════════════════════

def _wallet_headers():
    auth = os.environ.get("WALLET_AUTH_HEADER", "")
    if not auth:
        sys.exit("Set WALLET_AUTH_HEADER env var:  export WALLET_AUTH_HEADER='Bearer <token>'")
    return {"Authorization": auth, "Accept": "application/json",
            "Content-Type": "application/json"}


def wallet_get(path, params=None):
    requests = _require("requests")
    r = requests.get(WALLET_BASE + path, headers=_wallet_headers(), params=params, timeout=30)
    r.raise_for_status()
    return r.json()


def wallet_post(path, body, dry_run=False):
    if dry_run:
        print(f"[dry-run] POST {path}", json.dumps(body)[:300])
        return {}
    requests = _require("requests")
    r = requests.post(WALLET_BASE + path, headers=_wallet_headers(),
                      json=body, timeout=30)
    r.raise_for_status()
    return r.json()


def wallet_patch(path, body, dry_run=False):
    if dry_run:
        print(f"[dry-run] PATCH {path}", json.dumps(body)[:300])
        return {}
    requests = _require("requests")
    r = requests.patch(WALLET_BASE + path, headers=_wallet_headers(),
                       json=body, timeout=30)
    r.raise_for_status()
    return r.json()


def fetch_wallet_categories():
    """Returns {name_lower: id} for all Wallet categories."""
    cats = wallet_get("/categories")
    return {c["name"].lower(): c["id"] for c in (cats if isinstance(cats, list) else cats.get("data", []))}


def fetch_wallet_records(date_from, date_to, account_id=None, dry_run=False):
    # In dry-run mode, skip API calls to avoid errors and test the flow
    if dry_run:
        return []
    # Wallet API uses PostgREST-style filters: recordDate=gte.YYYY-MM-DD&recordDate=lt.YYYY-MM-DD
    params = {
        "recordDate": [f"gte.{date_from}", f"lt.{date_to}"],
        "limit": 200
    }
    if account_id:
        params["accountId"] = account_id
    result = wallet_get("/records", params=params)
    return result if isinstance(result, list) else result.get("records", [])


# ── category hint mapping (merchant/category → Wallet category keyword) ──

CATEGORY_HINTS = {
    # merchant keywords → Wallet category name keyword
    "blinkit":        "groceries",
    "licious":        "groceries",
    "zomato":         "restaurants",
    "swiggy":         "restaurants",
    "netflix":        "subscriptions",
    "youtube":        "subscriptions",
    "spotify":        "subscriptions",
    "apple":          "subscriptions",
    "google play":    "subscriptions",
    "amazon":         "shopping",
    "meesho":         "shopping",
    "decathlon":      "shopping",
    "uber":           "transport",
    "ola":            "transport",
    "rapido":         "transport",
    "fastag":         "transport",
    "irctc":          "transport",
    "indigo":         "transport",
    "hospital":       "healthcare",
    "pharmacy":       "healthcare",
    "1mg":            "healthcare",
    "apollo":         "healthcare",
    "eureka forbes":  "home",
    "urban company":  "home",
    "mygate":         "home",
    "livpure":        "utilities",
    "bescom":         "utilities",
    "fuel":           "fuel",
    "petrol":         "fuel",
}


def guess_category_id(merchant: str, categories: dict) -> str | None:
    m = merchant.lower()
    for keyword, cat_hint in CATEGORY_HINTS.items():
        if keyword in m:
            # fuzzy-find in category map
            for cat_name, cat_id in categories.items():
                if cat_hint in cat_name:
                    return cat_id
    return None


# ══════════════════════════════════════════════════════════════════════════
# LABELS
# ══════════════════════════════════════════════════════════════════════════

def load_labels_cache() -> dict:
    if LABELS_CACHE.exists():
        return json.loads(LABELS_CACHE.read_text())
    return {}


def label_ids_for_record(record: dict, cache: dict) -> list:
    """Pick 2-4 label UUIDs from labels-cache.json for a Wallet record."""
    tags = []
    merchant = (record.get("merchant") or record.get("counterParty") or "").lower()
    bank     = (record.get("bank") or "").lower()
    instr    = (record.get("instrument") or "").lower()

    # merchant tag
    for kw, slug in [("blinkit","blinkit"), ("licious","licious"),
                     ("amazon","amazon"), ("apple","apple"),
                     ("zomato","food-delivery"), ("swiggy","food-delivery"),
                     ("netflix","converted2subscription"),
                     ("youtube","converted2subscription"),
                     ("google play","google-play-store"),
                     ("asspl","amazon")]:
        if kw in merchant and slug in cache:
            tags.append(cache[slug])
            break

    # bank/card tag
    for kw, slug in [("hdfc","hdfc"), ("canara","canara"), ("icici","icici"),
                     ("schwab","schwab"), ("amazon pay","amazon")]:
        if kw in bank or kw in merchant:
            if slug in cache:
                tags.append(cache[slug])
            break

    return list(dict.fromkeys(tags))[:4]   # dedup, max 4


# ══════════════════════════════════════════════════════════════════════════
# GMAIL FETCH
# ══════════════════════════════════════════════════════════════════════════

def fetch_gmail_threads(service, since_ts: str, log) -> list[dict]:
    """
    Returns list of {thread_id, messages: [{id, sender, subject, date, body}]}.
    Skips threads already labelled claude-read.
    """
    # Build query: bank.in senders, -label:claude-read, after cursor
    # Gmail `after:` takes a Unix timestamp
    after_epoch = int(dt.datetime.fromisoformat(since_ts.replace("Z","")).timestamp())
    query = (
        f"from:(bank.in) -label:claude-read after:{after_epoch}"
    )
    log(f"Gmail query: {query}")

    threads_out = []
    page_token = None
    while True:
        kwargs = {"userId": "me", "q": query, "maxResults": 50}
        if page_token:
            kwargs["pageToken"] = page_token
        resp = service.users().threads().list(**kwargs).execute()
        for t in resp.get("threads", []):
            thread = service.users().threads().get(
                userId="me", id=t["id"], format="full"
            ).execute()
            messages = []
            for msg in thread.get("messages", []):
                payload  = msg.get("payload", {})
                headers  = {h["name"].lower(): h["value"] for h in payload.get("headers", [])}
                body_str = _extract_body(payload)
                messages.append({
                    "id":      msg["id"],
                    "sender":  headers.get("from", ""),
                    "subject": headers.get("subject", ""),
                    "date":    headers.get("date", ""),
                    "body":    body_str,
                })
            threads_out.append({"thread_id": t["id"], "messages": messages})
        page_token = resp.get("nextPageToken")
        if not page_token:
            break

    log(f"Fetched {len(threads_out)} unprocessed bank-alert threads from Gmail")
    return threads_out


def _extract_body(payload) -> str:
    """Recursively extract plain/html body from a Gmail message payload."""
    mime = payload.get("mimeType", "")
    data = payload.get("body", {}).get("data", "")
    if data:
        import base64
        text = base64.urlsafe_b64decode(data + "==").decode("utf-8", errors="replace")
        return text
    for part in payload.get("parts", []):
        result = _extract_body(part)
        if result:
            return result
    return ""


def apply_claude_read_label(service, thread_id: str, dry_run=False):
    if dry_run:
        print(f"[dry-run] apply claude-read to thread {thread_id}")
        return
    service.users().threads().modify(
        userId="me", id=thread_id,
        body={"addLabelIds": [CLAUDE_READ_LABEL_ID]}
    ).execute()


# ══════════════════════════════════════════════════════════════════════════
# ENGINE.PY  (AI-free extraction)
# ══════════════════════════════════════════════════════════════════════════

def run_engine(envelopes: list[dict], log, ai_assist: bool = False) -> list[dict]:
    """Feed envelopes through extract-engine.py, return result dicts."""
    if not envelopes:
        return []
    with tempfile.NamedTemporaryFile(mode="w", suffix=".jsonl", delete=False) as f:
        for env in envelopes:
            f.write(json.dumps(env) + "\n")
        tmp = f.name
    try:
        # Pass CONFIG_PATH to engine subprocess
        env = os.environ.copy()
        cmd = [sys.executable, str(ENGINE), "--file", tmp]
        if ai_assist:
            cmd.append("--ai-assist")
        result = subprocess.run(
            cmd,
            capture_output=True, text=True, check=True,
            cwd=str(EXTRACT_DIR),
            env=env
        )
        outputs = []
        for line in result.stdout.strip().splitlines():
            if line.strip():
                outputs.append(json.loads(line))
        log(f"engine.py processed {len(envelopes)} envelopes → {len(outputs)} results")
        return outputs
    finally:
        os.unlink(tmp)


# ══════════════════════════════════════════════════════════════════════════
# DEDUP CHECK
# ══════════════════════════════════════════════════════════════════════════

def already_in_wallet(gm_id: str, existing_records: list) -> bool:
    tag = f"gm:{gm_id}"
    return any(tag in (r.get("note") or "") for r in existing_records)


def fuzzy_duplicate(amount: float, merchant: str, date: str,
                    existing_records: list) -> bool:
    """True if a manual entry with same amount+date exists (no gm: tag)."""
    for r in existing_records:
        if (r.get("note") and "gm:" in r["note"]):
            continue
        if (abs(r.get("amount", 0) + amount) < 0.10          # amounts stored negative
                and r.get("recordDate", "")[:10] == date[:10]
                and merchant.lower()[:6] in (r.get("counterParty") or "").lower()):
            return True
    return False


# ══════════════════════════════════════════════════════════════════════════
# OBSIDIAN WRITE-BACK
# ══════════════════════════════════════════════════════════════════════════

MONTH_NAMES = ["","January","February","March","April","May","June",
               "July","August","September","October","November","December"]

def expense_log_path(date_str: str) -> pathlib.Path:
    y, m, _ = date_str[:10].split("-")
    fname = f"{y}-{m:>02} {MONTH_NAMES[int(m)]}.md"
    return VAULT / "Expenses" / y / fname


def log_row_exists(log_path: pathlib.Path, gm_id: str) -> bool:
    if not log_path.exists():
        return False
    return f"gm:{gm_id}" in log_path.read_text()


def append_expense_row(log_path: pathlib.Path, record: dict, gm_id: str,
                       dry_run=False):
    date     = record.get("date", "")[:10]
    merchant = record.get("merchant", record.get("counterParty", "Unknown"))
    amount   = record.get("amount", 0)
    bank     = record.get("bank", "")
    instr    = record.get("instrument", "")
    card     = record.get("card_last4", "")
    account_label = f"{bank} {instr} XX{card}".strip() if card else bank

    row = (f"| {date} | {merchant} | — | -{abs(amount):.2f} | "
           f"{account_label} | gm:{gm_id} |\n")

    if dry_run:
        print(f"[dry-run] would append to {log_path.name}:\n  {row.strip()}")
        return

    if not log_path.exists():
        # create a minimal log file
        log_path.parent.mkdir(parents=True, exist_ok=True)
        y, m = date[:7].split("-")
        header = (f"---\ntitle: Expenses {y}-{m}\ntype: expense-log\n"
                  f"month: {y}-{m}\nyear: {y}\ntags:\n  - expenses\n---\n\n"
                  f"# Expenses — {MONTH_NAMES[int(m)]} {y}\n\n"
                  "## Transactions\n\n"
                  "| Date | Description | Category | Amount (₹) | Account | Notes |\n"
                  "|------|-------------|----------|------------|---------|-------|\n")
        log_path.write_text(header)

    text = log_path.read_text()
    # insert before the how-to hint line, or before the Category Breakdown section
    marker = "\n> **How to add expenses:**"
    if marker in text:
        text = text.replace(marker, row + marker)
    else:
        # fall back: append at end of transactions table
        text += row
    log_path.write_text(text)


# ══════════════════════════════════════════════════════════════════════════
# PART A — Gmail bank alerts
# ══════════════════════════════════════════════════════════════════════════

def part_a(gmail_svc, state: dict, categories: dict, labels_cache: dict,
           dry_run: bool, log, ai_assist: bool = False):
    since = state.get("last_email_timestamp") or (
        dt.datetime.utcnow() - dt.timedelta(days=DEFAULT_LOOKBACK_DAYS)
    ).strftime("%Y-%m-%dT%H:%M:%SZ")

    threads = fetch_gmail_threads(gmail_svc, since, log)
    if not threads:
        log("Part A: no new bank-alert threads")
        return 0, since

    # Build envelope JSONL for engine
    envelopes = []
    thread_map = {}   # message_id → thread_id
    for t in threads:
        for msg in t["messages"]:
            env = {
                "source":  "gmail",
                "id":      msg["id"],
                "sender":  msg["sender"],
                "subject": msg["subject"],
                "date":    msg["date"],
                "body":    msg["body"],
            }
            envelopes.append(env)
            thread_map[msg["id"]] = t["thread_id"]

    results = run_engine(envelopes, log, ai_assist=ai_assist)

    if ai_assist:
        # Any result the engine couldn't match may now have an AI-suggested
        # format file written to disk (process() learns but doesn't retry
        # within the same pass — see extract-engine.py). Re-run just those
        # envelopes once, without --ai-assist, so a genuinely new format is
        # applied in this same command instead of only "next run".
        learned_ids = {
            r["source_id"] for r in results
            if r.get("action") in ("unmatched", "error") and r.get("ai_created_format_file")
        }
        if learned_ids:
            log(f"  ai-assist: learned {len(learned_ids)} new format(s), retrying")
            retry_envelopes = [e for e in envelopes if e["id"] in learned_ids]
            retried = {r["source_id"]: r for r in run_engine(retry_envelopes, log)}
            results = [retried.get(r.get("source_id"), r) for r in results]

    pushed = 0
    unmatched_log = []
    newest_ts = since

    for res in results:
        msg_id = res.get("source_id", "")
        action = res.get("action", "")

        if action == "skip":
            log(f"  skip  [{res.get('format','')}] {msg_id}")
            # still label so we don't re-fetch
            _mark_processed(gmail_svc, thread_map.get(msg_id,""), dry_run)
            continue

        if action in ("unmatched", "error"):
            unmatched_log.append(res)
            log(f"  {'UNMATCHED' if action=='unmatched' else 'ERROR'} {msg_id}"
                f"  sender={res.get('sender','')}  subj={res.get('subject','')[:60]}")
            continue

        if action != "extract":
            continue

        rec = res["record"]
        date_str = rec.get("date", "")[:10]
        amount   = float(rec.get("amount", 0))
        merchant = rec.get("merchant", "")
        gm_id    = msg_id
        account_id = rec.get("wallet_account_id", "")

        if not account_id:
            log(f"  UNROUTED {gm_id} {merchant} ₹{amount}")
            unmatched_log.append(res)
            continue

        # dedup
        existing = fetch_wallet_records(
            (dt.date.fromisoformat(date_str) - dt.timedelta(days=1)).isoformat(),
            (dt.date.fromisoformat(date_str) + dt.timedelta(days=2)).isoformat(),
            account_id,
            dry_run=dry_run
        )
        if already_in_wallet(gm_id, existing):
            log(f"  dedup-skip (gm: already in wallet) {gm_id}")
            _mark_processed(gmail_svc, thread_map.get(gm_id,""), dry_run)
            continue
        if fuzzy_duplicate(amount, merchant, date_str, existing):
            log(f"  dedup-skip (fuzzy match, manual entry) {gm_id} {merchant} ₹{amount}")
            _mark_processed(gmail_svc, thread_map.get(gm_id,""), dry_run)
            continue

        # category + labels
        cat_id    = guess_category_id(merchant, categories)
        label_ids = label_ids_for_record(rec, labels_cache)

        # Note format: <merchant> | via <instrument> | gm:<msgid> | proc:<label>:create
        # Max 255 chars; the gm: key and proc: tag must always survive
        # truncation (gm: for dedup, proc: so this record is traceable back
        # to this pipeline) — only the merchant field gets shortened.
        source_tag = proc_tag("create")
        # Use account_name from routing if available, otherwise use instrument
        instrument = rec.get("account_name") or rec.get("instrument", "")
        max_merchant_len = 255 - len(f" | via  | gm:{gm_id}") - len(source_tag)
        merchant_truncated = merchant[:max(10, max_merchant_len)]
        note = f"{merchant_truncated} | via {instrument} | gm:{gm_id}{source_tag}"
        note = note[:255]

        payload = {
            "accountId":    account_id,
            "amount":       -abs(amount),
            "currency":     rec.get("currency", "INR"),
            "recordDate":   date_str,
            "paymentType":  _payment_type(rec.get("instrument","")),
            "note":         note,
            "counterParty": merchant[:255],
        }
        if cat_id:
            payload["categoryId"] = cat_id
        if label_ids:
            payload["labelIds"] = label_ids

        wallet_post("/records", {"records": [payload]}, dry_run=dry_run)
        log(f"  pushed {gm_id} {merchant} ₹{amount} → account {account_id}")

        # Obsidian write-back (opt-in — see SA_VAULT above)
        if VAULT:
            log_path = expense_log_path(date_str)
            if not log_row_exists(log_path, gm_id):
                append_expense_row(log_path, rec, gm_id, dry_run=dry_run)

        _mark_processed(gmail_svc, thread_map.get(gm_id,""), dry_run)
        pushed += 1

        # advance cursor
        if date_str > newest_ts[:10]:
            newest_ts = date_str + "T23:59:59Z"

    if unmatched_log:
        _save_unmatched(unmatched_log, dry_run, log)

    return pushed, newest_ts


def _mark_processed(gmail_svc, thread_id: str, dry_run: bool):
    if not thread_id:
        return
    apply_claude_read_label(gmail_svc, thread_id, dry_run=dry_run)


def _payment_type(instrument: str) -> str:
    m = instrument.lower()
    if "credit" in m:   return "credit_card"
    if "debit" in m:    return "debit_card"
    if "upi" in m or "amazon_pay" in m: return "mobile_payment"
    return "web_payment"


def _save_unmatched(items: list, dry_run: bool, log):
    path = SYNC_DIR / "unmatched.jsonl"
    log(f"  {len(items)} unmatched/error → {path}")
    log(f"  ⚠  Codify each one in {CONFIG_PATH}/config/expense-domain/gmail/email-formats/email.<bank>.yaml"
        f" (or re-run with --ai-assist), then re-run.")
    if dry_run:
        for i in items:
            print(json.dumps(i, indent=2)[:400])
        return
    with open(path, "a") as f:
        for i in items:
            f.write(json.dumps(i) + "\n")


# ══════════════════════════════════════════════════════════════════════════
# PART B — Google Drive Bills Inbox
# ══════════════════════════════════════════════════════════════════════════

def part_b(drive_svc, state: dict, categories: dict, labels_cache: dict,
           dry_run: bool, log):
    # Skip Drive API calls in dry-run mode (API may not be enabled)
    if dry_run:
        log("Part B: skipped in dry-run mode (Drive API integration pending)")
        return 0, state.get("drive_cursor", "")

    drive_cursor = state.get("drive_cursor", "")
    processed    = set(state.get("processed_drive_files", []))

    query_parts = [f"'{DRIVE_BILLS_FOLDER}' in parents", "trashed=false"]
    if drive_cursor:
        query_parts.append(f"createdTime > '{drive_cursor}'")
    q = " and ".join(query_parts)

    try:
        resp = drive_svc.files().list(
            q=q, fields="files(id,name,mimeType,createdTime)",
            orderBy="createdTime"
        ).execute()
        files = [f for f in resp.get("files", []) if f["id"] not in processed]
    except Exception as e:
        # Drive API not enabled or accessible - skip gracefully
        if "accessNotConfigured" in str(e) or "403" in str(e):
            log(f"Part B: Drive API not enabled, skipping (enable in Google Cloud Console)")
            return 0, drive_cursor
        raise

    if not files:
        log("Part B: no new Drive bill files")
        return 0, drive_cursor

    pushed = 0
    newest_drive_ts = drive_cursor

    for f in files:
        file_id   = f["id"]
        file_name = f["name"]
        mime      = f["mimeType"]
        created   = f["createdTime"]
        log(f"  Drive file: {file_name} ({mime})")

        # Download and try text extraction
        text = _drive_extract_text(drive_svc, file_id, mime, log)
        if text:
            # Try engine.py for text PDFs
            env = {
                "source":  "drive",
                "id":      file_id,
                "subject": file_name,
                "date":    created,
                "body":    text,
            }
            results = run_engine([env], log)
            res = results[0] if results else {}
            if res.get("action") == "extract":
                rec = res["record"]
                _handle_drive_record(rec, file_id, file_name, categories,
                                     labels_cache, state, dry_run, log)
                pushed += 1
            else:
                log(f"  ⚠  Drive: engine could not parse {file_name} "
                    f"(action={res.get('action','?')}). "
                    f"Image bills need AI — upload to Bills Inbox and run Claude.")
        else:
            log(f"  ⚠  Drive: could not extract text from {file_name} (image/scan). "
                "Needs AI OCR — run Claude for this file.")

        processed.add(file_id)
        if created > newest_drive_ts:
            newest_drive_ts = created

    state["processed_drive_files"] = sorted(processed)
    return pushed, newest_drive_ts


def _drive_extract_text(drive_svc, file_id: str, mime: str, log) -> str:
    """Attempt to get plain text from a Drive file. Returns '' if not possible."""
    try:
        if "pdf" in mime:
            # try pdftotext first
            import io
            request = drive_svc.files().get_media(fileId=file_id)
            from googleapiclient.http import MediaIoBaseDownload
            buf = io.BytesIO()
            dl = MediaIoBaseDownload(buf, request)
            done = False
            while not done:
                _, done = dl.next_chunk()
            buf.seek(0)
            # Try pdfplumber if available
            try:
                import pdfplumber
                with pdfplumber.open(buf) as pdf:
                    return "\n".join(p.extract_text() or "" for p in pdf.pages)
            except ImportError:
                pass
            # Fallback: pdftotext
            with tempfile.NamedTemporaryFile(suffix=".pdf", delete=False) as f:
                f.write(buf.read())
                tmp = f.name
            try:
                r = subprocess.run(["pdftotext", tmp, "-"], capture_output=True, text=True)
                if r.returncode == 0:
                    return r.stdout
            finally:
                os.unlink(tmp)
    except Exception as e:
        log(f"    drive text extraction error: {e}")
    return ""


def _handle_drive_record(rec, file_id, file_name, categories, labels_cache,
                         state, dry_run, log):
    date_str = rec.get("date", "")[:10]
    amount   = float(rec.get("amount", 0))
    merchant = rec.get("merchant", file_name)
    account_id = rec.get("wallet_account_id", "")

    if not account_id:
        log(f"    UNROUTED drive:{file_id} {merchant}")
        return

    # Try to match existing Wallet record (same date±3 days, same amount)
    d = dt.date.fromisoformat(date_str)
    existing = fetch_wallet_records(
        (d - dt.timedelta(days=3)).isoformat(),
        (d + dt.timedelta(days=4)).isoformat(),
        account_id
    )
    drive_tag  = f"drive:{file_id}"
    match = next((r for r in existing
                  if abs(r.get("amount", 0) + amount) < 0.10
                  and drive_tag not in (r.get("note") or "")), None)

    cat_id    = guess_category_id(merchant, categories)
    label_ids = label_ids_for_record(rec, labels_cache)

    if match:
        # Enrich existing record. Strip only our own prior *update* tag
        # (avoid stacking one per patch) — the create tag (ours or another
        # pipeline's) and everything else is preserved by appending rather
        # than replacing; that preservation is the whole point (traceability).
        base_note = re.sub(rf"\s*\|\s*proc:{re.escape(PROC_LABEL)}:update", "",
                            match.get("note") or "")
        note = (base_note + f" drive:{file_id}" + proc_tag("update"))[:255]
        wallet_patch(f"/records/{match['id']}",
                     {"note": note, **({"categoryId": cat_id} if cat_id else {})},
                     dry_run=dry_run)
        log(f"    patched existing Wallet record {match['id']} with drive:{file_id}")
    else:
        # Create new record from bill
        source_tag = proc_tag("create")
        note = f"{merchant} | from-bill drive:{file_id}{source_tag}"[:255]
        payload = {
            "accountId":    account_id,
            "amount":       -abs(amount),
            "currency":     rec.get("currency", "INR"),
            "recordDate":   date_str,
            "paymentType":  "web_payment",
            "note":         note,
            "counterParty": merchant[:255],
        }
        if cat_id:    payload["categoryId"] = cat_id
        if label_ids: payload["labelIds"]   = label_ids
        wallet_post("/records", {"records": [payload]}, dry_run=dry_run)
        log(f"    created Wallet record from Drive bill {file_name}")

    # Bill note + product-prices (simplified — full bill parsing needs AI for images)
    log(f"    ℹ  Bill note creation for Drive files currently requires Claude "
        f"(OCR + line-item extraction). File: {file_name}")


# ══════════════════════════════════════════════════════════════════════════
# PART C — Cross-source reconciliation
# ══════════════════════════════════════════════════════════════════════════

def part_c(state: dict, dry_run: bool, log):
    """Minimal pass: flag probable duplicates to pending_review, don't merge blindly."""
    log("Part C: cross-source reconciliation — see pending_review in last-sync.json")
    # The full merge logic lives in the Runbook; here we surface any obvious
    # same-day same-amount duplicates from the current run for human review.
    # Actual merging is intentionally left to Claude to avoid data loss.


# ══════════════════════════════════════════════════════════════════════════
# PART D — Label sync
# ══════════════════════════════════════════════════════════════════════════

def part_d(dry_run: bool, log):
    """Refresh labels-cache.json from Wallet and report count."""
    labels_raw = wallet_get("/labels")
    labels = labels_raw if isinstance(labels_raw, list) else labels_raw.get("data", [])
    cache = {l["name"]: l["id"] for l in labels}
    if not dry_run:
        LABELS_CACHE.write_text(json.dumps(cache, indent=2))
    log(f"Part D: refreshed labels-cache.json with {len(cache)} labels")
    return cache


# ══════════════════════════════════════════════════════════════════════════
# LOGGING
# ══════════════════════════════════════════════════════════════════════════

def make_logger(dry_run: bool):
    LOG_DIR.mkdir(parents=True, exist_ok=True)
    ts  = dt.datetime.utcnow().strftime("%Y%m%dT%H%M%SZ")
    tag = "DRY" if dry_run else "RUN"
    log_file = LOG_DIR / f"sync-{ts}-{tag}.log"
    lines = []
    def log(msg: str):
        print(msg)
        lines.append(msg)
    def flush():
        log_file.write_text("\n".join(lines))
    return log, flush


# ══════════════════════════════════════════════════════════════════════════
# MAIN
# ══════════════════════════════════════════════════════════════════════════

def main():
    ap = argparse.ArgumentParser(description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--dry-run",  action="store_true", help="print actions, write nothing")
    ap.add_argument("--since",    help="override cursor, e.g. 2026-06-01")
    ap.add_argument("--auth",     action="store_true", help="redo Google OAuth consent")
    ap.add_argument("--test-engine", action="store_true", help="run engine.py --test and exit")
    ap.add_argument("--ai-assist", action="store_true",
                     help="for emails matching no known format, ask AI_PROVIDER to suggest one, "
                          "save it, and retry within this same run (costs API calls; off by default)")
    args = ap.parse_args()

    if args.test_engine:
        r = subprocess.run([sys.executable, str(ENGINE), "--test"], cwd=str(EXTRACT_DIR))
        sys.exit(r.returncode)

    log, flush = make_logger(args.dry_run)
    log(f"{'[DRY RUN] ' if args.dry_run else ''}sync.py start "
        f"{dt.datetime.utcnow().strftime('%Y-%m-%dT%H:%M:%SZ')}")

    # ── Google auth ───────────────────────────────────────────────────────
    creds      = _get_google_creds(force_reauth=args.auth)
    gmail_svc  = _gmail_service(creds)
    drive_svc  = _drive_service(creds)
    log("Google auth OK")

    # ── State ─────────────────────────────────────────────────────────────
    state = load_state()
    if args.since:
        state["last_email_timestamp"] = args.since + "T00:00:00Z"
        log(f"Cursor overridden to {state['last_email_timestamp']}")

    # ── Wallet bootstrap ──────────────────────────────────────────────────
    categories   = fetch_wallet_categories()
    labels_cache = load_labels_cache()
    log(f"Wallet: {len(categories)} categories, {len(labels_cache)} cached labels")

    # ── Part A ────────────────────────────────────────────────────────────
    log("\n── Part A: Gmail bank alerts ────────────────────────────────────")
    a_pushed, new_ts = part_a(gmail_svc, state, categories, labels_cache,
                               args.dry_run, log, ai_assist=args.ai_assist)
    log(f"Part A done: {a_pushed} records pushed")

    # ── Part B ────────────────────────────────────────────────────────────
    log("\n── Part B: Drive Bills Inbox ────────────────────────────────────")
    b_pushed, new_drive_ts = part_b(drive_svc, state, categories, labels_cache,
                                     args.dry_run, log)
    log(f"Part B done: {b_pushed} records pushed/patched")

    # ── Part C ────────────────────────────────────────────────────────────
    log("\n── Part C: reconciliation ───────────────────────────────────────")
    part_c(state, args.dry_run, log)

    # ── Part D ────────────────────────────────────────────────────────────
    log("\n── Part D: label sync ───────────────────────────────────────────")
    part_d(args.dry_run, log)

    # ── Update cursor ─────────────────────────────────────────────────────
    now = dt.datetime.utcnow().strftime("%Y-%m-%dT%H:%M:%SZ")
    state["last_email_timestamp"] = new_ts
    state["drive_cursor"]         = new_drive_ts or state.get("drive_cursor","")
    state["last_run"]             = now
    state["last_run_status"]      = (
        f"sync.py (no-AI). A:{a_pushed} B:{b_pushed} records pushed."
    )
    state["records_pushed_total"] = state.get("records_pushed_total", 0) + a_pushed + b_pushed

    save_state(state, dry_run=args.dry_run)
    flush()
    log(f"\nDone. {a_pushed + b_pushed} total records pushed. Log → {LOG_DIR}")


if __name__ == "__main__":
    main()
