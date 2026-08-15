"""Redaction for shared copies.

The rule that matters (FR-047): redaction DELETES fields from the data
document. A page that merely hides a figure still ships it — anyone can open
the file and read the payload. So there is no "hide" path in this module, only
removal, and the test asserts absence from the delivered bytes.

The second rule (FR-049): a profile is refused if what it withholds can be
reconstructed from what it keeps. Withholding absolute money while retaining
quantity and a per-share price withholds nothing, and that is the first profile
anyone writes.
"""
from __future__ import annotations

import re


class DisclosureRefused(Exception):
    """The profile would leak what it claims to withhold."""


def _walk_delete(node, parts: list[str]):
    """Delete a field path like `positions[].lots[].qty` from the document."""
    if not parts:
        return
    head, rest = parts[0], parts[1:]
    is_list = head.endswith("[]")
    key = head[:-2] if is_list else head

    if isinstance(node, dict):
        if key not in node:
            return
        if is_list:
            for item in node[key] or []:
                _walk_delete(item, rest) if rest else None
            if not rest:
                node[key] = []
        elif rest:
            _walk_delete(node[key], rest)
        else:
            node.pop(key, None)
    elif isinstance(node, list):
        for item in node:
            _walk_delete(item, parts)


def _present(node, parts: list[str]) -> bool:
    """Does this field path still resolve to at least one value?"""
    if not parts:
        return node is not None
    head, rest = parts[0], parts[1:]
    is_list = head.endswith("[]")
    key = head[:-2] if is_list else head

    if isinstance(node, dict):
        if key not in node:
            return False
        value = node[key]
        if is_list:
            return any(_present(item, rest) for item in value or []) if rest \
                else bool(value)
        return _present(value, rest) if rest else value is not None
    if isinstance(node, list):
        return any(_present(item, parts) for item in node)
    return False


def check_derivable(document: dict, withhold: list[str],
                    derivations: list[dict]) -> list[str]:
    """Every withheld field that survives via a known reconstruction path."""
    withheld = set(withhold)
    problems = []
    for rule in derivations or []:
        target = rule["target"]
        if target not in withheld:
            continue
        sources = rule["from"]
        if all(src not in withheld for src in sources):
            problems.append(
                f"{target} is withheld, but {' x '.join(sources)} are retained "
                f"and reconstruct it"
                + (f" ({rule['note']})" if rule.get("note") else ""))
    return problems


def apply(document: dict, profile: dict, derivations: list[dict]) -> dict:
    """Return a redacted copy of `document`. Refuses rather than leaking."""
    withhold = profile.get("withhold", []) or []
    if not withhold:
        return dict(document, disclosure=profile.get("_name", "full"), withheld=[])

    problems = check_derivable(document, withhold, derivations)
    if problems:
        raise DisclosureRefused(
            "this profile would not actually withhold what it claims:\n  "
            + "\n  ".join(problems)
            + "\n\nNothing was produced. Either withhold the reconstructing "
              "fields too, or accept that these figures are shared.")

    import copy
    redacted = copy.deepcopy(document)
    for path in withhold:
        _walk_delete(redacted, path.split("."))

    redacted["disclosure"] = profile.get("_name", "redacted")
    redacted["withheld"] = list(withhold)

    leftover = [p for p in withhold if _present(redacted, p.split("."))]
    if leftover:
        raise DisclosureRefused(
            f"redaction did not remove {leftover} — refusing to produce a copy "
            f"that claims to withhold them. This is a bug in the field paths.")

    # Field-path deletion is necessary but NOT sufficient. A withheld number can
    # survive inside prose: the rating `why` templates state a lot's own spot,
    # basis and break-even in words, so deleting positions[].spot leaves
    # "At $427.76 the stock is trading BELOW the $249.68 this batch cost you"
    # sitting in the payload. Found by running it, not by unit test — the
    # synthetic fixture had no filled templates.
    #
    # So: collect the actual VALUES that were withheld, and refuse if any of
    # them still appears anywhere in the serialized copy. This makes the
    # guarantee independent of which field happens to carry a number.
    survivors = _values_surviving(document, redacted, withhold)
    if survivors:
        raise DisclosureRefused(
            "these withheld values still appear in the copy, in prose or in a "
            "field not listed:\n  "
            + "\n  ".join(f"{value} (from {path})" for path, value in survivors)
            + "\n\nNothing was produced. Withhold the field that carries them "
              "too — `positions[].lots[].why` is the usual culprit, because a "
              "rating explains itself using that lot's own figures.")
    return redacted


def _collect(node, parts: list[str], out: list):
    """Every value a field path resolves to."""
    if not parts:
        if node is not None:
            out.append(node)
        return
    head, rest = parts[0], parts[1:]
    is_list = head.endswith("[]")
    key = head[:-2] if is_list else head
    if isinstance(node, dict):
        if key not in node:
            return
        value = node[key]
        if is_list:
            for item in value or []:
                _collect(item, rest, out)
        else:
            _collect(value, rest, out)
    elif isinstance(node, list):
        for item in node:
            _collect(item, parts, out)


def _values_surviving(original: dict, redacted: dict,
                      withhold: list[str]) -> list[tuple[str, str]]:
    import json
    blob = json.dumps(redacted, default=str)
    survivors = []
    for path in withhold:
        values: list = []
        _collect(original, path.split("."), values)
        for value in values:
            if not isinstance(value, (int, float)) or isinstance(value, bool):
                continue
            text = f"{value:g}"
            # Short values are skipped: a bare "1" or "45" occurs inside
            # "1.0.0", a date, or a holding-day count, so checking them yields
            # only false positives and would make every profile unusable. The
            # figures worth protecting here — prices, bases, break-evens — are
            # all longer. This is a safety net over field-path deletion, not a
            # substitute for withholding the field that carries the prose.
            if len(text) < 4:
                continue
            # Match the number as written, not as a substring of a longer one:
            # 85.5 must not be "found" inside 185.53.
            if re.search(rf"(?<![\d.]){re.escape(text)}(?![\d])", blob):
                survivors.append((path, text))
    return survivors


def verify_absent(rendered: str, values: list[str]) -> list[str]:
    """Which of `values` still appear in the delivered bytes.

    SC-011 is verified by searching the artefact, not by inspecting the page —
    this is the function that does it.
    """
    return [v for v in values if re.search(re.escape(str(v)), rendered)]
