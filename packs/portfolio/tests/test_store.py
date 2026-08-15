"""The pack/data boundary, and the trap that would quietly break it.

research.md R-004: every path this pack writes is a symlink into
data/portfolio/. A naive temp-and-rename replaces the SYMLINK with a real file,
putting data inside packs/ — ADR 0018's credentials-drift bug, as data. The
sandbox would deny the next write and `auto doctor` would fail, but only after
the register had already moved.

These tests fail loudly if anyone "simplifies" store._atomic_write.
"""
import json
import os
import tempfile
import unittest
from pathlib import Path

from portfolio import store


class TestAtomicWriteThroughSymlink(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        root = Path(self.tmp.name)
        self.data = root / "data" / "portfolio"      # stands in for data/<pack>/
        self.workdir = root / "packs" / "portfolio" / "data"
        self.data.mkdir(parents=True)
        self.workdir.mkdir(parents=True)
        self.link = self.workdir / "register.yaml"
        self.target = self.data / "register.yaml"
        self.link.symlink_to(self.target)            # dangling, as auto leaves it

    def tearDown(self):
        self.tmp.cleanup()

    def test_dangling_link_reads_as_absent_not_error(self):
        """A freshly mounted pack has dangling links — that is 'not generated
        yet', not a broken install."""
        self.assertTrue(self.link.is_symlink())
        self.assertFalse(store.exists(self.link))
        self.assertEqual(store.resolve_target(self.link), self.target)

    def test_write_does_not_replace_the_symlink(self):
        store.save_yaml(self.link, {"contract_version": "1.0.0"})

        self.assertTrue(self.link.is_symlink(),
                        "THE TRAP: the declared path is no longer a symlink — "
                        "a real data file now sits inside packs/, which the "
                        "sandbox forbids and auto doctor fails on")
        self.assertTrue(self.target.exists(), "the real file must be in data/")
        self.assertEqual(store.resolve_target(self.link), self.target)

    def test_repeated_writes_keep_the_symlink(self):
        for i in range(3):
            store.save_yaml(self.link, {"contract_version": f"1.0.{i}"})
            self.assertTrue(self.link.is_symlink(), f"lost the symlink on pass {i}")

    def test_no_temp_file_left_behind(self):
        store.save_json(self.link, {"a": 1})
        leftovers = list(self.data.glob("*.tmp")) + list(self.workdir.glob("*.tmp"))
        self.assertEqual(leftovers, [], f"temp files left: {leftovers}")

    def test_temp_file_is_written_inside_data(self):
        """The temp must live beside the target, not beside the link — both so
        the rename is atomic (same filesystem) and so the write stays inside an
        allowed sandbox root."""
        seen = []
        real_replace = os.replace

        def spy(src, dst):
            seen.append(Path(src).parent)
            return real_replace(src, dst)

        os.replace = spy
        try:
            store.save_json(self.link, {"a": 1})
        finally:
            os.replace = real_replace
        self.assertEqual(seen, [self.data],
                         "temp file was not created inside data/<pack>/")

    def test_reader_never_sees_a_partial_file(self):
        """FR-011: a consumer sees the complete previous state or the complete
        new one. os.replace is atomic; this guards against it being swapped for
        a truncate-and-write."""
        store.save_json(self.link, {"contract_version": "1.0.0", "positions": {}})
        first = json.loads(self.target.read_text())
        store.save_json(self.link, {"contract_version": "1.0.1", "positions": {}})
        second = json.loads(self.target.read_text())
        self.assertEqual(first["contract_version"], "1.0.0")
        self.assertEqual(second["contract_version"], "1.0.1")


class TestPlainPathsStillWork(unittest.TestCase):
    """Not every path is a symlink — during development the pack is run
    directly, so the same code must handle a real file."""

    def test_write_to_a_real_path(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "plain.yaml"
            store.save_yaml(path, {"contract_version": "1.0.0"})
            self.assertTrue(path.exists())
            self.assertFalse(path.is_symlink())


if __name__ == "__main__":
    unittest.main()
