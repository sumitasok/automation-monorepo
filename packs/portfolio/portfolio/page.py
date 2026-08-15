"""Putting the data document into the page.

The template carries a `<script id="explorer-data" type="application/json">`
block and nothing else data-shaped. Two variants come out of here:

  embedded — the document inside the block. This is THE DECLARED ARTEFACT:
             self-contained, renders from file:// with no network and no
             companion file (FR-037, FR-065, FR-073).

  fetch    — the block left empty and a fetch URL set. For the owner's own use
             only; NOT what the workspace serves, because browsers block
             fetch() of file:// URLs, which would break the very scenario
             FR-067 requires (research R-008).
"""
from __future__ import annotations

import json
import re
from pathlib import Path

BLOCK = re.compile(
    r'(<script id="explorer-data" type="application/json">)(.*?)(</script>)',
    re.DOTALL)

URL_BLOCK = re.compile(r'(<meta name="explorer-source" content=")([^"]*)(")')


class TemplateError(Exception):
    pass


def _encode(document: dict) -> str:
    """JSON safe to sit inside a <script> element.

    Escaping `</script>` matters even though the payload is numeric: one
    pathological instrument name would otherwise close the element early and
    produce a page that silently renders nothing.
    """
    blob = json.dumps(document, separators=(",", ":"), default=str)
    return blob.replace("</", "<\\/")


def render(template_path: Path, document: dict,
           fetch_url: str | None = None) -> str:
    html = Path(template_path).read_text()
    if not BLOCK.search(html):
        raise TemplateError(
            f'{template_path} has no <script id="explorer-data"> block — the '
            f'page has nowhere to read its figures from.')

    payload = "{}" if fetch_url else _encode(document)
    html = BLOCK.sub(lambda m: m.group(1) + payload + m.group(3), html, count=1)

    if URL_BLOCK.search(html):
        html = URL_BLOCK.sub(lambda m: m.group(1) + (fetch_url or "") + m.group(3),
                             html, count=1)

    if fetch_url:
        # The fetch variant still carries the document as its fallback, so an
        # unreachable location degrades to "stale but complete" rather than
        # "blank" (FR-039).
        html = html.replace("<!--FALLBACK-->",
                            f'<script id="explorer-fallback" '
                            f'type="application/json">{_encode(document)}</script>')
    return html
