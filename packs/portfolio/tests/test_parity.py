"""SC-001 — the gate. The port must not have moved a single figure.

Expected values are the vault program's own output, captured from:

    cd ~/Claude/Projects/sa.finances/_db/tax
    python3 planner.py --ticker AVGO --today 2026-08-11 --fx-later 85.5

at spot $427.76 / FX 85.50.

Both inputs and expected outputs are frozen here on purpose. The test must keep
meaning something after the vault program is retired (FR-058/FR-060), so it
cannot shell out to it; and it must not silently start passing because the
register changed underneath it, so it does not read the register either.

⚠ The INPUTS are the register's exact stored values, NOT the planner's printed
ones. The vault's table rounds for display — cb 388.355 prints as 388.36, and
five lots' acq_fx differ from what the table shows. Transcribing from the
printed table produces a test that fails by ~0.01% and looks like a real
arithmetic bug. It isn't; it is the wrong input.

A genuine failure here is not a rounding question. It means a tax convention
was lost in the port — stop and find it before touching anything else.
"""
import unittest
from datetime import date

from portfolio.lots import matures_on
from portfolio.planner import analyse_lot
from portfolio.rules import Rates

SPOT = 427.76
FX = 85.50
WHEN = date(2026, 8, 11)

RULES = {
    "capital_gains": {
        "foreign_equity": {
            "ltcg_after_months": 24, "stcg_rate": 0.39,
            "ltcg_rate": 0.125, "ltcg_surcharge_cap": 0.15,
        },
        "lot_matching": {"method": "fifo",
                         "same_date_tiebreak": ["RSU", "ESPP", "MARKET"],
                         "lapse_sale_window_days": 5},
    },
    "cess": 0.04,
}

# (id, qty, cb, acq_fx, acq_date) -> (matures, breakeven, cushion, tax_now, saving)
# Inputs: data/portfolio/register.yaml. Outputs: the vault planner's table.
#
# ⚠ The vault table's LAST money column is SAVING (tax_now - tax_lt), not
# tax-at-maturity. Reading it as tax_lt makes every lot fail by ~60%: the
# ratio tax_now/tax_lt must be 0.39/0.1495 = 2.609, but the printed pair
# ratios to 1.62 on every row, which is the giveaway.
GOLDEN = [
    (("PRE-JUN24-RSU",   45.0,    171.09,   83.40, "2024-06-15"),
     ("2026-06-15", None,   None,    150054,      0)),
    (("PRE-SEP24-RSU",   61.0,    167.21,   83.89, "2024-09-15"),
     ("2026-09-15", 353.19, -0.174,  536375, 330765)),
    (("PRE-SEP24-ESPP",  33.0,    105.02,   83.89, "2024-09-15"),
     ("2026-09-15", 335.94, -0.215,  357314, 220344)),
    (("PRE-DEC24-RSU",   91.0,    240.00,   84.60, "2024-12-15"),
     ("2026-12-15", 373.95, -0.126,  577404, 356066)),
    (("PRE-MAR25-ESPP",   1.0,    139.39,   86.69, "2025-03-15"),
     ("2027-03-15", 346.76, -0.189,    9551,   5890)),
    (("JUN25-RSU",      178.0,    249.68,   85.75, "2025-06-15"),
     ("2027-06-15", 377.61, -0.117, 1052645, 649131)),
    (("SEP25-RSU",       78.0,    361.98,   87.85, "2025-09-15"),
     ("2027-09-15", 411.97, -0.037,  145211,  89547)),
    (("SEP25-ESPP",      19.0,    361.39,   87.95, "2025-09-15"),
     ("2027-09-15", 411.92, -0.037,   35488,  21884)),
    (("DEC25-RSU",      140.0,    350.85,   90.10, "2025-12-15"),
     ("2027-12-15", 411.35, -0.038,  270919, 167067)),
    (("DEC25-BUY",      157.0,    323.25,   90.10, "2025-12-17"),
     ("2027-12-17", 403.13, -0.058,  456081, 281250)),
    (("MAR26-ESPP",      11.0,    329.92,   91.93, "2026-03-14"),
     ("2028-03-14", 407.11, -0.048,   26786,  16518)),
    (("MAR26-RSU",       79.0,    329.92,   91.93, "2026-03-15"),
     ("2028-03-15", 407.11, -0.048,  192376, 118632)),
    (("MAR26-BUY",       1.4775,  302.29,   91.93, "2026-03-31"),
     ("2028-03-31", 398.71, -0.068,    5062,   3121)),
    (("JUN26-BUY",       4.019,   398.10,   85.50, "2026-06-08"),
     ("2028-06-08", 419.37, -0.020,    3975,   2451)),
    (("JUN26-RSU",      66.0,     388.355,  85.50, "2026-06-17"),
     ("2028-06-17", 416.62, -0.026,   86721,  53478)),
    (("JUN26-RSU-2",    54.0,     388.355,  85.50, "2026-06-17"),
     ("2028-06-17", 416.62, -0.026,   70954,  43755)),
    (("JUN26-RSU-3",    19.0,     388.355,  85.50, "2026-06-17"),
     ("2028-06-17", 416.62, -0.026,   24965,  15395)),
    (("JUN26-RSU-4",    18.0,     388.355,  85.50, "2026-06-17"),
     ("2028-06-17", 416.62, -0.026,   23651,  14585)),
    (("JUN26-BUY-2",     1.3733,  376.1721, 85.50, "2026-06-30"),
     ("2028-06-30", 413.17, -0.034,    2362,   1457)),
]


class TestParity(unittest.TestCase):
    def setUp(self):
        self.rates = Rates(RULES)

    def test_effective_rates(self):
        """LTCG is not a plain rate: 12.5% x 1.15 surcharge cap x 1.04 cess.
        Flattening it to a literal would drop the cap the moment either moved."""
        self.assertAlmostEqual(self.rates.ltcg, 0.1495, places=6)
        self.assertAlmostEqual(self.rates.stcg, 0.39, places=6)
        self.assertAlmostEqual(self.rates.spread(), 0.2405, places=6)

    def test_every_lot_matches_the_vault(self):
        for (lot_id, qty, cb, acq_fx, acq), \
            (matures, breakeven, cushion, tax_now, saving) in GOLDEN:
            with self.subTest(lot=lot_id):
                lot = {"id": lot_id, "qty": qty, "cb_per_share": cb,
                       "acq_fx": acq_fx, "acq_date": acq, "funding": "RSU",
                       "broker": "schwab"}

                self.assertEqual(matures_on(lot, 24).isoformat(), matures,
                                 f"{lot_id}: maturity date moved")

                got = analyse_lot(lot, SPOT, FX, self.rates, WHEN)

                if breakeven is None:
                    self.assertTrue(got["mature"], f"{lot_id}: should be mature")
                    self.assertIsNone(got["breakeven"],
                                      f"{lot_id}: a matured lot has no break-even")
                else:
                    self.assertFalse(got["mature"], f"{lot_id}: should not be mature")
                    self.assertAlmostEqual(got["breakeven"], breakeven, places=2,
                                           msg=f"{lot_id}: break-even moved")
                    self.assertAlmostEqual(got["cushion"], cushion, places=3,
                                           msg=f"{lot_id}: cushion moved")

                # The vault prints whole rupees, so allow half-rupee rounding.
                self.assertAlmostEqual(got["tax_now"], tax_now, delta=1.0,
                                       msg=f"{lot_id}: tax-now moved")
                self.assertAlmostEqual(got["saving"], saving, delta=1.0,
                                       msg=f"{lot_id}: saving moved")
                self.assertAlmostEqual(got["tax_lt"], tax_now - saving, delta=1.0,
                                       msg=f"{lot_id}: tax-at-maturity moved")

    def test_portfolio_headline(self):
        """The vault's headline: liquidate today vs at full LTCG maturity."""
        total_now = sum(exp[3] for _, exp in GOLDEN)
        total_saving = sum(exp[4] for _, exp in GOLDEN)
        # Per-lot figures are printed rounded, so the sums carry a few rupees
        # of accumulated rounding against the vault's own headline.
        self.assertAlmostEqual(total_now, 4027895, delta=20)
        self.assertAlmostEqual(total_saving, 2391335, delta=20)
        self.assertAlmostEqual(total_now - total_saving, 1636560, delta=20)

    def test_both_legs_converted(self):
        """Both legs go through FX, which is what captures currency movement.
        Foreign-currency P&L alone is NOT the taxable figure."""
        lot = {"id": "T", "qty": 10, "cb_per_share": 100.0, "acq_fx": 80.0,
               "acq_date": "2020-01-01", "funding": "MARKET", "broker": "x"}
        got = analyse_lot(lot, 100.0, 90.0, self.rates, WHEN)
        # Flat in USD, but the rupee moved — there is a real taxable gain.
        self.assertAlmostEqual(got["gain"], 10 * (100 * 90 - 100 * 80), places=4)
        self.assertGreater(got["gain"], 0)


if __name__ == "__main__":
    unittest.main()
