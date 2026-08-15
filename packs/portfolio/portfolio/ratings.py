"""Rating a lot: which verdict fires, and why in that lot's own numbers.

The rules file owns both the thresholds and the prose. Adding a rating, or
moving a threshold, is a data edit (FR-032). This module only evaluates.

Colour discipline (FR-034): each rating carries one `tone`, and that tone drives
the pill, the card border and the gauge together, so all three always agree.
Do not reuse a tone for a different meaning.
"""
from __future__ import annotations

import re

from .planner import rating_tokens
from .rules import evaluate_predicate

TOKEN = re.compile(r"\{(\w+)\}")


def metrics_for(analysis: dict) -> dict:
    """The metric surface a rating predicate may test against."""
    return {
        "mature": analysis["mature"],
        "cushion": analysis["cushion"],
        "price_vs_basis": analysis["price_vs_basis"],
        "gain_frac": analysis["gain_frac"],
        "wait_days": analysis["wait_days"],
    }


def choose(analysis: dict, definitions: list[dict]) -> dict:
    """First match wins, evaluated top-down.

    A rules file whose last entry is not a catch-all can leave a lot unrated;
    that is a data error and is reported as such rather than defaulting.
    """
    metrics = metrics_for(analysis)
    for definition in definitions:
        if evaluate_predicate(definition.get("when", {}), metrics):
            return definition
    raise ValueError(
        f"lot {analysis.get('id')!r} matched no rating. The last entry in "
        f"lot_ratings must be a catch-all with an empty `when`."
    )


def fill(template: str, tokens: dict) -> str:
    """Substitute {tokens}. An unknown token is left visible rather than
    blanked, so a typo in the rules file shows up instead of silently
    producing a sentence with a hole in it."""
    return TOKEN.sub(lambda m: tokens.get(m.group(1), m.group(0)), template or "")


def apply(analysis: dict, definitions: list[dict], ticker: str,
          spot: float, rates) -> dict:
    """Attach `rating`, `tone`, `icon`, `label` and a filled `why` to an
    analysed lot."""
    definition = choose(analysis, definitions)
    tokens = rating_tokens(analysis, ticker, spot, rates)
    return dict(
        analysis,
        rating=definition["code"],
        rating_label=definition.get("label", definition["code"]),
        icon=definition.get("icon", ""),
        tone=definition.get("tone", "neutral"),
        why=fill(definition.get("why", ""), tokens),
    )


def validate_definitions(definitions: list[dict]) -> list[str]:
    """Data-model rules 4 and 5, checked before anything is computed.

    `why` is a per-lot template and must carry at least one token; `cond` and
    `plain` are shown un-filled in the legend and must carry none.
    """
    problems: list[str] = []
    if not definitions:
        return ["rules: lot_ratings is empty"]

    for definition in definitions:
        code = definition.get("code", "<unnamed>")
        if not TOKEN.search(definition.get("why", "")):
            problems.append(
                f"rating {code}: `why` has no {{token}} — it would state a "
                f"generic threshold instead of this lot's own numbers (FR-033)")
        for field in ("cond", "plain"):
            if TOKEN.search(definition.get(field, "")):
                problems.append(
                    f"rating {code}: `{field}` contains a {{token}} but is "
                    f"shown un-filled in the legend")
        if definition.get("tone") not in {"go", "ok", "caution", "stop",
                                          "neutral", "buy"}:
            problems.append(
                f"rating {code}: tone {definition.get('tone')!r} is not one of "
                f"go/ok/caution/stop/neutral/buy")

    if definitions[-1].get("when"):
        problems.append(
            "rules: the last lot_ratings entry must be a catch-all with an "
            "empty `when`, otherwise some lots can match nothing")
    return problems
