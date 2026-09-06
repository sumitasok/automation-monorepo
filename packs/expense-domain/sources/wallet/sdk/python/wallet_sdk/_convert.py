"""Tolerant decoders shared by every model.

The API documents RFC 3339 with millisecond precision, but real payloads also
carry second-precision stamps, offsets other than ``Z``, bare dates, and nulls
where a value is merely absent. Decoding here never raises on a shape the
model can survive, because a client that dies on an unexpected null is a client
that dies in production.
"""

from __future__ import annotations

from datetime import date, datetime, timezone
from typing import Any, Callable, Dict, List, Optional, Type, TypeVar

__all__ = [
    "parse_datetime",
    "parse_date",
    "format_datetime",
    "format_date_value",
    "opt_str",
    "opt_bool",
    "opt_float",
    "opt_int",
    "as_str",
    "as_bool",
    "as_float",
    "as_int",
    "str_list",
    "enum_or_raw",
    "sub_model",
    "sub_models",
]

T = TypeVar("T")

#: Tried in order when decoding a timestamp.
_DATETIME_FORMATS = (
    "%Y-%m-%dT%H:%M:%S.%f%z",
    "%Y-%m-%dT%H:%M:%S%z",
    "%Y-%m-%dT%H:%M:%S.%f",
    "%Y-%m-%dT%H:%M:%S",
    "%Y-%m-%d",
)


def parse_datetime(value: Any) -> Optional[datetime]:
    """Decode a timestamp, or ``None`` when absent, null or unparseable.

    The result is always timezone-aware and normalised to UTC, so two
    timestamps from different offsets compare correctly.
    """
    if value is None or isinstance(value, bool):
        return None
    if isinstance(value, datetime):
        return value if value.tzinfo else value.replace(tzinfo=timezone.utc)
    if isinstance(value, date):
        return datetime(value.year, value.month, value.day, tzinfo=timezone.utc)
    if not isinstance(value, str):
        return None
    text = value.strip()
    if not text:
        return None
    # Python's %z accepts +0200 but historically not Z, so normalise first.
    normalised = text[:-1] + "+0000" if text.endswith("Z") else text
    for fmt in _DATETIME_FORMATS:
        try:
            parsed = datetime.strptime(normalised, fmt)
        except ValueError:
            continue
        return parsed if parsed.tzinfo else parsed.replace(tzinfo=timezone.utc)
    # Last resort: fromisoformat handles offsets like +02:00 on every version.
    try:
        parsed = datetime.fromisoformat(text.replace("Z", "+00:00"))
    except ValueError:
        return None
    return parsed if parsed.tzinfo else parsed.replace(tzinfo=timezone.utc)


def parse_date(value: Any) -> Optional[date]:
    """Decode a calendar date, keeping only the day part of a full timestamp."""
    if isinstance(value, datetime):
        return value.date()
    if isinstance(value, date):
        return value
    moment = parse_datetime(value)
    return moment.date() if moment is not None else None


def format_datetime(value: Any) -> Optional[str]:
    """Encode a timestamp in the canonical UTC millisecond form the API emits."""
    if value is None:
        return None
    if isinstance(value, str):
        return value
    if isinstance(value, datetime):
        moment = value if value.tzinfo else value.replace(tzinfo=timezone.utc)
        moment = moment.astimezone(timezone.utc)
        return moment.strftime("%Y-%m-%dT%H:%M:%S.") + f"{moment.microsecond // 1000:03d}Z"
    if isinstance(value, date):
        return f"{value.isoformat()}T00:00:00.000Z"
    raise TypeError(f"cannot encode {type(value).__name__} as a timestamp")


def format_date_value(value: Any) -> Optional[str]:
    """Encode a calendar date as ``YYYY-MM-DD``."""
    if value is None:
        return None
    if isinstance(value, str):
        return value
    if isinstance(value, datetime):
        moment = value if value.tzinfo else value.replace(tzinfo=timezone.utc)
        return moment.astimezone(timezone.utc).date().isoformat()
    if isinstance(value, date):
        return value.isoformat()
    raise TypeError(f"cannot encode {type(value).__name__} as a date")


def opt_str(data: Dict[str, Any], key: str) -> Optional[str]:
    """Read an optional string, treating a non-string as absent."""
    value = data.get(key)
    return value if isinstance(value, str) else None


def as_str(data: Dict[str, Any], key: str, default: str = "") -> str:
    """Read a string, falling back to ``default`` when absent or null."""
    value = data.get(key)
    return value if isinstance(value, str) else default


def opt_bool(data: Dict[str, Any], key: str) -> Optional[bool]:
    """Read an optional boolean, distinguishing absent from ``False``."""
    value = data.get(key)
    return value if isinstance(value, bool) else None


def as_bool(data: Dict[str, Any], key: str, default: bool = False) -> bool:
    """Read a boolean, falling back to ``default`` when absent or null."""
    value = data.get(key)
    return value if isinstance(value, bool) else default


def opt_float(data: Dict[str, Any], key: str) -> Optional[float]:
    """Read an optional number, distinguishing absent from ``0``."""
    value = data.get(key)
    if isinstance(value, bool) or value is None:
        return None
    if isinstance(value, (int, float)):
        return float(value)
    return None


def as_float(data: Dict[str, Any], key: str, default: float = 0.0) -> float:
    """Read a number, falling back to ``default`` when absent or null."""
    value = opt_float(data, key)
    return default if value is None else value


def opt_int(data: Dict[str, Any], key: str) -> Optional[int]:
    """Read an optional integer, distinguishing absent from ``0``."""
    value = data.get(key)
    if isinstance(value, bool) or value is None:
        return None
    if isinstance(value, int):
        return value
    if isinstance(value, float) and value.is_integer():
        return int(value)
    return None


def as_int(data: Dict[str, Any], key: str, default: int = 0) -> int:
    """Read an integer, falling back to ``default`` when absent or null."""
    value = opt_int(data, key)
    return default if value is None else value


def str_list(data: Dict[str, Any], key: str) -> List[str]:
    """Read a list of strings, skipping anything that is not one."""
    value = data.get(key)
    if not isinstance(value, list):
        return []
    return [item for item in value if isinstance(item, str)]


def enum_or_raw(enum_cls: Type[T], value: Any) -> Any:
    """Coerce to an enum member, keeping the raw value when it is unknown.

    A server that adds an enum member must not break an older client, so an
    unrecognised string round-trips verbatim.
    """
    if value is None:
        return None
    try:
        return enum_cls(value)  # type: ignore[call-arg]
    except (ValueError, TypeError):
        return value


def sub_model(factory: Callable[[Dict[str, Any]], T], value: Any) -> Optional[T]:
    """Build a nested model, or ``None`` when the field is absent or null."""
    if not isinstance(value, dict):
        return None
    return factory(value)


def sub_models(factory: Callable[[Dict[str, Any]], T], value: Any) -> List[T]:
    """Build a list of nested models, skipping entries that are not objects."""
    if not isinstance(value, list):
        return []
    return [factory(item) for item in value if isinstance(item, dict)]
