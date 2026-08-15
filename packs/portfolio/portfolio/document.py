"""Building the explorer data document — the page's sole input (FR-036).

A projection of the register, regenerated on every run. Never a second source
of truth, never hand-edited.
"""
from __future__ import annotations

from datetime import date, datetime, timezone

from . import ratings as ratinglib
from .planner import build_ladder, totals
from .rules import Rates

CONTRACT_VERSION = "1.0.0"


def build(register: dict, rules: dict, *, when: date,
          fx: float, ticker: str | None = None,
          spot_override: float | None = None,
          disclosure: str = "full") -> dict:
    rates = Rates(rules)
    definitions = rules.get("lot_ratings", [])

    problems = ratinglib.validate_definitions(definitions)
    if problems:
        raise ValueError("rules file problems:\n  " + "\n  ".join(problems))

    positions_out = []
    any_flag = False

    for tick, position in sorted((register.get("positions") or {}).items()):
        if ticker and tick != ticker.upper():
            continue

        market = position.get("market") or {}
        spot = spot_override if spot_override is not None else market.get("spot")
        priced = spot is not None

        entry = {
            "ticker": tick,
            "currency": position.get("currency", "USD"),
            "priced": priced,
            "lots": [],
            "closed": [],
        }
        if priced:
            entry["spot"] = spot
            entry["fx"] = fx
            ladder = build_ladder(position, rates, when, spot=spot, fx=fx)
            for analysis in ladder:
                rated = ratinglib.apply(analysis, definitions, tick, spot, rates)
                if rated["flags"]:
                    any_flag = True
                entry["lots"].append(_doc_lot(rated))
        else:
            # Unpriced: keep the lots visible but carry no computed figure.
            # Excluded from value totals, never valued at zero (FR-016).
            for lot in position.get("lots", []):
                entry["lots"].append({
                    "id": lot.get("id", ""),
                    "broker": lot.get("broker", ""),
                    "acq": str(lot["acq_date"])[:10],
                    "mat": _mat(lot, rates),
                    "funding": lot.get("funding", ""),
                    "rating": "UNPRICED",
                    "qty": lot["qty"],
                    "cb": lot["cb_per_share"],
                    "afx": lot["acq_fx"],
                })

        for disposal in position.get("closed", []):
            entry["closed"].append({
                "from_lot": disposal["from_lot"],
                "acq": str(disposal["acq_date"])[:10],
                "disp": str(disposal["disp_date"])[:10],
                "long_term": disposal["long_term"],
                "holding_days": disposal["holding_days"],
                "qty": disposal["qty"],
                "cb": disposal["cb_per_share"],
                "price": disposal["disp_price"],
                "lapse": bool(disposal.get("lapse", False)),
            })

        positions_out.append(entry)

    return {
        "contract_version": CONTRACT_VERSION,
        "generated_at": datetime.now(timezone.utc).isoformat(timespec="seconds"),
        "valuation_date": when.isoformat(),
        "reporting_currency": register.get("reporting_currency", "INR"),
        "disclosure": disclosure,
        "withheld": [],
        "rules": {"stcg": rates.stcg, "ltcg": rates.ltcg,
                  "ltcg_months": rates.ltcg_months},
        "funding": register.get("funding", {}),
        "ratings": [{k: d[k] for k in ("code", "label", "icon", "tone", "cond", "plain")}
                    for d in definitions],
        "flags_present": any_flag,
        "positions": positions_out,
    }


def _mat(lot, rates):
    from .lots import matures_on
    return matures_on(lot, rates.ltcg_months).isoformat()


def _doc_lot(rated: dict) -> dict:
    out = {
        "id": rated["id"],
        "broker": rated["broker"],
        "acq": rated["acq_date"].isoformat(),
        "mat": rated["matures_on"].isoformat(),
        "funding": rated["funding"],
        "rating": rated["rating"],
        "why": rated["why"],
        "qty": rated["qty"],
        "cb": rated["cb"],
        "afx": rated["acq_fx"],
        "buyback_st": round(rated["buyback_st"], 4),
        "buyback_lt": round(rated["buyback_lt"], 4),
    }
    if rated["breakeven"] is not None:
        out["breakeven"] = round(rated["breakeven"], 4)
        out["cushion"] = round(rated["cushion"], 6)
    if rated["flags"]:
        out["flags"] = [{"code": f["code"], "note": f["note"]}
                        for f in rated["flags"]]
    return out


def summarise(document: dict) -> dict:
    """Portfolio-level aggregates over a built document, for the text report."""
    priced = [p for p in document["positions"] if p.get("priced")]
    return {
        "instruments": len(document["positions"]),
        "unpriced": len(document["positions"]) - len(priced),
        "lots": sum(len(p["lots"]) for p in priced),
        "shares": sum(l.get("qty", 0) for p in priced for l in p["lots"]),
    }
