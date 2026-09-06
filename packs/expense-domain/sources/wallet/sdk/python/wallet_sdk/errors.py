"""Every failure mode the Wallet API and this SDK can produce.

Two families live here. :class:`APIError` and its subclasses describe what came
back over the wire; :class:`InvalidRequestError` describes what the SDK
rejected before anything was sent. Both derive from :class:`WalletError`, so a
caller that only wants "something went wrong" can catch one type.
"""

from __future__ import annotations

from typing import TYPE_CHECKING, List, Optional, Sequence

if TYPE_CHECKING:  # pragma: no cover - typing only
    from .models import ResponseMeta

__all__ = [
    "WalletError",
    "APIError",
    "ValidationError",
    "UnauthorizedError",
    "ForbiddenError",
    "NotFoundError",
    "RateLimitError",
    "ServerError",
    "UnexpectedStatusError",
    "DecodeError",
    "TransportError",
    "Problem",
    "InvalidRequestError",
    "error_for_status",
]

#: How much of an error body is kept on the exception.
MAX_ERROR_BODY = 8 * 1024


class WalletError(Exception):
    """Base class for everything this SDK raises."""


class APIError(WalletError):
    """A request that reached the server and came back unusable.

    Attributes:
        operation: The OpenAPI ``operationId`` that failed, e.g. ``getRecords``.
        status_code: The HTTP status, or ``None`` for a transport failure.
        message: The server's ``error`` field, the ``text/plain`` body, or a
            locally generated description.
        body: The response body verbatim, truncated to 8 KiB.
        retry_after: The parsed ``Retry-After`` header, in seconds.
        meta: The response metadata, when a response was received.
        cause: The underlying exception, for transport and decode failures.
    """

    #: Whether retrying the identical request could plausibly succeed. It is
    #: advisory: the retry policy also weighs the HTTP method, because a
    #: retried non-idempotent write can duplicate data.
    retryable: bool = False

    def __init__(
        self,
        message: str,
        *,
        operation: str = "",
        status_code: Optional[int] = None,
        body: str = "",
        retry_after: Optional[float] = None,
        meta: "Optional[ResponseMeta]" = None,
        cause: Optional[BaseException] = None,
    ) -> None:
        self.operation = operation
        self.status_code = status_code
        self.message = message
        self.body = body[:MAX_ERROR_BODY]
        self.retry_after = retry_after
        self.meta = meta
        self.cause = cause
        super().__init__(self._render())
        if cause is not None:
            self.__cause__ = cause

    def _render(self) -> str:
        parts: List[str] = []
        if self.operation:
            parts.append(f"{self.operation}:")
        if self.status_code is not None:
            parts.append(f"{self.status_code}:")
        parts.append(self.message or type(self).__name__)
        if self.retry_after:
            parts.append(f"(retry after {self.retry_after:g}s)")
        if self.cause is not None:
            parts.append(f"({self.cause})")
        return " ".join(parts)

    def __repr__(self) -> str:  # pragma: no cover - debugging aid
        return f"{type(self).__name__}({self._render()!r})"


class ValidationError(APIError):
    """HTTP 400 — the server rejected a parameter or body."""


class UnauthorizedError(APIError):
    """HTTP 401 — missing, invalid or expired token."""


class ForbiddenError(APIError):
    """HTTP 403 — the token belongs to the other app.

    Tokens carry the flavour of the login that minted them: a Wallet token
    works on ``/wallet/...``, a Board token on ``/board/...``. Re-authenticating
    against the same app does not help. The body is ``text/plain``.
    """


class NotFoundError(APIError):
    """HTTP 404 — e.g. the Board-only ``orders`` collection on Wallet."""


class RateLimitError(APIError):
    """HTTP 429 — the hourly request budget is spent.

    ``retry_after`` carries the ``Retry-After`` header when the server sent one.
    """

    retryable = True


class ServerError(APIError):
    """HTTP 5xx on a non-batch endpoint. The request was valid; retry later."""

    retryable = True


class UnexpectedStatusError(APIError):
    """A status the specification does not document."""


class DecodeError(APIError):
    """A response arrived but could not be decoded."""


class TransportError(APIError):
    """The request never produced an HTTP response."""

    retryable = True


#: Maps a status code onto the exception class that describes it.
_STATUS_ERRORS = {
    400: ValidationError,
    401: UnauthorizedError,
    403: ForbiddenError,
    404: NotFoundError,
    429: RateLimitError,
}

#: The message used when the server sends an empty error body.
_DEFAULT_MESSAGES = {
    401: "invalid or expired token",
    403: "token belongs to the other app; use a token minted for this API flavour",
    404: "endpoint or entity type not found",
    429: "rate limit exceeded",
}


def error_for_status(status: int) -> type:
    """Return the exception class describing an HTTP status."""
    cls = _STATUS_ERRORS.get(status)
    if cls is not None:
        return cls
    if status >= 500:
        return ServerError
    return UnexpectedStatusError


def default_message_for_status(status: int) -> str:
    """Return the fallback message for a status whose body was empty."""
    if status in _DEFAULT_MESSAGES:
        return _DEFAULT_MESSAGES[status]
    if status >= 500:
        return "internal server error"
    return f"unexpected status {status}"


class Problem:
    """One client-side validation failure.

    Attributes:
        field: A dotted path into the request, e.g. ``records[2].amount.value``.
            It is empty when the failure belongs to the request as a whole.
        reason: The constraint that was violated.
    """

    __slots__ = ("field", "reason")

    def __init__(self, field: str, reason: str) -> None:
        self.field = field
        self.reason = reason

    def __str__(self) -> str:
        return f"{self.field}: {self.reason}" if self.field else self.reason

    def __repr__(self) -> str:  # pragma: no cover - debugging aid
        return f"Problem({self.field!r}, {self.reason!r})"

    def __eq__(self, other: object) -> bool:
        if not isinstance(other, Problem):
            return NotImplemented
        return self.field == other.field and self.reason == other.reason

    def __hash__(self) -> int:
        return hash((self.field, self.reason))


class InvalidRequestError(WalletError, ValueError):
    """The SDK rejected the arguments before sending anything.

    It carries every problem found in one request, so a caller fixing a
    50-record batch sees all of them at once instead of one per round trip.

    It also subclasses :class:`ValueError`, so code that already guards against
    bad arguments keeps working.
    """

    def __init__(self, problems: Sequence[Problem]) -> None:
        self.problems: List[Problem] = list(problems)
        super().__init__(self._render())

    def _render(self) -> str:
        if len(self.problems) == 1:
            return f"invalid request: {self.problems[0]}"
        joined = "; ".join(str(p) for p in self.problems)
        return f"invalid request: {len(self.problems)} problems: {joined}"

    @property
    def fields(self) -> List[str]:
        """The field paths every problem names, in the order they were found."""
        return [p.field for p in self.problems]

    def has_field(self, field: str) -> bool:
        """Report whether any problem names the given field path."""
        return any(p.field == field for p in self.problems)
