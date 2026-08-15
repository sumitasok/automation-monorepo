"""Import: one contract for two export shapes, and the invariants that keep it honest.

The qty_sign test is the one that matters most (research R-003): a broker with
no Buy/Sell action string encodes direction in the sign of the quantity. Get it
wrong and every trade routes to the same event — silently, with plausible-looking
output.
"""
import tempfile
import unittest
from pathlib import Path

import yaml

from portfolio import importer, profiles
from portfolio.rules import RateTable

PACK = Path(__file__).resolve().parent.parent

RULES = {
    "capital_gains": {
        "foreign_equity": {"ltcg_after_months": 24, "stcg_rate": 0.39,
                           "ltcg_rate": 0.125, "ltcg_surcharge_cap": 0.15},
        "lot_matching": {"method": "fifo",
                         "same_date_tiebreak": ["RSU", "ESPP", "MARKET"],
                         "lapse_sale_window_days": 5},
    },
    "cess": 0.04,
}
FX = RateTable({"rates": [{"from": "2020-01-01", "to": "2030-01-01",
                           "rate": 85.0, "source": "sbi-tt"}]})

SCHWAB_CSV = """\
"Transactions for account XXXX"
"Date","Action","Symbol","Description","Quantity","Price","Fees & Comm","Amount"
"06/15/2025","Buy","AVGO","BROADCOM","10","$250.00","$0.00","-$2,500.00"
"07/20/2025","Sell","AVGO","BROADCOM","4","$300.00","$0.00","$1,200.00"
"07/25/2025","Cash Dividend","AVGO","DIVIDEND","","","","$52.00"
"08/01/2025","Bananas","AVGO","NONSENSE","1","$1.00","","$1.00"
"""

IBKR_CSV = """\
Trades,Header,DataDiscriminator,Asset Category,Currency,Symbol,Date/Time,Quantity,T. Price,Comm/Fee,Proceeds,Code
Trades,Data,Order,Stocks,USD,AAPL,"2025-06-15, 10:00:00",10,150.00,-1.00,-1500.00,O
Trades,Data,Order,Stocks,USD,AAPL,"2025-07-20, 10:00:00",-4,180.00,-1.00,720.00,C
Trades,SubTotal,,Stocks,USD,AAPL,,6,,,,
"""


def load_profile(name):
    return yaml.safe_load((PACK / "profiles" / f"{name}.yaml").read_text())


def write(text):
    fh = tempfile.NamedTemporaryFile("w", suffix=".csv", delete=False)
    fh.write(text)
    fh.close()
    return Path(fh.name)


def empty_register():
    return {"contract_version": "1.0.0", "generated_at": "2026-08-15T00:00:00+00:00",
            "reporting_currency": "INR",
            "funding": {"MARKET": {"label": "Own money", "own_money": "full",
                                   "desc": "x"},
                        "RSU": {"label": "RSU", "own_money": "none", "desc": "x"}},
            "positions": {}}


class TestRowAccounting(unittest.TestCase):
    def test_every_row_is_accounted_for(self):
        """SC-007: nothing is dropped without being reported."""
        batch = importer.run(write(SCHWAB_CSV), load_profile("schwab"),
                             empty_register(), RULES, FX, dry_run=True)
        total = (len(batch["created"]) + len(batch["matched"]) +
                 len(batch["skipped"]) + len(batch["unrecognised"]) +
                 len(batch["ignored"]))
        self.assertEqual(total, batch["rows_read"])

    def test_unknown_action_is_reported_not_dropped(self):
        batch = importer.run(write(SCHWAB_CSV), load_profile("schwab"),
                             empty_register(), RULES, FX, dry_run=True)
        buckets = [i["bucket"] for i in batch["ignored"]]
        self.assertIn("unmatched", buckets,
                      "the nonsense row must land in the catch-all, visibly")


class TestIdempotence(unittest.TestCase):
    def test_reimport_changes_nothing(self):
        """SC-006. Dedupe is by src fingerprint, never (date, qty)."""
        path, profile = write(SCHWAB_CSV), load_profile("schwab")
        register = empty_register()

        importer.run(path, profile, register, RULES, FX)
        import copy
        snapshot = copy.deepcopy(register)

        second = importer.run(path, profile, register, RULES, FX)
        self.assertEqual(register, snapshot, "re-import mutated the register")
        self.assertEqual(len(second["created"]), 0)
        self.assertEqual(len(second["matched"]), 0)
        self.assertGreater(len(second["skipped"]), 0)


class TestFifoAndSplitting(unittest.TestCase):
    def test_sell_consumes_and_splits(self):
        register = empty_register()
        importer.run(write(SCHWAB_CSV), load_profile("schwab"),
                     register, RULES, FX)
        position = register["positions"]["AVGO"]
        self.assertEqual(len(position["lots"]), 1)
        self.assertAlmostEqual(position["lots"][0]["qty"], 6.0)   # 10 bought, 4 sold
        self.assertEqual(len(position["closed"]), 1)
        self.assertAlmostEqual(position["closed"][0]["qty"], 4.0)

    def test_disposal_is_self_contained(self):
        """It carries the acquisition facts, so a consumer never has to resolve
        a lot that may have been fully consumed."""
        register = empty_register()
        importer.run(write(SCHWAB_CSV), load_profile("schwab"),
                     register, RULES, FX)
        disposal = register["positions"]["AVGO"]["closed"][0]
        for field in ("acq_date", "cb_per_share", "acq_fx", "funding",
                      "holding_days", "long_term"):
            self.assertIn(field, disposal)


class TestQtySignRouting(unittest.TestCase):
    """research R-003 — the trap. IBKR has no Buy/Sell string."""

    def test_sign_decides_direction(self):
        register = empty_register()
        batch = importer.run(write(IBKR_CSV), load_profile("ibkr"),
                             register, RULES, FX)
        self.assertEqual(len(batch["created"]), 1,
                         "positive quantity must create a lot")
        self.assertEqual(len(batch["matched"]), 1,
                         "negative quantity must become a disposal — if this is "
                         "0, qty_sign is not being applied and every trade "
                         "routed to the same event")
        position = register["positions"]["AAPL"]
        self.assertAlmostEqual(position["lots"][0]["qty"], 6.0)

    def test_subtotal_rows_are_not_data(self):
        rows = profiles.read_rows(write(IBKR_CSV), load_profile("ibkr"))
        self.assertEqual(len(rows), 2, "SubTotal row leaked in as data")


class TestCorporateActionRefused(unittest.TestCase):
    def test_split_refuses_the_whole_import(self):
        csv_text = SCHWAB_CSV + '"09/01/2025","Split","AVGO","10 FOR 1","90","","",""\n'
        with self.assertRaises(importer.ImportRefused) as ctx:
            importer.run(write(csv_text), load_profile("schwab"),
                         empty_register(), RULES, FX, dry_run=True)
        self.assertIn("corporate action", str(ctx.exception).lower())


class TestProfileValidation(unittest.TestCase):
    def test_shipped_profiles_pass_their_own_checks(self):
        for name in ("schwab", "ibkr"):
            with self.subTest(profile=name):
                self.assertEqual(profiles.validate_profile(load_profile(name)), [])

    def test_no_broker_name_appears_in_the_code(self):
        """SC-005 / FR-020: broker knowledge lives in profiles/, never in code."""
        offenders = []
        for path in (PACK / "portfolio").glob("*.py"):
            text = path.read_text().lower()
            for name in ("schwab", "ibkr", "avgo"):
                # Docstrings may not name a broker either — a comment saying
                # "for Schwab, do X" is the first step back to a forked path.
                if name in text:
                    offenders.append(f"{path.name} mentions {name!r}")
        self.assertEqual(offenders, [], "; ".join(offenders))


if __name__ == "__main__":
    unittest.main()
