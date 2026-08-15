"""Reconciling a broker export against the register.

Invariants this module exists to keep:

* **Every row is accounted for.** created + matched + skipped + unrecognised +
  ignored == rows read. Asserted at the end of every import (SC-007).
* **Re-import changes nothing.** Dedupe is by `src` fingerprint, never by
  (date, qty) — a lapse sale shrinks the vest lot it came from, so quantity
  stops matching after the first run and the vest would be re-imported.
* **Nothing is silently dropped.** A row matching no action rule is reported.
"""
from __future__ import annotations

from datetime import date
from pathlib import Path

from . import lots as lotlib
from . import profiles as profilelib
from .rules import RateTable, Rates

LOT_CREATING = {"buy", "vest", "espp", "transfer_in"}


class ImportRefused(Exception):
    """Something was detected that makes importing unsafe (e.g. a split)."""


def fingerprint(broker: str, row: dict, event: str, index: int) -> str:
    """`<broker>:<date>:<event>:<qty>@<price>#<n>` — the dedupe key."""
    when = row.get("date")
    return (f"{broker}:{when.isoformat() if when else 'nodate'}:{event}:"
            f"{row.get('qty') or 0:g}@{row.get('price') or 0:g}#{index}")


def _existing_fingerprints(register: dict) -> set[str]:
    seen = set()
    for position in register.get("positions", {}).values():
        for lot in position.get("lots", []):
            seen.add(lot.get("src", ""))
        for disposal in position.get("closed", []):
            seen.add(disposal.get("src", ""))
    seen.discard("")
    return seen


def _lot_id(row: dict, rule: dict, profile: dict, taken: set[str]) -> str:
    when = row["date"]
    template = profile.get("lot_id_template", "{YM}-{kind}")
    base = (template
            .replace("{YM}", when.strftime("%b%y").upper())
            .replace("{ym}", when.strftime("%y%m"))
            .replace("{kind}", rule.get("funding", "LOT")))
    candidate, n = base, 1
    while candidate in taken:
        n += 1
        candidate = f"{base}-{n}"
    taken.add(candidate)
    return candidate


def run(path: Path, profile: dict, register: dict, rules: dict,
        rate_table: RateTable, *, dry_run: bool = False,
        allow_duplicate_rows: bool = False) -> dict:
    """Reconcile `path` into `register`.

    A dry run simulates against a COPY and reports exactly what a real run
    would do, then discards it. Simulating without applying would be worse than
    useless: a lot created earlier in the file would not exist when a later sell
    looked for it, so the preview would omit disposals the real run then makes —
    the opposite of what FR-027 exists for.
    """
    import copy

    rates = Rates(rules)
    broker = profile["name"]
    rows = profilelib.read_rows(path, profile)

    working = copy.deepcopy(register) if dry_run else register
    seen = _existing_fingerprints(working)

    batch = {"source": str(path), "profile": broker, "created": [],
             "matched": [], "skipped": [], "unrecognised": [], "ignored": [],
             "warnings": []}

    counters: dict[str, int] = {}
    within_file: set[str] = set()
    # One entry per ROW, recording how that row was handled. Artifact counts
    # cannot serve as the accounting identity: a single sell row legitimately
    # splits across several lots and produces several disposals.
    outcomes: list[str] = []

    for row in rows:
        rule = profilelib.route(row, profile)
        if rule is None:
            batch["unrecognised"].append({
                "action": row.get("action"), "section": row.get("section"),
                "date": row.get("date"), "raw": row.get("_raw")})
            outcomes.append("unrecognised")
            continue

        event = rule["event"]
        if event == "ignore":
            batch["ignored"].append({"bucket": rule.get("bucket", "unspecified"),
                                     "action": row.get("action")})
            outcomes.append("ignored")
            continue
        if event == "corporate_action":
            raise ImportRefused(
                f"{path}: row dated {row.get('date')} is a corporate action "
                f"({row.get('action')!r}). Quantities and per-share basis for "
                f"{row.get('symbol')} would be wrong across it, so the import "
                f"is refused rather than producing confidently wrong figures. "
                f"Handling corporate actions is out of scope (FR-030).")

        if row.get("date") is None or not row.get("symbol"):
            batch["unrecognised"].append({
                "action": row.get("action"), "reason": "no usable date or symbol",
                "raw": row.get("_raw")})
            outcomes.append("unrecognised")
            continue

        key = f"{row['date']}|{event}|{row.get('qty')}|{row.get('price')}"
        counters[key] = counters.get(key, 0) + 1
        src = fingerprint(broker, row, event, counters[key])

        if src in seen:
            batch["skipped"].append({"src": src, "reason": "already imported"})
            outcomes.append("skipped")
            continue
        if src in within_file and not allow_duplicate_rows:
            batch["skipped"].append({"src": src, "reason": "duplicate row within file"})
            batch["warnings"].append(
                f"dropped a repeated row ({row['date']} {row.get('qty')}@"
                f"{row.get('price')}). Pass --allow-duplicate-rows if these "
                f"were two genuine fills.")
            outcomes.append("skipped")
            continue
        within_file.add(src)

        ticker = str(row["symbol"]).strip().upper()
        position = working.setdefault("positions", {}).setdefault(
            ticker, {"broker": broker, "currency": "USD", "lots": [], "closed": []})

        if event in LOT_CREATING:
            _create_lot(row, rule, profile, position, broker, src,
                        rate_table, batch)
            outcomes.append("created")
        elif event == "sell":
            _match_disposal(row, rule, position, rates, src, batch)
            outcomes.append("matched")

    if len(outcomes) != len(rows):
        raise AssertionError(
            f"row accounting broken: read {len(rows)}, accounted "
            f"{len(outcomes)}. Every row must be created, matched, skipped, "
            f"unrecognised or ignored (SC-007).")
    batch["rows_read"] = len(rows)
    batch["outcomes"] = outcomes
    return batch


def _create_lot(row, rule, profile, position, broker, src, rate_table,
                batch):
    taken = {lot.get("id") for lot in position["lots"]}
    acq_fx, fx_flag = rate_table.lookup(row["date"])
    flags = [fx_flag] if fx_flag else []

    if rule["funding"] != "MARKET":
        # The broker reports the market price; the tax basis for compensation
        # shares is the employer's valuation. Never trust the export for this.
        flags.append({
            "code": "cost_basis_unverified",
            "note": (f"cost basis {row.get('price')} came from the broker export. "
                     f"For {rule['funding']} shares the basis is the employer's "
                     f"valuation — reconcile before filing."),
            "raised_by": "import"})

    lot = {
        "id": _lot_id(row, rule, profile, taken),
        "broker": broker,
        "acq_date": row["date"].isoformat(),
        "qty": abs(row["qty"] or 0.0),
        "cb_per_share": row.get("price") or 0.0,
        "price_paid_per_share": (row.get("price") or 0.0)
                                if rule["funding"] != "RSU" else 0.0,
        "acq_fx": acq_fx or 0.0,
        "funding": rule["funding"],
        "src": src,
        "confirmed": not flags,
    }
    if flags:
        lot["flags"] = flags
    if acq_fx is None:
        lot.setdefault("flags", []).append({
            "code": "fx_interpolated",
            "note": f"no FX entry at or before {row['date']}; recorded as 0.",
            "raised_by": "import"})
        lot["confirmed"] = False

    # Snapshot, not the live dict: a disposal later in the same file consumes
    # part of this lot and rewrites its qty in place, so reporting the shared
    # reference would show what REMAINS rather than what was CREATED — 15 for a
    # 25-share buy that was partly sold. Reconciling that against a statement
    # is exactly the confusion the import summary exists to prevent.
    batch["created"].append(dict(lot))
    position["lots"].append(lot)


def _match_disposal(row, rule, position, rates, src, batch):
    qty_left = abs(row["qty"] or 0.0)
    disp_date = row["date"]
    price = row.get("price") or 0.0
    open_lots = position["lots"]

    if rule.get("lapse"):
        target = lotlib.find_lapse_lot(open_lots, disp_date,
                                       rates.lapse_window, rates.tiebreak)
        order = [target] if target else []
    else:
        order = lotlib.fifo_order(open_lots, rates.tiebreak, disp_date)

    for lot in order:
        if qty_left <= 1e-9:
            break
        take = min(qty_left, lot["qty"])
        consumed, remainder = lotlib.split(lot, take)
        disposal = lotlib.to_disposal(
            consumed, disp_date, price, lot["acq_fx"], rates.ltcg_months,
            lapse=bool(rule.get("lapse")), fees=row.get("fees") or 0.0, src=src)

        warning = lotlib.near_maturity_warning(disposal, rates.ltcg_months)
        if warning:
            batch["warnings"].append(f"⚠ {warning}")

        batch["matched"].append(disposal)
        qty_left -= take

        position["closed"].append(disposal)
        if remainder is None:
            open_lots.remove(lot)
        else:
            lot["qty"] = remainder["qty"]

    if qty_left > 1e-6:
        batch["warnings"].append(
            f"⚠ disposal on {disp_date} for {abs(row['qty'])} exceeded the open "
            f"lots available by {qty_left:g} — the register may be missing an "
            f"acquisition. Nothing was invented to cover the shortfall.")


def format_batch(batch: dict, dry_run: bool) -> str:
    """Human summary. The counts are the point — they are SC-007 made visible."""
    lines = [
        f"{'DRY RUN — nothing written' if dry_run else 'Imported'}"
        f" from {batch['source']} using profile {batch['profile']}",
        f"  rows read      {batch['rows_read']}",
        f"  lots created   {len(batch['created'])}",
        f"  disposals      {len(batch['matched'])}",
        f"  skipped        {len(batch['skipped'])}  (already imported)",
        f"  ignored        {len(batch['ignored'])}",
        f"  unrecognised   {len(batch['unrecognised'])}",
    ]
    for lot in batch["created"]:
        lines.append(f"    + {lot['id']:<16} {lot['qty']:>10g} @ "
                     f"{lot['cb_per_share']:<10g} {lot['funding']}")
    for disposal in batch["matched"]:
        kind = "LTCG" if disposal["long_term"] else "STCG"
        lines.append(f"    - {disposal['from_lot']:<16} {disposal['qty']:>10g} @ "
                     f"{disposal['disp_price']:<10g} {kind}")
    # A row reaching the profile's catch-all is NOT the same as one the profile
    # deliberately ignores. Both land in `ignored`, so without this split a
    # genuinely unknown action hides inside the ignored count and FR-022's
    # guarantee is defeated by the very rule that exists to satisfy SC-007.
    unmatched = [i for i in batch["ignored"]
                 if i.get("bucket") in ("unmatched", "unspecified")]
    for row in batch["unrecognised"]:
        unmatched.append({"action": row.get("action"), "bucket": "no rule"})

    if unmatched:
        lines.append(f"  ⚠ {len(unmatched)} row(s) MATCHED NO SPECIFIC RULE and "
                     f"were not imported. If any of these is a real acquisition "
                     f"or disposal, add a rule to the profile:")
        seen = []
        for row in unmatched:
            label = str(row.get("action"))
            if label not in seen:
                seen.append(label)
                lines.append(f"    ? {label!r}")
    for warning in batch["warnings"]:
        lines.append(f"  {warning}")
    return "\n".join(lines)
