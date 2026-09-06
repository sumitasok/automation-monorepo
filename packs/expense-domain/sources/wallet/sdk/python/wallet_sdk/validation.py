"""The documented request limits, and the checks that enforce them locally.

Every constraint the specification states is checked before a request leaves
the process, so a caller learns about a violation without spending a request
on it. Set ``validate=False`` on the client to skip this and let the server
decide — useful when the API gains a value this SDK does not know yet.
"""

from __future__ import annotations

from typing import Any, Optional, Sequence

from ._problems import Problems
from .enums import (
    EntityType,
    StrEnum,
    UniversalColor,
    Cardinality,
    names_of,
)

__all__ = [
    "MIN_PAGE_LIMIT",
    "MAX_PAGE_LIMIT",
    "DEFAULT_PAGE_LIMIT",
    "MAX_ID_FILTER_VALUES",
    "MAX_ACCOUNT_ID_FILTER_VALUES",
    "MAX_CATEGORY_ID_FILTER_VALUES",
    "MAX_CREATE_RECORDS",
    "MAX_PATCH_ITEMS",
    "MAX_DELETE_IDS",
    "MAX_REFERENCE_IDS",
    "MAX_LIMIT_OVERRIDES",
    "MAX_NAME_LENGTH",
    "MAX_NOTE_LENGTH",
    "MAX_COUNTER_PARTY_LENGTH",
    "MAX_ADDRESS_LENGTH",
    "MAX_PLACE_NAME_LENGTH",
    "MAX_BANK_ACCOUNT_NUMBER_LENGTH",
    "MIN_PERIOD_COUNT",
    "MAX_PERIOD_COUNT",
    "MAX_RECORD_DATE_FUTURE_SECONDS",
]

# Pagination.
MIN_PAGE_LIMIT = 1
MAX_PAGE_LIMIT = 200
DEFAULT_PAGE_LIMIT = 30

# Comma-separated filter ceilings.
MAX_ID_FILTER_VALUES = 30
MAX_ACCOUNT_ID_FILTER_VALUES = 10
MAX_CATEGORY_ID_FILTER_VALUES = 10

# Batch ceilings.
MAX_CREATE_RECORDS = 50
MAX_PATCH_ITEMS = 10
MAX_DELETE_IDS = 10
MAX_REFERENCE_IDS = 20
MAX_LIMIT_OVERRIDES = 50

# String bounds.
MAX_NAME_LENGTH = 80
MAX_NOTE_LENGTH = 255
MAX_COUNTER_PARTY_LENGTH = 255
MAX_ADDRESS_LENGTH = 255
MAX_PLACE_NAME_LENGTH = 255
MAX_BANK_ACCOUNT_NUMBER_LENGTH = 80

# Budget override bounds.
MIN_PERIOD_COUNT = 1
MAX_PERIOD_COUNT = 120

#: How far ahead a created record's date may sit.
MAX_RECORD_DATE_FUTURE_SECONDS = 24 * 60 * 60


def check_limit(problems: Problems, value: Optional[int], field: str = "limit") -> None:
    """Check a page size against the documented 1-200 range."""
    if value is None:
        return
    if not isinstance(value, int) or isinstance(value, bool):
        problems.add(field, f"must be an integer, got {type(value).__name__}")
        return
    if value < MIN_PAGE_LIMIT or value > MAX_PAGE_LIMIT:
        problems.add(field, f"must be between {MIN_PAGE_LIMIT} and {MAX_PAGE_LIMIT}, got {value}")


def check_offset(problems: Problems, value: Optional[int], field: str = "offset") -> None:
    """Check a pagination offset is not negative."""
    if value is None:
        return
    if not isinstance(value, int) or isinstance(value, bool):
        problems.add(field, f"must be an integer, got {type(value).__name__}")
        return
    if value < 0:
        problems.add(field, f"must not be negative, got {value}")


def check_id_list(problems: Problems, field: str, ids: Optional[Sequence[str]], maximum: int) -> None:
    """Check a comma-separated ID filter: no blanks, no duplicates, not too many."""
    if ids is None:
        return
    if isinstance(ids, str):
        problems.add(field, "must be a list of ids, not a single comma-joined string")
        return
    values = list(ids)
    if len(values) > maximum:
        problems.add(field, f"at most {maximum} values allowed, got {len(values)}")
    seen = {}
    for index, value in enumerate(values):
        at = f"{field}[{index}]"
        if not isinstance(value, str):
            problems.add(at, f"must be a string, got {type(value).__name__}")
            continue
        if not value.strip():
            problems.add(at, "must not be empty")
            continue
        if "," in value:
            problems.add(at, "must not contain a comma; pass separate values instead")
        if value in seen:
            problems.add(at, f"duplicates {field}[{seen[value]}] ({value!r})")
            continue
        seen[value] = index


def check_required_id(problems: Problems, field: str, value: Optional[str]) -> None:
    """Check a single mandatory identifier."""
    if not isinstance(value, str) or not value.strip():
        problems.add(field, "is required")


def check_max_length(problems: Problems, field: str, value: Optional[str], maximum: int) -> None:
    """Bound a string field, counting characters rather than bytes."""
    if value is None:
        return
    if not isinstance(value, str):
        problems.add(field, f"must be a string, got {type(value).__name__}")
        return
    if len(value) > maximum:
        problems.add(field, f"must be at most {maximum} characters, got {len(value)}")


def check_required_text(problems: Problems, field: str, value: Optional[str], maximum: int) -> None:
    """Check a mandatory string with a length bound."""
    if not isinstance(value, str) or not value.strip():
        problems.add(field, "is required")
        if not isinstance(value, str):
            return
    check_max_length(problems, field, value, maximum)


def check_currency_code(problems: Problems, field: str, value: Optional[str]) -> None:
    """Check the shape of an ISO 4217 code.

    The API resolves the code itself; this only rejects values that cannot
    possibly be one.
    """
    if value is None or value == "":
        problems.add(field, "is required")
        return
    if not isinstance(value, str) or len(value) != 3 or not value.isalpha():
        problems.add(field, f"must be a 3-letter ISO 4217 code, got {value!r}")


def check_optional_currency_code(problems: Problems, field: str, value: Optional[str]) -> None:
    """Check a currency code only when one was supplied."""
    if value is None:
        return
    check_currency_code(problems, field, value)


def check_enum(
    problems: Problems,
    field: str,
    value: Any,
    allowed: Sequence[StrEnum],
    noun: str,
) -> None:
    """Check a value against an allow-list of enum members."""
    if value is None:
        return
    if value not in allowed:
        problems.add(field, f"{str(value)!r} is not {noun}; allowed: {names_of(allowed)}")


def check_color(problems: Problems, field: str, value: Any) -> None:
    """Check a colour against the 16 universal names the write endpoints accept."""
    if value is None:
        return
    if not UniversalColor.is_valid(value):
        problems.add(
            field,
            f"{str(value)!r} is not a universal colour name; allowed: {names_of(tuple(UniversalColor))}",
        )


def check_cardinality(problems: Problems, field: str, value: Any) -> None:
    """Check a cardinality against the four documented values."""
    if value is None:
        return
    if not Cardinality.is_valid(value):
        problems.add(
            field,
            f"{str(value)!r} is not a cardinality; allowed: {names_of(tuple(Cardinality))}",
        )


def check_amount_value(problems: Problems, field: str, value: Any) -> None:
    """Reject the one amount the API refuses outright."""
    if value is None:
        problems.add(field, "is required")
        return
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        problems.add(field, f"must be a number, got {type(value).__name__}")
        return
    if value == 0:
        problems.add(field, "must not be zero; negative is an expense, positive an income")


def check_batch_size(problems: Problems, field: str, count: int, maximum: int) -> None:
    """Bound a batch and reject an empty one, which the API answers with a 400."""
    if count == 0:
        problems.add(field, "must contain at least 1 item")
    elif count > maximum:
        problems.add(field, f"must contain at most {maximum} items, got {count}")


def check_period_format(problems: Problems, field: str, period: Optional[str]) -> None:
    """Check a budget override period: ``YYYY``, ``YYYY-MM`` or ``YYYY-Www``."""
    if not period:
        problems.add(field, "is required")
        return
    if not isinstance(period, str):
        problems.add(field, f"must be a string, got {type(period).__name__}")
        return

    bad = f"must be YYYY-MM (monthly), YYYY-Www (weekly) or YYYY (yearly), got {period!r}"
    if len(period) == 4:
        if not period.isdigit():
            problems.add(field, bad)
        return
    if len(period) == 7 and period[4] == "-":
        year, month = period[:4], period[5:]
        if not year.isdigit() or not month.isdigit() or not 1 <= int(month) <= 12:
            problems.add(field, f"monthly period must be YYYY-MM, got {period!r}")
        return
    if len(period) == 8 and period[4] == "-" and period[5] in "Ww":
        year, week = period[:4], period[6:]
        if not year.isdigit() or not week.isdigit() or not 1 <= int(week) <= 53:
            problems.add(field, f"weekly period must be YYYY-Www with week 01-53, got {period!r}")
        return
    problems.add(field, bad)


#: The ceilings the api-usage ``period`` parameter documents per unit.
_USAGE_PERIOD_LIMITS = (("days", 366), ("weeks", 53), ("months", 13))


def check_usage_period(problems: Problems, field: str, period: Optional[str]) -> None:
    """Check the ``Xdays``/``Xweeks``/``Xmonths`` period and its per-unit ceiling."""
    if not period:
        problems.add(field, "is required")
        return
    if not isinstance(period, str):
        problems.add(field, f"must be a string, got {type(period).__name__}")
        return
    for suffix, maximum in _USAGE_PERIOD_LIMITS:
        if not period.endswith(suffix):
            continue
        head = period[: -len(suffix)]
        if not head.isdigit():
            problems.add(field, f"must be a number followed by days, weeks or months, got {period!r}")
            return
        count = int(head)
        if count < 1:
            problems.add(field, f"must be at least 1{suffix}, got {period!r}")
        elif count > maximum:
            problems.add(field, f"at most {maximum}{suffix} allowed, got {period!r}")
        return
    problems.add(field, f"must be Xdays, Xweeks or Xmonths, got {period!r}")


def check_spending_depth(problems: Problems, field: str, value: Optional[str]) -> None:
    """Check the budgets ``spending`` parameter: none, current, or current+N."""
    if value is None or value in ("", "none", "current"):
        return
    if isinstance(value, str) and value.startswith("current+"):
        tail = value[len("current+"):]
        if tail.isdigit() and int(tail) >= 1:
            return
        problems.add(field, f"must be none, current or current+N with N >= 1, got {value!r}")
        return
    problems.add(field, f"must be none, current or current+N, got {value!r}")


def check_entity_type(
    problems: Problems,
    field: str,
    value: Any,
    allowed: Sequence[EntityType],
) -> None:
    """Check a ``/{type}`` path segment, flagging the Board-only collection."""
    if value is None or value == "":
        problems.add(field, "is required")
        return
    if value not in allowed:
        problems.add(field, f"{str(value)!r} is not supported here; allowed: {names_of(allowed)}")
        return
    if value == EntityType.ORDERS:
        problems.add(field, f"{str(value)!r} is Board-only; the Wallet API answers 404 for it")
