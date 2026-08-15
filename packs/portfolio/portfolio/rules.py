"""Tax constants and FX resolution — all of it data-driven.

No rate, threshold or matching convention is written in Python here; each is
read from the rules file so a change is a data edit (FR-031).
"""
from __future__ import annotations

from datetime import date

from .lots import as_date


class Rates:
    """Effective reporting-currency rates for one asset class.

    The LTCG derivation is ported verbatim and is NOT a plain rate: gains under
    s.112 get a capped surcharge, then cess. For the shipped rules that is
    0.125 x 1.15 x 1.04 = 0.1495. Flattening this to a single literal would
    silently drop the cap the moment either input changed.
    """

    def __init__(self, rules: dict, asset_class: str = "foreign_equity"):
        try:
            cg = rules["capital_gains"][asset_class]
        except KeyError as exc:
            raise KeyError(
                f"rules file has no capital_gains.{asset_class}; "
                f"a new asset class is a rules edit, not a code change"
            ) from exc
        self.asset_class = asset_class
        self.ltcg_months = cg["ltcg_after_months"]
        self.stcg = cg["stcg_rate"]
        cess = rules.get("cess", 0.04)
        self.ltcg = (cg["ltcg_rate"]
                     * (1 + cg.get("ltcg_surcharge_cap", 0.15))
                     * (1 + cess))
        matching = rules["capital_gains"]["lot_matching"]
        self.method = matching["method"]
        self.tiebreak = matching.get("same_date_tiebreak", [])
        self.lapse_window = matching.get("lapse_sale_window_days", 5)

    def rate(self, long_term: bool) -> float:
        return self.ltcg if long_term else self.stcg

    def spread(self) -> float:
        return self.stcg - self.ltcg


class RateTable:
    """Dated FX with provenance.

    A lookup resolving to anything other than a verified source returns an
    `fx_interpolated` flag alongside the number, so the estimate stays visible
    all the way to the page (FR-029, SC-015).
    """

    # An employer slip rate is definitive for the lot it belongs to, so it
    # counts as verified alongside a looked-up SBI TT rate.
    VERIFIED = {"sbi-tt", "rsu-slip"}

    def __init__(self, table: dict):
        # Two entry shapes, both in real use: a single {date, rate} and a
        # {from, to, rate} window covering a period. Normalising to (start,
        # end) here keeps the lookup one code path.
        entries = []
        for e in table.get("rates", []):
            if "date" in e:
                start = end = as_date(e["date"])
            elif "from" in e and "to" in e:
                start, end = as_date(e["from"]), as_date(e["to"])
            else:
                continue
            entries.append({"start": start, "end": end, "rate": float(e["rate"]),
                            "source": e.get("source", "manual"),
                            "exact": "date" in e})
        self.entries = sorted(entries, key=lambda e: e["start"])

    def lookup(self, when) -> tuple[float | None, dict | None]:
        """Returns (rate, flag). A covering entry wins; else the nearest prior."""
        if not self.entries:
            return None, None
        when = as_date(when)
        covering = [e for e in self.entries if e["start"] <= when <= e["end"]]
        entry = covering[0] if covering else None
        if entry is None:
            prior = [e for e in self.entries if e["end"] <= when]
            if not prior:
                return None, None
            entry = prior[-1]
        covered = entry["start"] <= when <= entry["end"]
        flag = None
        if entry["source"] not in self.VERIFIED or not covered:
            reason = (f"source is {entry['source']!r}"
                      if entry["source"] not in self.VERIFIED
                      else f"nearest prior entry ends {entry['end'].isoformat()}")
            flag = {
                "code": "fx_interpolated",
                "note": (f"FX {entry['rate']} for {when.isoformat()} is not a "
                         f"verified same-day rate ({reason}). Verify before filing."),
                "raised_by": "import",
            }
        return entry["rate"], flag


def evaluate_predicate(when: dict, metrics: dict) -> bool:
    """A rating's `when` block: metric+comparator keys, ANDed. Empty matches.

    An unknown metric or comparator raises rather than skipping — a typo in a
    threshold must not quietly make a rating never fire (data-model.md rule 5).
    """
    comparators = {
        "lt": lambda a, b: a < b,
        "lte": lambda a, b: a <= b,
        "gt": lambda a, b: a > b,
        "gte": lambda a, b: a >= b,
        "eq": lambda a, b: a == b,
    }
    for key, expected in (when or {}).items():
        metric, _, cmp_name = key.rpartition("_")
        if cmp_name not in comparators:
            raise ValueError(
                f"rating predicate {key!r}: unknown comparator {cmp_name!r} "
                f"(expected one of {', '.join(sorted(comparators))})")
        if metric not in metrics:
            raise ValueError(
                f"rating predicate {key!r}: unknown metric {metric!r} "
                f"(available: {', '.join(sorted(metrics))})")
        actual = metrics[metric]
        if actual is None:
            return False
        if not comparators[cmp_name](actual, expected):
            return False
    return True
