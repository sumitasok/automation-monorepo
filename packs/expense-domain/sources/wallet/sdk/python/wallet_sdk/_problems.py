"""The accumulator that collects validation failures into one exception."""

from __future__ import annotations

from typing import List, Optional

from .errors import InvalidRequestError, Problem

__all__ = ["Problems", "join_path"]


def join_path(prefix: str, field: str) -> str:
    """Join a path prefix with a field, keeping ``items[0]`` style indices tidy."""
    if not prefix:
        return field
    if not field:
        return prefix
    if field.startswith("["):
        return prefix + field
    return f"{prefix}.{field}"


class Problems:
    """Collects validation failures and collapses to nothing when empty.

    Callers build one of these, add to it unconditionally, and finish with
    :meth:`raise_if_any`, so a request reports every problem at once.
    """

    __slots__ = ("_items",)

    def __init__(self) -> None:
        self._items: List[Problem] = []

    def add(self, field: str, reason: str) -> None:
        """Record one failure."""
        self._items.append(Problem(field, reason))

    def extend(self, problems: List[Problem], prefix: str = "") -> None:
        """Fold in another validator's failures, optionally under a path prefix."""
        for p in problems:
            self._items.append(Problem(join_path(prefix, p.field), p.reason) if prefix else p)

    def absorb(self, err: Optional[BaseException], prefix: str = "") -> None:
        """Fold in an :class:`InvalidRequestError` raised by a nested validator."""
        if err is None:
            return
        if isinstance(err, InvalidRequestError):
            self.extend(err.problems, prefix)
            return
        self._items.append(Problem(prefix, str(err)))

    def capture(self, prefix: str, fn, *args, **kwargs) -> None:
        """Run a nested validator, folding any failure in under ``prefix``."""
        try:
            fn(*args, **kwargs)
        except InvalidRequestError as err:
            self.extend(err.problems, prefix)

    @property
    def items(self) -> List[Problem]:
        """Every failure recorded so far."""
        return list(self._items)

    def __bool__(self) -> bool:
        return bool(self._items)

    def __len__(self) -> int:
        return len(self._items)

    def raise_if_any(self) -> None:
        """Raise :class:`InvalidRequestError` when anything was recorded."""
        if self._items:
            raise InvalidRequestError(self._items)
