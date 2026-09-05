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
import hashlib
import html
import html.parser
import json
import os
import pathlib
import re
import subprocess
import sys

import requests
import yaml

BASE = pathlib.Path(__file__).resolve().parent
# Read format files from external config if CONFIG_PATH is set
CONFIG_PATH = pathlib.Path(os.environ.get("CONFIG_PATH", pathlib.Path.home() / "automation-monorepo-config"))
# Email format patterns live under the gmail source of expense-domain
# (config/<domain>/<source>/ — the domain/source addressing convention;
# config/expense-domain/wallet/email-formats symlinks here for reuse).
FORMATS_DIR = CONFIG_PATH / "config" / "expense-domain" / "gmail" / "email-formats"
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
def process(doc, fmts=None, routes=None, ai_assist=False):
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
            "source_id": doc.get("id"),
            "record": route(record, routes),
        }

    result = {
        "matched": False,
        "action": "unmatched",
        "source_id": doc.get("id"),
        "sender": doc.get("sender"),
        "subject": doc.get("subject"),
        "unmatched_excerpt": norm_text[:600],
    }

    if ai_assist:
        suggestion = suggest_pattern_via_ai(doc, norm_text)
        if suggestion:
            result["ai_suggestion"] = suggestion
            if suggestion.get("success"):
                new_file = create_format_file_from_suggestion(
                    suggestion, doc.get("sender", ""), doc.get("subject", "")
                )
                if new_file:
                    result["ai_created_format_file"] = new_file

    return result


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
# AI-assisted pattern learning
# --------------------------------------------------------------------------
def get_ai_provider():
    """Return (provider, model, api_key, api_base) or (None, None, None, None) if not configured."""
    provider = os.environ.get("AI_PROVIDER", "").lower()
    if not provider:
        return None, None, None, None

    if provider == "deepseek":
        api_key = os.environ.get("DEEPSEEK_API_KEY")
        model = os.environ.get("DEEPSEEK_MODEL", "deepseek-chat")
        api_base = os.environ.get("DEEPSEEK_API_BASE", "https://api.deepseek.com")
        return provider, model, api_key, api_base
    elif provider in ("claude", "anthropic"):
        api_key = os.environ.get("ANTHROPIC_API_KEY")
        model = os.environ.get("ANTHROPIC_MODEL", "claude-3-5-sonnet-20241022")
        return provider, model, api_key, None

    return None, None, None, None


def suggest_pattern_via_ai(doc: dict, norm_text: str) -> dict | None:
    """Call AI provider to suggest a regex pattern for an unmatched email.

    Returns: {
        "success": bool,
        "suggested_pattern": str (regex),
        "suggested_fields": dict (field_name -> capture_group_name),
        "format_name": str (slug),
        "reasoning": str,
        "error": str (if failed)
    }
    """
    provider, model, api_key, api_base = get_ai_provider()
    if not provider or not api_key:
        return None

    sender = doc.get("sender", "")
    subject = doc.get("subject", "")

    prompt = f"""Analyze this unmatched email and suggest a SINGLE regex pattern with named capture groups.

Email metadata:
- Sender: {sender}
- Subject: {subject}

Email body (first 1000 chars):
{norm_text[:1000]}

Your task:
1. Identify what kind of transaction this is
2. Create ONE complete regex pattern with NAMED CAPTURE GROUPS that extracts all transaction fields
3. Use named groups like (?P<amount>...), (?P<date>...), (?P<merchant>...), (?P<card_last4>...), (?P<reference>...)

Return ONLY a JSON object:
{{
  "format_name": "slug-name-from-sender-subject",
  "body_pattern": "complete regex with (?P<field>...) named groups",
  "reasoning": "brief explanation"
}}

Example body_pattern:
"Your payment of [₹$]*\\s*(?P<amount>[\\d,]+(?:\\.\\d{{2}})?)[\\s\\w]+ on (?P<date>\\d{{2}}-[A-Z]{{3}}-\\d{{2}})"

Important:
- Use (?P<name>...) for named capture groups only, no other groups
- Include optional patterns like [₹$]* for currency symbols
- Match the EXACT sequence as it appears in the email
- Use (?:...) for non-capturing groups
- Make patterns permissive to handle whitespace variations
- Include only fields that are present"""

    try:
        if provider == "deepseek":
            return _call_deepseek_api(model, api_key, api_base, prompt)
        elif provider == "claude":
            return _call_claude_api(model, api_key, prompt)
    except Exception as e:
        return {
            "success": False,
            "error": str(e),
            "suggested_pattern": None,
            "format_name": None
        }

    return None


def _call_deepseek_api(model: str, api_key: str, api_base: str, prompt: str) -> dict:
    """Call DeepSeek API for pattern suggestion."""
    headers = {
        "Authorization": f"Bearer {api_key}",
        "Content-Type": "application/json"
    }

    payload = {
        "model": model,
        "messages": [{"role": "user", "content": prompt}],
        "temperature": 0.3,
    }

    resp = requests.post(f"{api_base}/v1/chat/completions", json=payload, headers=headers, timeout=30)
    resp.raise_for_status()

    data = resp.json()
    content = data.get("choices", [{}])[0].get("message", {}).get("content", "")

    try:
        # Extract JSON from response (might have surrounding text)
        json_match = re.search(r'\{.*\}', content, re.DOTALL)
        if json_match:
            suggestion = json.loads(json_match.group(0))
            suggestion["success"] = True
            return suggestion
    except (json.JSONDecodeError, AttributeError):
        pass

    return {
        "success": False,
        "error": f"Could not parse AI response: {content[:200]}",
        "suggested_pattern": None,
        "format_name": None
    }


def _call_claude_api(model: str, api_key: str, prompt: str) -> dict:
    """Call Claude API for pattern suggestion."""
    import requests

    headers = {
        "x-api-key": api_key,
        "anthropic-version": "2023-06-01",
        "content-type": "application/json"
    }

    payload = {
        "model": model,
        "max_tokens": 1024,
        "messages": [{"role": "user", "content": prompt}]
    }

    resp = requests.post("https://api.anthropic.com/v1/messages", json=payload, headers=headers, timeout=30)
    resp.raise_for_status()

    data = resp.json()
    content = data.get("content", [{}])[0].get("text", "")

    try:
        json_match = re.search(r'\{.*\}', content, re.DOTALL)
        if json_match:
            suggestion = json.loads(json_match.group(0))
            suggestion["success"] = True
            return suggestion
    except (json.JSONDecodeError, AttributeError):
        pass

    return {
        "success": False,
        "error": f"Could not parse AI response: {content[:200]}",
        "suggested_pattern": None,
        "format_name": None
    }


def create_format_file_from_suggestion(suggestion: dict, sender: str, subject: str) -> str | None:
    """Create a new format YAML file from an AI suggestion.

    Returns: path to created file, or None if failed.
    """
    if not suggestion.get("success") or not suggestion.get("format_name"):
        return None

    format_name = suggestion["format_name"]
    body_pattern = suggestion.get("body_pattern", "")
    reasoning = suggestion.get("reasoning", "")

    # Fallback: if body_pattern not provided, try to reconstruct from fields
    if not body_pattern:
        fields = suggestion.get("fields", {})
        if not fields:
            return None
        # This is a simple concatenation; ideally the AI provides body_pattern
        body_pattern = " ".join(fields.values())

    # Create YAML content with body pattern
    yaml_content = f"""# {reasoning}
# Generated by AI pattern suggestion
---
name: {format_name}
source: gmail
priority: 90
match:
  sender: {re.escape(sender.split('+')[0])}
  subject: {subject.split()[0] if subject else 'transaction'}
action: extract
fields:
  body: >-
    {body_pattern}
transforms:
  amount: decimal
  date:
    type: date
    formats: ["%d-%m-%y", "%d-%m-%Y", "%Y-%m-%d", "%d-%b-%y"]
  merchant: strip
set:
  currency: INR
  direction: debit
"""

    # Write to config directory
    output_file = FORMATS_DIR / f"email.{format_name}.yaml"
    try:
        output_file.parent.mkdir(parents=True, exist_ok=True)
        output_file.write_text(yaml_content)
        return str(output_file)
    except Exception as e:
        print(f"Error writing format file: {e}", file=sys.stderr)
        return None


# --------------------------------------------------------------------------
def main():
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--json", help="single document as a JSON string")
    ap.add_argument("--file", help="JSONL file, one document per line")
    ap.add_argument("--test", action="store_true", help="run regression tests")
    ap.add_argument("--ai-assist", action="store_true",
                    help="enable AI-powered pattern suggestions for unmatched emails (requires AI_PROVIDER env var)")
    args = ap.parse_args()

    if args.test:
        sys.exit(run_tests())
    if args.json:
        print(json.dumps(process(json.loads(args.json), ai_assist=args.ai_assist), indent=2))
        return
    if args.file:
        fmts, routes = load_formats(), load_routing()
        for line in open(args.file):
            line = line.strip()
            if line:
                print(json.dumps(process(json.loads(line), fmts, routes, ai_assist=args.ai_assist)))
        return
    ap.print_help()


if __name__ == "__main__":
    main()
