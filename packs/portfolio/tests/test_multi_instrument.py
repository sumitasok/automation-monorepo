"""US2 — any instrument, with no behaviour change.

The point of these tests is that adding an instrument is DATA. If any of them
required touching a module to pass, the seam would not be real.
"""
import unittest
from datetime import date

from portfolio import document as doclib

RULES = {
    "capital_gains": {
        "foreign_equity": {"ltcg_after_months": 24, "stcg_rate": 0.39,
                           "ltcg_rate": 0.125, "ltcg_surcharge_cap": 0.15},
        "lot_matching": {"method": "fifo",
                         "same_date_tiebreak": ["RSU", "ESPP", "MARKET"],
                         "lapse_sale_window_days": 5},
    },
    "cess": 0.04,
    "lot_ratings": [
        {"code": "SELL_OK", "label": "Sell first", "icon": "v", "tone": "ok",
         "when": {"mature_eq": True}, "cond": "Past 24 months",
         "plain": "The low rate applies.",
         "why": "Past 24 months, so {ltcg_rate} already applies to {qty} shares."},
        {"code": "HOLD", "label": "Hold", "icon": "*", "tone": "caution",
         "when": {"cushion_lte": -0.05}, "cond": "5-10% of room",
         "plain": "A usable margin.",
         "why": "Can fall {fall_pct} to {breakeven} before waiting stops paying."},
        {"code": "NEUTRAL", "label": "Tax-neutral", "icon": "o", "tone": "neutral",
         "when": {}, "cond": "Less than 5%", "plain": "Too little to matter.",
         "why": "Only {fall_pct} of room by {mature_date}."},
    ],
}


def lot(lid, acq, qty=10.0, cb=100.0, fx=85.0, funding="MARKET"):
    return {"id": lid, "broker": "b", "acq_date": acq, "qty": qty,
            "cb_per_share": cb, "price_paid_per_share": cb, "acq_fx": fx,
            "funding": funding, "src": lid, "confirmed": True}


def register(positions):
    return {"contract_version": "1.0.0",
            "generated_at": "2026-08-15T00:00:00+00:00",
            "reporting_currency": "INR",
            "funding": {"MARKET": {"label": "Own money", "own_money": "full",
                                   "desc": "d"},
                        "RSU": {"label": "RSU", "own_money": "none", "desc": "d"}},
            "positions": positions}


PRICED = {"broker": "b", "currency": "USD",
          "market": {"spot": 150.0, "as_of": "2026-08-15"},
          "lots": [lot("L1", "2025-06-15")], "closed": []}
SECOND = {"broker": "b", "currency": "USD",
          "market": {"spot": 60.0, "as_of": "2026-08-15"},
          "lots": [lot("M1", "2024-01-10", qty=5.0, cb=50.0)], "closed": []}
UNPRICED = {"broker": "b", "currency": "USD",
            "lots": [lot("U1", "2025-06-15")], "closed": []}

WHEN = date(2026, 8, 15)


class TestMultipleInstruments(unittest.TestCase):
    def test_second_instrument_needs_no_code_change(self):
        doc = doclib.build(register({"AAA": PRICED, "BBB": SECOND}),
                           RULES, when=WHEN, fx=85.5)
        self.assertEqual([p["ticker"] for p in doc["positions"]], ["AAA", "BBB"])
        for position in doc["positions"]:
            self.assertTrue(position["priced"])
            self.assertTrue(position["lots"][0]["rating"])

    def test_scoping_to_one_instrument(self):
        doc = doclib.build(register({"AAA": PRICED, "BBB": SECOND}),
                           RULES, when=WHEN, fx=85.5, ticker="BBB")
        self.assertEqual([p["ticker"] for p in doc["positions"]], ["BBB"])

    def test_each_instrument_keeps_its_own_attribution(self):
        doc = doclib.build(register({"AAA": PRICED, "BBB": SECOND}),
                           RULES, when=WHEN, fx=85.5)
        ids = {p["ticker"]: [l["id"] for l in p["lots"]] for p in doc["positions"]}
        self.assertEqual(ids, {"AAA": ["L1"], "BBB": ["M1"]})


class TestUnpricedPositions(unittest.TestCase):
    def test_unpriced_is_reported_not_zeroed(self):
        """FR-016 — the difference between 'no price' and 'worth nothing'."""
        doc = doclib.build(register({"AAA": PRICED, "ZZZ": UNPRICED}),
                           RULES, when=WHEN, fx=85.5)
        unpriced = next(p for p in doc["positions"] if p["ticker"] == "ZZZ")
        self.assertFalse(unpriced["priced"])
        self.assertNotIn("spot", unpriced)
        self.assertEqual(len(unpriced["lots"]), 1,
                         "the lots are still visible, just not valued")
        self.assertEqual(unpriced["lots"][0]["rating"], "UNPRICED")

    def test_unpriced_carries_no_computed_figure(self):
        doc = doclib.build(register({"ZZZ": UNPRICED}), RULES, when=WHEN, fx=85.5)
        entry = doc["positions"][0]["lots"][0]
        for field in ("breakeven", "cushion", "buyback_st"):
            self.assertNotIn(field, entry,
                             f"{field} cannot exist without a price")

    def test_summary_counts_unpriced_separately(self):
        doc = doclib.build(register({"AAA": PRICED, "ZZZ": UNPRICED}),
                           RULES, when=WHEN, fx=85.5)
        summary = doclib.summarise(doc)
        self.assertEqual(summary["instruments"], 2)
        self.assertEqual(summary["unpriced"], 1)
        self.assertEqual(summary["lots"], 1, "unpriced lots excluded from totals")


class TestRatingsAreData(unittest.TestCase):
    def test_a_new_rating_needs_only_a_rules_edit(self):
        rules = {**RULES, "lot_ratings": [
            {"code": "CUSTOM", "label": "Custom", "icon": "!", "tone": "buy",
             "when": {"price_vs_basis_gt": 1.2}, "cond": "c", "plain": "p",
             "why": "spot is {pvb} of the {cb} basis"},
            *RULES["lot_ratings"]]}
        doc = doclib.build(register({"AAA": PRICED}), rules, when=WHEN, fx=85.5)
        self.assertEqual(doc["positions"][0]["lots"][0]["rating"], "CUSTOM")

    def test_why_states_this_lots_own_numbers(self):
        doc = doclib.build(register({"AAA": PRICED}), RULES, when=WHEN, fx=85.5)
        why = doc["positions"][0]["lots"][0]["why"]
        self.assertNotIn("{", why, "an unfilled token means a typo in the rules")
        self.assertTrue(any(ch.isdigit() for ch in why))

    def test_unknown_metric_is_an_error_not_a_silent_skip(self):
        rules = {**RULES, "lot_ratings": [
            {"code": "BAD", "label": "b", "icon": "x", "tone": "go",
             "when": {"nonsense_lt": 1}, "cond": "c", "plain": "p",
             "why": "{qty}"}, *RULES["lot_ratings"]]}
        with self.assertRaises(ValueError) as ctx:
            doclib.build(register({"AAA": PRICED}), rules, when=WHEN, fx=85.5)
        self.assertIn("nonsense", str(ctx.exception))

    def test_rules_without_a_catch_all_are_rejected(self):
        rules = {**RULES, "lot_ratings": [RULES["lot_ratings"][0]]}
        with self.assertRaises(ValueError):
            doclib.build(register({"AAA": PRICED}), rules, when=WHEN, fx=85.5)


if __name__ == "__main__":
    unittest.main()
