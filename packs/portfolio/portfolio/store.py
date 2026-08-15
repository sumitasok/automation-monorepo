"""Loading and saving the pack's data files.

Two things here are load-bearing and easy to get wrong:

1. **Atomic save must resolve the symlink first.** Every path this pack writes
   is a symlink from its workdir into data/portfolio/ (ADR 0019). The obvious
   `os.replace(tmp, link_path)` REPLACES THE SYMLINK with a real file, putting
   data inside packs/ — the exact drift ADR 0018 exists to prevent, and a
   guaranteed `auto doctor` failure. So: resolve, write the temp beside the
   *target*, replace the target. See research.md R-004.

2. **A dangling symlink means "not generated yet", not "broken".** `auto` links
   before the target exists, so a freshly mounted pack has dangling links. That
   is spec edge case "pack mounted, job never run" and must read as absent.
"""
from __future__ import annotations

import json
import os
from datetime import date, datetime
from pathlib import Path

import yaml

from .schema import Validator

HERE = Path(__file__).resolve().parent.parent      # packs/portfolio/
SCHEMA_DIR = HERE / "schemas"


class DataError(Exception):
    """A data file is missing, unparseable, or fails its schema."""


def resolve_target(path: Path) -> Path:
    """The real file a declared path points at.

    Returns the symlink's target when `path` is a link (even a dangling one),
    otherwise `path` itself. Never raises for a dangling link — that is a
    normal, expected state before the first run.
    """
    p = Path(path)
    if p.is_symlink():
        target = Path(os.readlink(p))
        return target if target.is_absolute() else (p.parent / target).resolve()
    return p


def exists(path: Path) -> bool:
    """True only if the declared path resolves to a file that is really there."""
    return resolve_target(path).exists()


def _yaml_safe(value):
    """Make PyYAML-parsed values JSON-safe (dates become ISO strings)."""
    if isinstance(value, dict):
        return {k: _yaml_safe(v) for k, v in value.items()}
    if isinstance(value, list):
        return [_yaml_safe(v) for v in value]
    if isinstance(value, datetime):
        return value.isoformat()
    if isinstance(value, date):
        return value.isoformat()
    return value


def load_yaml(path: Path, what: str) -> dict:
    target = resolve_target(path)
    if not target.exists():
        raise DataError(
            f"{what} not found.\n"
            f"  expected at: {target}\n"
            f"  reached via: {path}\n"
            f"  If this is a fresh install, run `migrate` or copy a file from "
            f"packs/portfolio/samples/ into data/portfolio/."
        )
    try:
        with open(target) as fh:
            data = yaml.safe_load(fh)
    except yaml.YAMLError as exc:
        raise DataError(f"{what} at {target} is not valid YAML:\n  {exc}") from exc
    if data is None:
        raise DataError(f"{what} at {target} is empty.")
    return _yaml_safe(data)


def load_schema(name: str) -> dict:
    with open(SCHEMA_DIR / name) as fh:
        return json.load(fh)


def validate_against(data: dict, schema_name: str, what: str) -> list[str]:
    """Every problem, prefixed with the file it came from (FR-053/FR-054)."""
    validator = Validator(load_schema(schema_name), label=schema_name)
    return [f"{what}: {err}" for err in validator.validate(data)]


def load_validated(path: Path, schema_name: str, what: str) -> dict:
    data = load_yaml(path, what)
    errors = validate_against(data, schema_name, what)
    if errors:
        raise DataError(
            f"{what} failed validation ({len(errors)} problem"
            f"{'s' if len(errors) != 1 else ''}):\n  "
            + "\n  ".join(errors)
        )
    return data


def _atomic_write(path: Path, render) -> Path:
    """Write through a declared path without ever replacing its symlink.

    `render` receives an open file handle. Returns the real path written.
    """
    target = resolve_target(path)
    target.parent.mkdir(parents=True, exist_ok=True)
    tmp = target.with_name(target.name + ".tmp")     # same dir => same filesystem
    try:
        with open(tmp, "w") as fh:
            render(fh)
            fh.flush()
            os.fsync(fh.fileno())
        os.replace(tmp, target)                      # atomic for any reader
    finally:
        if tmp.exists():
            tmp.unlink()
    return target


def save_yaml(path: Path, data: dict, header: str = "") -> Path:
    def render(fh):
        if header:
            fh.write(header if header.endswith("\n") else header + "\n")
        yaml.safe_dump(data, fh, sort_keys=False, default_flow_style=False,
                       allow_unicode=True, width=100)
    return _atomic_write(path, render)


def save_json(path: Path, data: dict) -> Path:
    return _atomic_write(path, lambda fh: json.dump(data, fh, separators=(",", ":"),
                                                    default=str))


def save_text(path: Path, text: str) -> Path:
    return _atomic_write(path, lambda fh: fh.write(text))
