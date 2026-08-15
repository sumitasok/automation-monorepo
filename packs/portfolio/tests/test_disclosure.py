"""SC-011 and FR-047/049 — the privacy gate.

The only irreversible mistake this pack can make is shipping a figure someone
thought was withheld. So absence is verified in the DELIVERED BYTES, not by
inspecting a rendered page: a page that hides a number still carries it.
"""
import json
import re
import unittest
from pathlib import Path

from portfolio import disclosure, page

TEMPLATE = Path(__file__).resolve().parent.parent / "templates" / "explorer.html"

DOC = {
    "contract_version": "1.0.0",
    "generated_at": "2026-08-15T00:00:00+00:00",
    "valuation_date": "2026-08-15",
    "reporting_currency": "INR",
    "disclosure": "full",
    "withheld": [],
    "rules": {"stcg": 0.39, "ltcg": 0.1495, "ltcg_months": 24},
    "funding": {"RSU": {"label": "RSU vest", "own_money": "none", "desc": "x"}},
    "ratings": [{"code": "HOLD", "label": "Hold", "icon": "*", "tone": "caution",
                 "cond": "c", "plain": "p"}],
    "flags_present": False,
    "positions": [{
        "ticker": "AVGO", "currency": "USD", "priced": True,
        "spot": 427.76, "fx": 85.5,
        "lots": [{"id": "L1", "acq": "2025-06-15", "mat": "2027-06-15",
                  "funding": "RSU", "rating": "HOLD", "why": "because",
                  "qty": 178.0, "cb": 249.68, "afx": 85.75,
                  "breakeven": 377.61, "cushion": -0.117}],
        "closed": [],
    }],
}

DERIVATIONS = [
    {"target": "positions[].lots[].cb",
     "from": ["positions[].lots[].qty", "positions[].lots[].breakeven"],
     "note": "quantity and any per-share price recover the basis"},
]


class TestRedactionRemoves(unittest.TestCase):
    def test_withheld_values_absent_from_delivered_bytes(self):
        profile = {"_name": "hidden", "withhold": [
            "positions[].lots[].qty", "positions[].lots[].cb",
            "positions[].lots[].afx", "positions[].lots[].breakeven",
            "positions[].spot", "positions[].fx"]}
        redacted = disclosure.apply(DOC, profile, DERIVATIONS)
        html = page.render(TEMPLATE, redacted)

        leaked = disclosure.verify_absent(
            html, ["178.0", "249.68", "85.75", "377.61", "427.76"])
        self.assertEqual(leaked, [],
                         f"withheld figures still present in the file: {leaked}")

    def test_decision_shaped_content_survives(self):
        """Redaction must leave something worth sharing (FR-046)."""
        profile = {"_name": "hidden", "withhold": [
            "positions[].lots[].qty", "positions[].lots[].cb",
            "positions[].lots[].afx", "positions[].lots[].breakeven",
            "positions[].spot", "positions[].fx"]}
        redacted = disclosure.apply(DOC, profile, DERIVATIONS)
        lot = redacted["positions"][0]["lots"][0]
        self.assertEqual(lot["rating"], "HOLD")
        self.assertIn("cushion", lot, "the cushion is the point of sharing")
        self.assertIn("mat", lot)

    def test_withheld_fields_are_absent_not_null(self):
        """A null still reveals the field exists and, in aggregate, leaks
        structure (data-model.md)."""
        profile = {"_name": "hidden", "withhold": ["positions[].lots[].qty"]}
        redacted = disclosure.apply(DOC, profile, [])
        self.assertNotIn("qty", redacted["positions"][0]["lots"][0])

    def test_document_records_what_was_withheld(self):
        profile = {"_name": "hidden", "withhold": ["positions[].lots[].qty"]}
        redacted = disclosure.apply(DOC, profile, [])
        self.assertEqual(redacted["withheld"], ["positions[].lots[].qty"])
        self.assertEqual(redacted["disclosure"], "hidden")


class TestDerivabilityRefusal(unittest.TestCase):
    def test_refuses_a_profile_that_leaks_via_arithmetic(self):
        """The naive first profile: withhold the basis, keep quantity and a
        per-share price. That withholds nothing."""
        profile = {"_name": "naive", "withhold": ["positions[].lots[].cb"]}
        with self.assertRaises(disclosure.DisclosureRefused) as ctx:
            disclosure.apply(DOC, profile, DERIVATIONS)
        self.assertIn("reconstruct", str(ctx.exception))

    def test_accepts_when_the_reconstructing_fields_go_too(self):
        profile = {"_name": "ok", "withhold": [
            "positions[].lots[].cb", "positions[].lots[].qty",
            "positions[].lots[].breakeven"]}
        redacted = disclosure.apply(DOC, profile, DERIVATIONS)
        self.assertNotIn("cb", redacted["positions"][0]["lots"][0])

    def test_full_profile_passes_through_untouched(self):
        redacted = disclosure.apply(DOC, {"_name": "full", "withhold": []}, [])
        self.assertEqual(redacted["withheld"], [])
        self.assertEqual(redacted["positions"][0]["lots"][0]["qty"], 178.0)


class TestShippedProfiles(unittest.TestCase):
    """The profiles the pack actually ships must survive their own check."""

    def test_figures_hidden_is_accepted_and_leaks_nothing(self):
        import yaml
        path = (Path(__file__).resolve().parents[3]
                / "data" / "portfolio" / "disclosure.yaml")
        if not path.exists():
            self.skipTest("workspace data not present")
        cfg = yaml.safe_load(path.read_text())
        profile = dict(cfg["profiles"]["figures-hidden"], _name="figures-hidden")
        redacted = disclosure.apply(DOC, profile, cfg.get("derivations", []))
        html = page.render(TEMPLATE, redacted)
        leaked = disclosure.verify_absent(html, ["178.0", "249.68", "427.76"])
        self.assertEqual(leaked, [], f"shipped profile leaks: {leaked}")


if __name__ == "__main__":
    unittest.main()
