#!/usr/bin/env python3
"""
Generic document-extraction engine for sa.finances.

Zero institution-specific logic lives here. All knowledge about senders,
subjects, body patterns, transforms and routing lives in declarative YAML
files under formats/ and routing.yaml. The engine just iterates.

Usage:
    # one document as JSON on the command line
    python3 engine.py --json '{"source":"gmail","id":"...","sender":"...","subject":"...","date":"...","body":"..."}'

    # many documents, one JSON object per line
    python3 engine.py --file documents.jsonl

    # run the regression tests in tests/samples/
    python3 engine.py --test

Input document envelope (source-agnostic):
    source   gmail | drive | sms | file ...
    id       stable unique id within the source (gmail message id, drive file id)
    sender   for emails; optional otherwise
    subject  for emails; filename for files; optional
    date     ISO timestamp the document was received/created
    body     raw text or HTML; engine normalizes HTML to text before matching

Output per document (JSON):
    matched      true/false
    format       name of the matching format (when matched)
    action       "extract" | "skip"
    reason       skip reason (when action == skip)
    record       normalized transaction dict (when action == extract)
    unmatched_excerpt  first 600 chars of normalized text (when not matched)
"""
import argparse
import datetime as dt
import html
import html.parser
import json
import os
import pathlib
import re
import sys

import yaml

BASE = pathlib.Path(__file__).resolve().parent
# Read format files from external config if CONFIG_PATH is set
CONFIG_PATH = pathlib.Path(os.environ.get("CONFIG_PATH", pathlib.Path.home() / "automation-monorepo-config"))
FORMATS_DIR = CONFIG_PATH / "config" / "expense-domain" / "wallet" / "email-formats"
ROUTING_FILE = CONFIG_PATH / "config" / "expense-domain" / "wallet" / "routing.yaml"

# Fallback to local formats if external config doesn't exist
if not FORMATS_DIR.exists():
    FORMATS_DIR = BASE / "formats"
    ROUTING_FILE = BASE / "routing.yaml"


# --------------------------------------------------------------------------
# HTML -> text normalization
# --------------------------------------------------------------------------
class _TextExtractor(html.parser.HTMLParser):
    _SKIP = {"style", "script", "head", "title"}

    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.parts = []
        self._skip_depth = 0

    def handle_starttag(self, tag, attrs):
        if tag in self._SKIP:
            self._skip_depth += 1
        elif tag in {"br", "p", "tr", "li", "div", "table"}:
            self.parts.append("\n")

    def handle_endtag(self, tag):
        if tag in self._SKIP and self._skip_depth:
            self._skip_depth -= 1

    def handle_data(self, data):
        if not self._skip_depth:
            self.parts.append(data)


def normalize_body(body: str) -> str:
    """HTML or plain text -> single-spaced plain text."""
    if "<" in body and ">" in body:
        p = _TextExtractor()
        try:
            p.feed(body)
            text = "".join(p.parts)
        except Exception:
            text = re.sub(r"<[^>]+>", " ", body)
    else:
        text = body
    text = html.unescape(text)
    return re.sub(r"\s+", " ", text).strip()


# --------------------------------------------------------------------------
# Format registry
# --------------------------------------------------------------------------
def load_formats(formats_dir=FORMATS_DIR):
    fmts = []
    for path in sorted(formats_dir.glob("*.yaml")):
        with open(path) as f:
            for doc in yaml.safe_load_all(f):
                if not doc:
                    continue
                doc.setdefault("priority", 100)
                doc["_file"] = path.name
                _validate(doc, path.name)
                fmts.append(doc)
    # lower priority number = tried first; stable by name
    fmts.sort(key=lambda d: (d["priority"], d.get("name", "")))
    return fmts


def _validate(fmt, fname):
    for key in ("name", "match", "action"):
        if key not in fmt:
            raise ValueError(f"{fname}: format missing required key '{key}'")
    if fmt["action"] == "extract" and "fields" not in fmt:
        raise ValueError(f"{fname}: extract format '{fmt['name']}' needs 'fields'")
    # compile early so a bad regex fails at load time, not match time
    for field, pattern in fmt["match"].items():
        re.compile(pattern, re.I | re.S)
    for key, pattern in fmt.get("fields", {}).items():
        re.compile(pattern, re.I | re.S)


def load_routing(routing_file=ROUTING_FILE):
    if not routing_file.exists():
        return []
    with open(routing_file) as f:
        data = yaml.safe_load(f) or {}
    return data.get("routes", [])


# --------------------------------------------------------------------------
# Matching & extraction
# --------------------------------------------------------------------------
def _doc_field(doc, name, norm_text):
    if name == "body":
        return norm_text
    return str(doc.get(name, ""))


def matches(fmt, doc, norm_text):
    if "source" in fmt and doc.get("source") and fmt["source"] != doc["source"]:
        return False
    return all(
        re.search(pat, _doc_field(doc, field, norm_text), re.I | re.S)
        for field, pat in fmt["match"].items()
    )


TRANSFORMS = {}


def transform(name):
    def deco(fn):
        TRANSFORMS[name] = fn
        return fn
    return deco


@transform("decimal")
def _t_decimal(value, spec):
    return float(value.replace(",", ""))


@transform("date")
def _t_date(value, spec):
    for f in spec.get("formats", ["%Y-%m-%d"]):
        try:
            return dt.datetime.strptime(value.strip(), f).date().isoformat()
        except ValueError:
            continue
    raise ValueError(f"date '{value}' matched none of {spec.get('formats')}")


@transform("strip")
def _t_strip(value, spec):
    return value.strip()


@transform("upper")
def _t_upper(value, spec):
    return value.strip().upper()


@transform("title")
def _t_title(value, spec):
    return value.strip().title()


def extract(fmt, doc, norm_text):
    record = {}
    for field, pattern in fmt.get("fields", {}).items():
        m = re.search(pattern, _doc_field(doc, field, norm_text), re.I | re.S)
        if not m:
            return None, f"field regex '{field}' did not match"
        record.update({k: v for k, v in m.groupdict().items() if v is not None})

    for key, spec in fmt.get("transforms", {}).items():
        if key in record:
            if isinstance(spec, str):
                spec = {"type": spec}
            record[key] = TRANSFORMS[spec["type"]](record[key], spec)

    record.update(fmt.get("set", {}))
    record["source"] = doc.get("source")
    record["source_id"] = doc.get("id")
    record["format"] = fmt["name"]
    return record, None


def route(record, routes):
    """First route whose conditions all match the record wins."""
    for r in routes:
        cond = r.get("when", {})
        if all(re.search(pat, str(record.get(k, "")), re.I) for k, pat in cond.items()):
            record.update(r.get("set", {}))
            return record
    record.setdefault("routing", "UNROUTED")
    return record


# --------------------------------------------------------------------------
# Pipeline
# --------------------------------------------------------------------------
def process(doc, fmts=None, routes=None):
    fmts = fmts if fmts is not None else load_formats()
    routes = routes if routes is not None else load_routing()
    norm_text = normalize_body(doc.get("body", ""))

    for fmt in fmts:
        if not matches(fmt, doc, norm_text):
            continue
        if fmt["action"] == "skip":
            return {
                "matched": True,
                "format": fmt["name"],
                "action": "skip",
                "reason": fmt.get("reason", ""),
                "source_id": doc.get("id"),
            }
        record, err = extract(fmt, doc, norm_text)
        if record is None:
            # match block hit but extraction failed -> surface loudly,
            # the format file probably needs updating for a layout change
            return {
                "matched": False,
                "action": "error",
                "format": fmt["name"],
                "error": err,
                "source_id": doc.get("id"),
                "unmatched_excerpt": norm_text[:600],
            }
        return {
            "matched": True,
            "format": fmt["name"],
            "action": "extract",
            "record": route(record, routes),
        }

    return {
        "matched": False,
        "action": "unmatched",
        "source_id": doc.get("id"),
        "sender": doc.get("sender"),
        "subject": doc.get("subject"),
        "unmatched_excerpt": norm_text[:600],
    }


# --------------------------------------------------------------------------
# Tests
# --------------------------------------------------------------------------
def run_tests():
    samples = sorted((BASE / "tests" / "samples").glob("*.json"))
    if not samples:
        print("no samples found")
        return 1
    fmts, routes = load_formats(), load_routing()
    failures = 0
    for path in samples:
        case = json.loads(path.read_text())
        got = process(case["document"], fmts, routes)
        exp = case["expect"]
        ok = all(_dig(got, k) == v for k, v in exp.items())
        print(f"{'PASS' if ok else 'FAIL'}  {path.name}")
        if not ok:
            failures += 1
            for k, v in exp.items():
                g = _dig(got, k)
                if g != v:
                    print(f"      {k}: expected {v!r}, got {g!r}")
    print(f"\n{len(samples) - failures}/{len(samples)} passed")
    return 1 if failures else 0


def _dig(obj, dotted):
    for part in dotted.split("."):
        if not isinstance(obj, dict):
            return None
        obj = obj.get(part)
    return obj


# --------------------------------------------------------------------------
def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--json", help="single document as a JSON string")
    ap.add_argument("--file", help="JSONL file, one document per line")
    ap.add_argument("--test", action="store_true", help="run regression tests")
    args = ap.parse_args()

    if args.test:
        sys.exit(run_tests())
    if args.json:
        print(json.dumps(process(json.loads(args.json)), indent=2))
        return
    if args.file:
        fmts, routes = load_formats(), load_routing()
        for line in open(args.file):
            line = line.strip()
            if line:
                print(json.dumps(process(json.loads(line), fmts, routes)))
        return
    ap.print_help()


if __name__ == "__main__":
    main()
