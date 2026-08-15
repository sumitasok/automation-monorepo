"""JSON Schema validation, minimal subset, no dependency.

Design note (specs/005-portfolio-tax-pack/research.md R-006): the pack authors
its own schemas, so it needs only the keywords those schemas use. That makes a
~200-line validator preferable to a dependency, and lets errors be phrased in
this pack's terms — file, path, expectation — which FR-053/FR-054 require.

THE IMPORTANT PROPERTY: an unknown keyword is an ERROR, not a no-op. A schema
that silently ignores a constraint looks stricter than it is, which is worse
than having no schema. `unsupported_keywords()` lets a test assert that every
keyword in the shipped schemas is one this module actually enforces.
"""
from __future__ import annotations

import re
from datetime import date, datetime

SUPPORTED = {
    # structural / annotation — recognised, not constraints
    "$schema", "$id", "$defs", "title", "description", "default",
    # constraints this module enforces
    "type", "required", "properties", "additionalProperties", "items",
    "enum", "const", "pattern", "minimum", "maximum", "exclusiveMinimum",
    "exclusiveMaximum", "minLength", "minItems", "minProperties",
    "oneOf", "$ref", "format",
}

_TYPES = {
    "object": dict, "array": list, "string": str, "boolean": bool,
    "number": (int, float), "integer": int, "null": type(None),
}


class SchemaError(Exception):
    """The schema itself is malformed or uses something we do not enforce."""


def unsupported_keywords(schema) -> set[str]:
    """Every keyword in `schema` that this module would not enforce.

    Walks only positions where a subschema can legally appear, so property
    *names* are never mistaken for keywords.
    """
    found: set[str] = set()

    def walk(node):
        if not isinstance(node, dict):
            return
        for key, val in node.items():
            if key not in SUPPORTED:
                found.add(key)
            if key in ("properties", "$defs"):
                if isinstance(val, dict):
                    for sub in val.values():
                        walk(sub)
            elif key in ("items", "additionalProperties"):
                walk(val)
            elif key == "oneOf":
                for sub in val or []:
                    walk(sub)

    walk(schema)
    return found


def _as_date(value):
    if isinstance(value, date):
        return value
    return datetime.strptime(str(value)[:10], "%Y-%m-%d").date()


class Validator:
    """Validates a parsed structure. Collects every error rather than raising
    on the first, because FR-054 requires all problems reported together."""

    def __init__(self, schema: dict, label: str = "<schema>"):
        bad = unsupported_keywords(schema)
        if bad:
            raise SchemaError(
                f"{label}: uses JSON Schema keywords this validator does not "
                f"enforce: {', '.join(sorted(bad))}. Either add support in "
                f"portfolio/schema.py or stop using them — silently ignoring "
                f"them would make the schema look stricter than it is."
            )
        self.schema = schema
        self.label = label

    def validate(self, data) -> list[str]:
        errors: list[str] = []
        self._check(data, self.schema, "$", errors)
        return errors

    # -- internals ---------------------------------------------------------
    def _resolve(self, node):
        ref = node.get("$ref")
        if not ref:
            return node
        if not ref.startswith("#/$defs/"):
            raise SchemaError(f"{self.label}: only #/$defs/* refs supported, got {ref!r}")
        name = ref[len("#/$defs/"):]
        target = (self.schema.get("$defs") or {}).get(name)
        if target is None:
            raise SchemaError(f"{self.label}: $ref to unknown def {name!r}")
        merged = dict(target)
        for k, v in node.items():          # sibling keys override the target
            if k != "$ref":
                merged[k] = v
        return merged

    def _check(self, data, node, path, errors):
        node = self._resolve(node)

        if "oneOf" in node:
            branches = node["oneOf"]
            matches = [i for i, sub in enumerate(branches)
                       if not self._probe(data, sub, path)]
            if len(matches) != 1:
                errors.append(
                    f"{path}: expected to match exactly one of "
                    f"{len(branches)} alternatives, matched {len(matches)}")
                return

        if "type" in node:
            expected = node["type"]
            names = expected if isinstance(expected, list) else [expected]
            pytypes = tuple(_TYPES[n] for n in names)
            # bool is a subclass of int; a boolean is never a number here
            if isinstance(data, bool) and "boolean" not in names:
                errors.append(f"{path}: expected {'/'.join(names)}, got boolean")
                return
            if not isinstance(data, pytypes):
                errors.append(
                    f"{path}: expected {'/'.join(names)}, got "
                    f"{type(data).__name__}")
                return

        if "const" in node and data != node["const"]:
            errors.append(f"{path}: expected the constant {node['const']!r}, got {data!r}")
        if "enum" in node and data not in node["enum"]:
            errors.append(f"{path}: expected one of {node['enum']}, got {data!r}")

        if isinstance(data, str):
            if "pattern" in node and not re.search(node["pattern"], data):
                errors.append(f"{path}: {data!r} does not match {node['pattern']}")
            if "minLength" in node and len(data) < node["minLength"]:
                errors.append(f"{path}: shorter than {node['minLength']} characters")
            fmt = node.get("format")
            if fmt in ("date", "date-time"):
                try:
                    (_as_date(data) if fmt == "date"
                     else datetime.fromisoformat(data.replace("Z", "+00:00")))
                except (ValueError, TypeError):
                    errors.append(f"{path}: {data!r} is not a valid {fmt}")
        elif isinstance(data, date) and node.get("format") == "date":
            pass                                   # YAML parsed it for us

        if isinstance(data, (int, float)) and not isinstance(data, bool):
            for key, ok, word in (
                ("minimum", lambda a, b: a >= b, "at least"),
                ("maximum", lambda a, b: a <= b, "at most"),
                ("exclusiveMinimum", lambda a, b: a > b, "greater than"),
                ("exclusiveMaximum", lambda a, b: a < b, "less than"),
            ):
                if key in node and not ok(data, node[key]):
                    errors.append(f"{path}: must be {word} {node[key]}, got {data}")

        if isinstance(data, list):
            if "minItems" in node and len(data) < node["minItems"]:
                errors.append(f"{path}: needs at least {node['minItems']} items")
            if "items" in node:
                for i, item in enumerate(data):
                    self._check(item, node["items"], f"{path}[{i}]", errors)

        if isinstance(data, dict):
            if "minProperties" in node and len(data) < node["minProperties"]:
                errors.append(f"{path}: needs at least {node['minProperties']} entries")
            for name in node.get("required", []):
                if name not in data:
                    errors.append(f"{path}: missing required field {name!r}")
            props = node.get("properties", {})
            for name, value in data.items():
                if name in props:
                    self._check(value, props[name], f"{path}.{name}", errors)
                else:
                    extra = node.get("additionalProperties")
                    if extra is False:
                        errors.append(
                            f"{path}: unexpected field {name!r} "
                            f"(known: {', '.join(sorted(props)) or 'none'})")
                    elif isinstance(extra, dict):
                        self._check(value, extra, f"{path}.{name}", errors)

    def _probe(self, data, node, path) -> list[str]:
        errors: list[str] = []
        self._check(data, node, path, errors)
        return errors
