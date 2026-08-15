"""Reading a broker export into canonical rows.

This is the module the feature exists to make possible. NOTHING HERE NAMES A
BROKER — if you find yourself adding `if broker == ...`, the profile contract is
wrong and should be extended instead (constitution Principle V, FR-020).

Two reader shapes behind one contract (research.md R-003):

    tabular    one flat table, maybe preceded by title lines
    sectioned  many sections, each with its own header row

Both emit the same thing: a list of dicts with canonical field names plus a
`section` label and the raw row. Everything downstream is shape-independent.
"""
from __future__ import annotations

import csv
import re
from datetime import datetime
from pathlib import Path

CANONICAL = ("date", "action", "symbol", "desc", "qty", "price",
             "fees", "amount", "section")


class ProfileError(Exception):
    """The profile is wrong, or the file does not match it."""


def parse_money(raw, profile: dict):
    if raw is None or raw == "":
        return None
    text = str(raw).strip()
    negative = False
    money = profile.get("money", {})
    if money.get("parens_negative") and text.startswith("(") and text.endswith(")"):
        negative, text = True, text[1:-1]
    for char in money.get("strip", []):
        text = text.replace(char, "")
    text = text.strip()
    if text in ("", "-", "--", "."):
        return None
    try:
        value = float(text)
    except ValueError:
        return None
    return -value if negative else value


def parse_date(raw, profile: dict):
    if raw is None or raw == "":
        return None
    text = str(raw).strip()
    dates = profile.get("dates", {})
    marker = dates.get("split_marker")
    if marker and marker in text:
        before, _, after = text.partition(marker)
        # "posted as of effective" — the holding period runs from the effective
        # date, so preferring it changes tax outcomes. Profile decides.
        text = (after if dates.get("prefer_as_of") else before).strip()
    text = text.split(",")[0].strip() if "," in text else text
    for fmt in dates.get("formats", []):
        try:
            return datetime.strptime(text, fmt).date()
        except ValueError:
            continue
    return None


def _read_tabular(path: Path, profile: dict) -> list[dict]:
    reader_cfg = profile["reader"]
    tokens = reader_cfg["header_tokens"]
    lines = Path(path).read_text(errors="replace").splitlines()

    header_index = None
    for i, line in enumerate(lines):
        if all(token in line for token in tokens):
            header_index = i
            break
    if header_index is None:
        raise ProfileError(
            f"{path}: no header row containing all of {tokens}. Either this is "
            f"not a {profile['name']} export, or header_tokens needs updating.")

    rows = list(csv.DictReader(lines[header_index:]))
    mapping = reader_cfg["columns"]
    out = []
    for raw in rows:
        if not any((v or "").strip() for v in raw.values()):
            continue
        record = {"section": "main", "_raw": raw}
        for field, candidates in mapping.items():
            value = next((raw[c] for c in candidates
                          if c in raw and (raw[c] or "").strip()), None)
            record[field] = value
        out.append(record)
    return out


def _read_sectioned(path: Path, profile: dict) -> list[dict]:
    reader_cfg = profile["reader"]
    type_col = reader_cfg.get("row_type_column", 1)
    data_value = reader_cfg.get("data_row_value", "Data")
    by_name = {s["section_name"]: s for s in reader_cfg["sections"]}

    headers: dict[str, list[str]] = {}
    out = []
    with open(path, newline="", errors="replace") as fh:
        for cells in csv.reader(fh):
            if len(cells) <= type_col:
                continue
            section_name, row_type = cells[0], cells[type_col]
            if section_name not in by_name:
                continue
            if row_type == "Header":
                headers[section_name] = cells
                continue
            if row_type != data_value:
                continue                      # SubTotal / Total — never data
            header = headers.get(section_name)
            if not header:
                continue
            raw = dict(zip(header, cells))
            spec = by_name[section_name]

            filter_col = spec.get("filter_col")
            if filter_col and raw.get(filter_col) not in (spec.get("filter_values") or []):
                continue

            record = {"section": spec["name"], "_raw": raw}
            for field, column in spec["field_map"].items():
                record[field] = raw.get(column)
            out.append(record)
    return out


READERS = {"tabular": _read_tabular, "sectioned": _read_sectioned}


def read_rows(path: Path, profile: dict) -> list[dict]:
    """File -> canonical rows, with money and dates already parsed."""
    kind = profile["reader"]["kind"]
    if kind not in READERS:
        raise ProfileError(f"unknown reader kind {kind!r}")
    rows = READERS[kind](path, profile)

    for row in rows:
        row["date"] = parse_date(row.get("date"), profile)
        for field in ("qty", "price", "fees", "amount"):
            if field in row:
                row[field] = parse_money(row.get(field), profile)
        for field in CANONICAL:
            row.setdefault(field, None)
    return rows


def route(row: dict, profile: dict) -> dict | None:
    """First matching action rule wins; None means nothing matched.

    A None return is NOT a silent drop — the caller reports it as unrecognised
    (FR-022, SC-007).
    """
    for rule in profile["actions"]:
        if "match" in rule:
            action = row.get("action") or ""
            if not re.search(rule["match"], str(action)):
                continue
        if "section" in rule and row.get("section") != rule["section"]:
            continue
        if "qty_sign" in rule:
            # Some brokers encode buy vs sell in the SIGN OF THE QUANTITY rather
            # than in an action string. Without this, every one of their trades
            # routes to the same event. See research.md R-003.
            qty = row.get("qty")
            if qty is None:
                continue
            if rule["qty_sign"] == "positive" and qty <= 0:
                continue
            if rule["qty_sign"] == "negative" and qty >= 0:
                continue
        return rule
    return None


def validate_profile(profile: dict) -> list[str]:
    """Checks beyond what the schema can express."""
    problems = []
    name = profile.get("name", "<unnamed>")

    rules = profile.get("actions", [])
    for i, rule in enumerate(rules):
        if rule["event"] in ("buy", "vest", "espp", "transfer_in") and not rule.get("funding"):
            problems.append(
                f"profile {name}: action rule {i} creates a lot "
                f"(event={rule['event']}) but declares no `funding`")
        if not any(k in rule for k in ("match", "section", "qty_sign")):
            problems.append(
                f"profile {name}: action rule {i} has no predicate — it would "
                f"match every row and shadow everything after it")

    if rules and not any(r.get("match") == ".*" and
                         not r.get("section") and not r.get("qty_sign")
                         for r in rules):
        problems.append(
            f"profile {name}: no catch-all rule. Add "
            f"{{match: '.*', event: ignore, bucket: unmatched}} last so "
            f"unrecognised rows are accounted for rather than dropped.")

    reader = profile.get("reader", {})
    if reader.get("kind") == "sectioned":
        seen = set()
        for section in reader.get("sections", []):
            if section["name"] in seen:
                problems.append(
                    f"profile {name}: duplicate section label {section['name']!r}")
            seen.add(section["name"])
    return problems
