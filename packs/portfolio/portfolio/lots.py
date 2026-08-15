"""Generic lot arithmetic: maturity, ordering, FIFO consumption, splitting.

Nothing here knows about a broker, an instrument, or a jurisdiction. The
holding-period threshold and the matching convention arrive as arguments,
sourced from the rules file.
"""
from __future__ import annotations

from datetime import date, datetime, timedelta

MONTH_DAYS = 30.436875


def as_date(value) -> date:
    if isinstance(value, datetime):
        return value.date()
    if isinstance(value, date):
        return value
    return datetime.strptime(str(value)[:10], "%Y-%m-%d").date()


def add_months(d: date, n: int) -> date:
    """Calendar-correct month addition, clamping to the end of a short month.

    Ported verbatim from the vault program: maturity dates must not move, and
    a naive 30-day approximation would shift every one of them.
    """
    y, m = divmod(d.year * 12 + (d.month - 1) + n, 12)
    leap = y % 4 == 0 and (y % 100 or y % 400 == 0)
    dim = [31, 29 if leap else 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31][m]
    return date(y, m + 1, min(d.day, dim))


def months_between(a: date, b: date) -> float:
    return (b - a).days / MONTH_DAYS


def matures_on(lot: dict, ltcg_months: int) -> date:
    """DERIVED, never stored — see contracts/README.md. Storing it would let a
    register drift from the rules that define it."""
    return add_months(as_date(lot["acq_date"]), ltcg_months)


def is_mature(lot: dict, ltcg_months: int, when: date) -> bool:
    return matures_on(lot, ltcg_months) <= when


def wait_days(lot: dict, ltcg_months: int, when: date) -> int:
    return max(0, (matures_on(lot, ltcg_months) - when).days)


def human_wait(days: int) -> str:
    if days <= 0:
        return "already"
    if days < 45:
        return f"{days} days"
    months = round(days / MONTH_DAYS)
    if months < 18:
        return f"{months} months"
    return f"{months // 12}y {months % 12}m" if months % 12 else f"{months // 12} years"


def fifo_order(lots: list[dict], tiebreak: list[str], before: date) -> list[dict]:
    """Oldest first, with `tiebreak` (funding keys) deciding same-date order.

    `fifo_from` hides a lot from disposals before that date — used when the
    broker's own specific-lot identification kept a lot untouched in an earlier
    period, so a replayed FIFO stays faithful to what actually happened.
    """
    rank = {key: i for i, key in enumerate(tiebreak)}
    eligible = [
        lot for lot in lots
        if not lot.get("fifo_from") or as_date(lot["fifo_from"]) <= before
    ]
    return sorted(
        eligible,
        key=lambda l: (as_date(l["acq_date"]),
                       rank.get(l.get("funding"), len(rank)),
                       str(l.get("id", ""))),
    )


def find_lapse_lot(lots: list[dict], sell_date: date, window_days: int,
                   tiebreak: list[str]) -> dict | None:
    """A disposal landing within `window_days` AFTER a vest is that vest's
    withholding sale — specific identification, not FIFO. Ties go to the
    funding class earliest in `tiebreak` (RSU before ESPP)."""
    rank = {key: i for i, key in enumerate(tiebreak)}
    candidates = [
        lot for lot in lots
        if 0 <= (sell_date - as_date(lot["acq_date"])).days <= window_days
    ]
    if not candidates:
        return None
    return sorted(
        candidates,
        key=lambda l: (-(as_date(l["acq_date"]).toordinal()),
                       rank.get(l.get("funding"), len(rank))),
    )[0]


def split(lot: dict, qty: float) -> tuple[dict, dict | None]:
    """Consume `qty` from `lot`.

    Returns (consumed, remainder). `remainder` is None when the lot is used up.
    Both carry the lot's own acquisition facts, so the resulting disposal is
    self-contained (contracts/README.md).
    """
    if qty > lot["qty"] + 1e-9:
        raise ValueError(
            f"lot {lot.get('id')} holds {lot['qty']}, cannot consume {qty}")
    consumed = dict(lot, qty=qty)
    if abs(lot["qty"] - qty) < 1e-9:
        return consumed, None
    return consumed, dict(lot, qty=lot["qty"] - qty)


def to_disposal(consumed: dict, disp_date: date, disp_price: float,
                disp_fx: float, ltcg_months: int, *, lapse: bool = False,
                fees: float = 0.0, src: str = "") -> dict:
    """Build a Disposal from a consumed lot. Acquisition facts are COPIED, not
    referenced, so a consumer never has to resolve a lot that no longer exists.
    """
    acq = as_date(consumed["acq_date"])
    holding = (disp_date - acq).days
    disposal = {
        "from_lot": consumed.get("id", ""),
        "qty": consumed["qty"],
        "acq_date": acq.isoformat(),
        "cb_per_share": consumed["cb_per_share"],
        "acq_fx": consumed["acq_fx"],
        "funding": consumed["funding"],
        "disp_date": disp_date.isoformat(),
        "disp_price": disp_price,
        "disp_fx": disp_fx,
        "holding_days": holding,
        "long_term": matures_on(consumed, ltcg_months) <= disp_date,
        "src": src or consumed.get("src", ""),
    }
    if fees:
        disposal["fees"] = fees
    if lapse:
        disposal["lapse"] = True
    if consumed.get("flags"):
        disposal["flags"] = consumed["flags"]
    return disposal


def near_maturity_warning(disposal: dict, ltcg_months: int,
                          threshold_days: int = 60) -> str | None:
    """FR-028: a disposal shortly before maturity costs real money and is
    invisible after the fact, so it must be said out loud at import time."""
    matured = add_months(as_date(disposal["acq_date"]), ltcg_months)
    disp = as_date(disposal["disp_date"])
    if disposal["long_term"]:
        return None
    short_by = (matured - disp).days
    if 0 < short_by <= threshold_days:
        return (f"sold {short_by} days BEFORE maturity "
                f"({disposal['qty']:g} from {disposal['from_lot']}, "
                f"would have matured {matured.isoformat()})")
    return None
