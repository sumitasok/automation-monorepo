"""Query filter prefixes, typed helpers, and sort directives.

Certain query parameters reject an unprefixed value, so every filter this SDK
sends is built here. Range parameters take at most two conditions; text
parameters take one.
"""

from __future__ import annotations

from datetime import date, datetime, timezone
from decimal import Decimal
from typing import List, Optional, Sequence, Union

from ._problems import Problems
from .enums import FilterOp

__all__ = [
    "Filter",
    "eq",
    "gt",
    "gte",
    "lt",
    "lte",
    "contains",
    "contains_i",
    "time_filter",
    "date_filter",
    "amount_filter",
    "between",
    "between_dates",
    "on_date",
    "parse_filter",
    "format_time",
    "format_date",
    "format_amount",
    "Sort",
    "asc",
    "desc",
    "COMMON_SORT_FIELDS",
    "RECORD_SORT_FIELDS",
    "MAX_RANGE_CONDITIONS",
]

#: The ceiling the API documents for range parameters.
MAX_RANGE_CONDITIONS = 2

#: The fields every sortable list endpoint accepts.
COMMON_SORT_FIELDS = ("createdAt", "updatedAt")

#: The fields ``GET /records`` accepts.
RECORD_SORT_FIELDS = ("recordDate", "amount", "createdAt", "updatedAt")


class Filter:
    """One prefixed condition, e.g. ``gte.2024-01-01``."""

    __slots__ = ("op", "value")

    def __init__(self, op: FilterOp, value: str) -> None:
        self.op = op
        self.value = value

    def __str__(self) -> str:
        """Render the wire form, ``<op>.<value>``."""
        return f"{self.op.value}.{self.value}"

    def __repr__(self) -> str:  # pragma: no cover - debugging aid
        return f"Filter({self.op!r}, {self.value!r})"

    def __eq__(self, other: object) -> bool:
        if not isinstance(other, Filter):
            return NotImplemented
        return self.op == other.op and self.value == other.value

    def __hash__(self) -> int:
        return hash((self.op, self.value))


def format_time(value: Union[datetime, date, str]) -> str:
    """Render a timestamp the way the filter parameters expect: UTC, to the second."""
    if isinstance(value, str):
        return value
    if isinstance(value, datetime):
        moment = value if value.tzinfo else value.replace(tzinfo=timezone.utc)
        return moment.astimezone(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    return f"{value.isoformat()}T00:00:00Z"


def format_date(value: Union[datetime, date, str]) -> str:
    """Render a calendar date, ``YYYY-MM-DD``.

    Beware the asymmetry the API documents: a date-only ``recordDate`` on a
    WRITE is stored at 12:00:00Z, while a date-only bound in a FILTER means UTC
    midnight. ``lt.<date>`` therefore excludes records stamped that same day at
    noon.
    """
    if isinstance(value, str):
        return value
    if isinstance(value, datetime):
        moment = value if value.tzinfo else value.replace(tzinfo=timezone.utc)
        return moment.astimezone(timezone.utc).date().isoformat()
    return value.isoformat()


def format_amount(value: Union[int, float, str]) -> str:
    """Render a decimal amount, trimming trailing zeroes.

    100.50 goes out as ``100.5`` and 100.00 as ``100``, which is what the
    ``amount`` filter compares against.
    """
    if isinstance(value, str):
        return value
    if isinstance(value, int):
        return str(value)
    # repr gives the shortest round-tripping form, but switches to exponent
    # notation for very large or very small magnitudes, which the amount
    # filter does not read. Decimal re-renders that exactly in plain notation;
    # format(value, "f") would not, because it truncates at six decimals and
    # would silently turn 1e-7 into 0.
    text = repr(float(value))
    if "e" in text or "E" in text:
        text = format(Decimal(text), "f")
    if "." in text:
        text = text.rstrip("0").rstrip(".")
    return text or "0"


def eq(value: Union[str, int, float]) -> Filter:
    """Match exactly. Valid on both range and text parameters.

    On a date-only value in a range filter the server spans the whole UTC day.
    """
    return Filter(FilterOp.EQ, _scalar(value))


def gt(value: Union[str, int, float]) -> Filter:
    """Match values strictly greater than ``value``."""
    return Filter(FilterOp.GT, _scalar(value))


def gte(value: Union[str, int, float]) -> Filter:
    """Match values greater than or equal to ``value``."""
    return Filter(FilterOp.GTE, _scalar(value))


def lt(value: Union[str, int, float]) -> Filter:
    """Match values strictly less than ``value``."""
    return Filter(FilterOp.LT, _scalar(value))


def lte(value: Union[str, int, float]) -> Filter:
    """Match values less than or equal to ``value``."""
    return Filter(FilterOp.LTE, _scalar(value))


def contains(value: str) -> Filter:
    """Match a case-sensitive substring."""
    return Filter(FilterOp.CONTAINS, value)


def contains_i(value: str) -> Filter:
    """Match a case-insensitive substring."""
    return Filter(FilterOp.CONTAINS_I, value)


def _scalar(value: Union[str, int, float]) -> str:
    if isinstance(value, str):
        return value
    return format_amount(value)


def time_filter(op: FilterOp, moment: Union[datetime, date, str]) -> Filter:
    """Build a range condition over a timestamp."""
    return Filter(op, format_time(moment))


def date_filter(op: FilterOp, day: Union[datetime, date, str]) -> Filter:
    """Build a range condition over a calendar date."""
    return Filter(op, format_date(day))


def amount_filter(op: FilterOp, value: Union[int, float, str]) -> Filter:
    """Build a range condition over a decimal amount."""
    return Filter(op, format_amount(value))


def between(start: Union[datetime, date, str], end: Union[datetime, date, str]) -> List[Filter]:
    """Build the half-open timestamp range ``[start, end)``: ``gte.`` and ``lt.``."""
    return [time_filter(FilterOp.GTE, start), time_filter(FilterOp.LT, end)]


def between_dates(start: Union[datetime, date, str], end: Union[datetime, date, str]) -> List[Filter]:
    """Build the half-open date range ``[start, end)``: ``gte.`` and ``lt.``."""
    return [date_filter(FilterOp.GTE, start), date_filter(FilterOp.LT, end)]


def on_date(day: Union[datetime, date, str]) -> List[Filter]:
    """Build the single ``eq.`` condition that spans one whole UTC day."""
    return [date_filter(FilterOp.EQ, day)]


# Longest first, so "contains-i." is not mistaken for a value that happens to
# start with "contains." and so a dotted value is never split at the wrong dot.
_PARSE_ORDER = (
    FilterOp.CONTAINS_I,
    FilterOp.CONTAINS,
    FilterOp.GTE,
    FilterOp.LTE,
    FilterOp.GT,
    FilterOp.LT,
    FilterOp.EQ,
)


def parse_filter(text: str) -> Filter:
    """Read a wire-form filter such as ``gte.2024-01-01`` back into a :class:`Filter`.

    Raises:
        ValueError: when the prefix is missing or unknown — which is what the
            server would answer with a 400.
    """
    for op in _PARSE_ORDER:
        prefix = f"{op.value}."
        if text.startswith(prefix):
            return Filter(op, text[len(prefix):])
    raise ValueError(
        f"{text!r} has no recognised filter prefix "
        "(want one of eq., gt., gte., lt., lte., contains., contains-i.)"
    )


class Sort:
    """A ``sortBy`` value: a field plus a direction rendered as a ``+``/``-`` prefix."""

    __slots__ = ("field", "descending")

    def __init__(self, field: str, descending: bool = False) -> None:
        self.field = field
        self.descending = descending

    def __str__(self) -> str:
        """Render the wire form, e.g. ``-recordDate``."""
        return f"{'-' if self.descending else '+'}{self.field}"

    def __repr__(self) -> str:  # pragma: no cover - debugging aid
        return f"Sort({self.field!r}, descending={self.descending!r})"

    def __eq__(self, other: object) -> bool:
        if not isinstance(other, Sort):
            return NotImplemented
        return self.field == other.field and self.descending == other.descending

    def __hash__(self) -> int:
        return hash((self.field, self.descending))


def asc(field: str) -> Sort:
    """Sort ascending on ``field``."""
    return Sort(field, descending=False)


def desc(field: str) -> Sort:
    """Sort descending on ``field``."""
    return Sort(field, descending=True)


# ---------------------------------------------------------------------------
# Validation
# ---------------------------------------------------------------------------


def validate_range_filters(problems: Problems, field: str, filters: Optional[Sequence[Filter]]) -> None:
    """Check a range parameter: at most two conditions, each range-capable."""
    if not filters:
        return
    if len(filters) > MAX_RANGE_CONDITIONS:
        problems.add(
            field,
            f"at most {MAX_RANGE_CONDITIONS} conditions allowed, got {len(filters)}",
        )
    for index, item in enumerate(filters):
        at = f"{field}[{index}]"
        if not isinstance(item, Filter):
            problems.add(at, "must be a Filter built with eq/gt/gte/lt/lte")
            continue
        if not item.op.is_range:
            problems.add(
                at,
                f"{item.op.value!r} is not a range operator; use eq., gt., gte., lt. or lte.",
            )
        if not str(item.value).strip():
            problems.add(at, "value must not be empty")


def validate_text_filter(problems: Problems, field: str, value: Optional[Filter]) -> None:
    """Check a text parameter: one text-capable prefix and a non-empty value."""
    if value is None:
        return
    if not isinstance(value, Filter):
        problems.add(field, "must be a Filter built with eq/contains/contains_i")
        return
    if not value.op.is_text:
        problems.add(
            field,
            f"{value.op.value!r} is not a text operator; use eq., contains. or contains-i.",
        )
    if not value.value:
        problems.add(field, "value must not be empty")


def validate_sort(problems: Problems, field: str, value: Optional[Sort], allowed: Sequence[str]) -> None:
    """Check a sort against the fields an endpoint allows."""
    if value is None:
        return
    if not isinstance(value, Sort):
        problems.add(field, "must be a Sort built with asc() or desc()")
        return
    if not value.field:
        problems.add(field, "sort field must not be empty")
        return
    if value.field not in allowed:
        problems.add(
            field,
            f"{value.field!r} is not sortable here; allowed: {', '.join(allowed)}",
        )
