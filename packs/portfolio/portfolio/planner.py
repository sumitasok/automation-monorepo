"""The arithmetic that answers "is selling this batch now cheap or expensive?"

Every formula here is ported verbatim from the vault program and cross-checked
against the page's own `calc()`. SC-001 requires the figures not to move, so
this module is the one place in the pack where "improving" the maths is a
defect rather than a contribution.

Break-even: the price at the maturity date that leaves you exactly as well off
as selling today at the short-term rate.

    net_now    = spot*fx - max(spot*fx - cb*acq_fx, 0) * stcg
    break_even = (net_now - ltcg * cb * acq_fx) / ((1 - ltcg) * fx_later)

Buy-back break-even answers a different question: sell and repurchase, how far
must the price fall before the same cash buys the share count back? Tax comes
out of the proceeds, so it does not.

    buy_back = (spot*fx - max(spot*fx - cb*acq_fx, 0) * rate) / fx

Deliberately excluded from buy-back, because both are assumptions rather than
arithmetic: the repurchased shares restart the holding-period clock (a cost),
and their basis resets higher (a benefit). Keeping both out makes the threshold
strict. Do not net them off without saying so on the page.
"""
from __future__ import annotations

from datetime import date

from .lots import as_date, human_wait, is_mature, matures_on, wait_days
from .rules import Rates


def net_proceeds(spot: float, fx: float, cb: float, acq_fx: float,
                 qty: float, rate: float) -> tuple[float, float, float]:
    """After-tax proceeds in reporting currency, plus the gain and tax.

    Both legs are converted, which is what captures FX movement — the
    foreign-currency P&L alone is not the taxable figure.
    """
    proceeds = spot * fx * qty
    cost = cb * acq_fx * qty
    gain = proceeds - cost
    tax = max(gain, 0.0) * rate
    return proceeds - tax, gain, tax


def breakeven_price(spot: float, fx: float, cb: float, acq_fx: float,
                    fx_later: float, rates: Rates) -> float:
    net_now = spot * fx - max(spot * fx - cb * acq_fx, 0.0) * rates.stcg
    return (net_now - rates.ltcg * cb * acq_fx) / ((1 - rates.ltcg) * fx_later)


def buyback_price(spot: float, fx: float, cb: float, acq_fx: float,
                  rate: float) -> float:
    gain_per_share = spot * fx - cb * acq_fx
    return (spot * fx - max(gain_per_share, 0.0) * rate) / fx


def analyse_lot(lot: dict, spot: float, fx: float, rates: Rates,
                when: date, fx_later: float | None = None) -> dict:
    """Every figure the page and the text report need for one lot."""
    fx_later = fx if fx_later is None else fx_later
    qty = lot["qty"]
    cb = lot["cb_per_share"]
    acq_fx = lot["acq_fx"]
    mature = is_mature(lot, rates.ltcg_months, when)

    _, gain, tax_now = net_proceeds(spot, fx, cb, acq_fx, qty, rates.rate(mature))
    _, _, tax_lt = net_proceeds(spot, fx, cb, acq_fx, qty, rates.ltcg)

    breakeven = None if mature else breakeven_price(spot, fx, cb, acq_fx, fx_later, rates)
    market_value = spot * fx * qty

    return {
        "id": lot.get("id", ""),
        "broker": lot.get("broker", ""),
        "funding": lot.get("funding", ""),
        "acq_date": as_date(lot["acq_date"]),
        "matures_on": matures_on(lot, rates.ltcg_months),
        "qty": qty,
        "cb": cb,
        "acq_fx": acq_fx,
        "mature": mature,
        "gain": gain,
        "gain_frac": (gain / market_value) if market_value else None,
        "tax_now": tax_now,
        "tax_lt": tax_lt,
        "saving": tax_now - tax_lt,
        "market_value": market_value,
        "breakeven": breakeven,
        "cushion": None if breakeven is None else breakeven / spot - 1,
        "price_vs_basis": (spot / cb) if cb else None,
        "wait_days": wait_days(lot, rates.ltcg_months, when),
        "buyback_st": buyback_price(spot, fx, cb, acq_fx, rates.stcg),
        "buyback_lt": buyback_price(spot, fx, cb, acq_fx, rates.ltcg),
        "flags": lot.get("flags", []),
    }


def build_ladder(position: dict, rates: Rates, when: date,
                 spot: float | None = None, fx: float | None = None,
                 fx_later: float | None = None) -> list[dict]:
    """Analyse every open lot in a position, oldest first.

    Returns [] when the position is unpriced — the caller reports it as such
    rather than substituting a zero (FR-016).
    """
    market = position.get("market") or {}
    spot = market.get("spot") if spot is None else spot
    if spot is None or fx is None:
        return []
    return [analyse_lot(lot, spot, fx, rates, when, fx_later)
            for lot in sorted(position.get("lots", []),
                              key=lambda l: as_date(l["acq_date"]))]


def totals(ladder: list[dict]) -> dict:
    """Portfolio- or position-level aggregates over an analysed ladder."""
    if not ladder:
        return {"qty": 0.0, "market_value": 0.0, "tax_now": 0.0,
                "tax_lt": 0.0, "saving": 0.0, "lots": 0}
    return {
        "qty": sum(l["qty"] for l in ladder),
        "market_value": sum(l["market_value"] for l in ladder),
        "tax_now": sum(l["tax_now"] for l in ladder),
        "tax_lt": sum(l["tax_lt"] for l in ladder),
        "saving": sum(l["saving"] for l in ladder),
        "lots": len(ladder),
    }


def rating_tokens(analysis: dict, ticker: str, spot: float, rates: Rates) -> dict:
    """Values for the `{token}` placeholders in a rating's `why` template.

    Every batch states its own numbers rather than a generic threshold, which
    is what FR-033 requires.
    """
    cushion = analysis["cushion"]
    breakeven = analysis["breakeven"]
    return {
        "ticker": ticker,
        "spot": f"${spot:,.2f}",
        "cb": f"${analysis['cb']:,.2f}",
        "qty": f"{analysis['qty']:g}",
        "breakeven": "—" if breakeven is None else f"${breakeven:,.2f}",
        "fall_pct": "—" if cushion is None else f"{abs(cushion):.1%}",
        "fall_usd": "—" if breakeven is None else f"${spot - breakeven:,.2f}",
        "mature_date": analysis["matures_on"].strftime("%d %b %Y"),
        "wait_days": str(analysis["wait_days"]),
        "wait_human": human_wait(analysis["wait_days"]),
        "pvb": "—" if analysis["price_vs_basis"] is None
               else f"{analysis['price_vs_basis']:.2f}x",
        "gain": _money(analysis["gain"]),
        "stcg_rate": f"{rates.stcg:.2%}",
        "ltcg_rate": f"{rates.ltcg:.2%}",
        "spread_pts": f"{rates.spread():.2%}",
        "saving": _money(analysis["saving"]),
        "tax_now": _money(analysis["tax_now"]),
        "tax_lt": _money(analysis["tax_lt"]),
        "tax_if_early": _money(max(analysis["gain"], 0.0) * rates.stcg),
    }


def _money(value: float) -> str:
    magnitude = abs(value)
    if magnitude >= 1e7:
        return f"₹{value / 1e7:.2f} Cr"
    if magnitude >= 1e5:
        return f"₹{value / 1e5:.2f} L"
    return f"₹{round(value):,.0f}"
