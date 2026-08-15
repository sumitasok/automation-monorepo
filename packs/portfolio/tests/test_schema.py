"""Schema validation, and the property that makes a hand-rolled validator safe.

research R-006: the validator supports a subset. That is only acceptable if an
unsupported keyword is REJECTED rather than ignored — a schema that silently
skips a constraint looks stricter than it is, which is worse than no schema.
"""
import json
import unittest
from pathlib import Path

from portfolio import store
from portfolio.schema import SchemaError, Validator, unsupported_keywords

PACK = Path(__file__).resolve().parent.parent
SCHEMAS = sorted((PACK / "schemas").glob("*.json"))


class TestSubsetDiscipline(unittest.TestCase):
    def test_every_shipped_schema_is_within_the_supported_subset(self):
        """If this fails, either add support or stop using the keyword. Do not
        widen SUPPORTED without implementing enforcement."""
        for path in SCHEMAS:
            with self.subTest(schema=path.name):
                schema = json.loads(path.read_text())
                self.assertEqual(unsupported_keywords(schema), set())

    def test_every_shipped_schema_loads(self):
        self.assertEqual(len(SCHEMAS), 4, "expected four published contracts")
        for path in SCHEMAS:
            Validator(json.loads(path.read_text()), label=path.name)

    def test_unknown_keyword_is_rejected_not_ignored(self):
        with self.assertRaises(SchemaError) as ctx:
            Validator({"type": "object", "patternProperties": {"^x": {}}})
        self.assertIn("patternProperties", str(ctx.exception))

    def test_property_names_are_not_mistaken_for_keywords(self):
        """A field literally called `contains` or `if` must not trip the check."""
        Validator({"type": "object",
                   "properties": {"contains": {"type": "string"},
                                  "if": {"type": "string"}}})


class TestValidation(unittest.TestCase):
    def setUp(self):
        self.validator = Validator(
            json.loads((PACK / "schemas" / "lot-register.schema.json").read_text()),
            label="lot-register")
        self.good = {
            "contract_version": "1.0.0",
            "generated_at": "2026-08-15T00:00:00+00:00",
            "reporting_currency": "INR",
            "funding": {"RSU": {"label": "RSU", "own_money": "none", "desc": "d"}},
            "positions": {"AVGO": {
                "broker": "schwab", "currency": "USD", "lots": [{
                    "id": "L1", "broker": "schwab", "acq_date": "2025-06-15",
                    "qty": 10.0, "cb_per_share": 250.0,
                    "price_paid_per_share": 0.0, "acq_fx": 85.0,
                    "funding": "RSU", "src": "x", "confirmed": True}],
                "closed": []}},
        }

    def test_valid_register_passes(self):
        self.assertEqual(self.validator.validate(self.good), [])

    def test_missing_required_field_is_named(self):
        bad = json.loads(json.dumps(self.good))
        del bad["positions"]["AVGO"]["lots"][0]["acq_fx"]
        errors = self.validator.validate(bad)
        self.assertTrue(any("acq_fx" in e for e in errors), errors)

    def test_misspelled_key_is_caught(self):
        bad = json.loads(json.dumps(self.good))
        lot = bad["positions"]["AVGO"]["lots"][0]
        lot["quantity"] = lot.pop("qty")
        errors = self.validator.validate(bad)
        self.assertTrue(any("quantity" in e for e in errors), errors)
        self.assertTrue(any("qty" in e for e in errors), errors)

    def test_wrong_type_is_caught(self):
        bad = json.loads(json.dumps(self.good))
        bad["positions"]["AVGO"]["lots"][0]["qty"] = "ten"
        errors = self.validator.validate(bad)
        self.assertTrue(any("expected number" in e for e in errors), errors)

    def test_boolean_is_not_a_number(self):
        bad = json.loads(json.dumps(self.good))
        bad["positions"]["AVGO"]["lots"][0]["qty"] = True
        self.assertTrue(self.validator.validate(bad))

    def test_out_of_range_is_caught(self):
        bad = json.loads(json.dumps(self.good))
        bad["positions"]["AVGO"]["lots"][0]["qty"] = 0
        errors = self.validator.validate(bad)
        self.assertTrue(any("greater than 0" in e for e in errors), errors)

    def test_every_problem_is_reported_not_just_the_first(self):
        """FR-054."""
        bad = json.loads(json.dumps(self.good))
        lot = bad["positions"]["AVGO"]["lots"][0]
        del lot["acq_fx"]
        del lot["src"]
        lot["qty"] = "ten"
        self.assertGreaterEqual(len(self.validator.validate(bad)), 3)

    def test_error_names_the_path(self):
        bad = json.loads(json.dumps(self.good))
        del bad["positions"]["AVGO"]["lots"][0]["acq_fx"]
        errors = self.validator.validate(bad)
        self.assertTrue(any("positions.AVGO.lots[0]" in e for e in errors), errors)


class TestOneOfDiscrimination(unittest.TestCase):
    """The broker profile's reader discriminator — one contract, two shapes."""

    def setUp(self):
        self.validator = Validator(
            json.loads((PACK / "schemas" / "broker-profile.schema.json").read_text()),
            label="broker-profile")

    def _profile(self, reader):
        return {"contract_version": "1.0.0", "name": "x", "reader": reader,
                "money": {"strip": [","]}, "dates": {"formats": ["%Y-%m-%d"]},
                "actions": [{"match": ".*", "event": "ignore"}]}

    def test_tabular_accepted(self):
        self.assertEqual(self.validator.validate(self._profile(
            {"kind": "tabular", "header_tokens": ["A"],
             "columns": {"date": ["Date"]}})), [])

    def test_sectioned_accepted(self):
        self.assertEqual(self.validator.validate(self._profile(
            {"kind": "sectioned",
             "sections": [{"name": "t", "section_name": "Trades",
                           "field_map": {"date": "Date"}}]})), [])

    def test_hybrid_rejected(self):
        """Matching both branches is as wrong as matching neither."""
        self.assertTrue(self.validator.validate(self._profile(
            {"kind": "tabular", "header_tokens": ["A"],
             "columns": {"date": ["Date"]},
             "sections": [{"name": "t", "section_name": "T", "field_map": {}}]})))


class TestShippedDataValidates(unittest.TestCase):
    def test_workspace_register_satisfies_the_contract(self):
        path = PACK.parents[1] / "data" / "portfolio" / "register.yaml"
        if not path.exists():
            self.skipTest("workspace data not present")
        errors = store.validate_against(
            store.load_yaml(path, "register.yaml"),
            "lot-register.schema.json", "register.yaml")
        self.assertEqual(errors, [])


if __name__ == "__main__":
    unittest.main()
