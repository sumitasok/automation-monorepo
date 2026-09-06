"""The HTTP layer: one pluggable transport, plus the retry policy around it.

The default transport is built on :mod:`urllib.request`, so the SDK has no
runtime dependencies. Swap in your own by passing anything with a ``send``
method to the client — a ``requests`` session wrapper, or a stub in a test.
"""

from __future__ import annotations

import email.utils
import json
import random
import time
import urllib.error
import urllib.request
from dataclasses import dataclass, field
from typing import Any, Callable, Dict, Optional

from .errors import TransportError

__all__ = [
    "HTTPRequest",
    "HTTPResponse",
    "Transport",
    "UrllibTransport",
    "RetryConfig",
    "parse_retry_after",
    "DEFAULT_TIMEOUT",
    "DEFAULT_MAX_RETRIES",
    "DEFAULT_RETRY_BASE_DELAY",
    "DEFAULT_RETRY_MAX_DELAY",
    "MAX_RESPONSE_BYTES",
]

#: Bounds a single request including its body, in seconds.
DEFAULT_TIMEOUT = 30.0
#: How many times a retryable failure is re-sent.
DEFAULT_MAX_RETRIES = 3
#: The first backoff step, in seconds.
DEFAULT_RETRY_BASE_DELAY = 0.5
#: The backoff ceiling, in seconds.
DEFAULT_RETRY_MAX_DELAY = 30.0
#: Bounds a decoded response, so a runaway body cannot exhaust memory. 32 MiB
#: is far above a 200-item page.
MAX_RESPONSE_BYTES = 32 * 1024 * 1024


@dataclass
class HTTPRequest:
    """One outgoing HTTP request, fully resolved."""

    #: The HTTP method.
    method: str
    #: The absolute URL, query string included.
    url: str
    #: The request headers.
    headers: Dict[str, str] = field(default_factory=dict)
    #: The encoded body, or ``None`` for a body-less request.
    body: Optional[bytes] = None
    #: The per-request timeout in seconds.
    timeout: Optional[float] = None


@dataclass
class HTTPResponse:
    """One incoming HTTP response, fully read."""

    #: The HTTP status.
    status: int
    #: The response headers, with lower-cased keys.
    headers: Dict[str, str] = field(default_factory=dict)
    #: The raw body.
    body: bytes = b""

    @property
    def content_type(self) -> str:
        """The ``Content-Type`` header, or ``""``."""
        return self.headers.get("content-type", "")

    def text(self) -> str:
        """The body decoded as UTF-8, replacing anything undecodable."""
        return self.body.decode("utf-8", errors="replace")

    def json(self) -> Any:
        """The body parsed as JSON.

        Raises:
            ValueError: when the body is not valid JSON.
        """
        return json.loads(self.body.decode("utf-8"))


class Transport:
    """The interface a transport must satisfy."""

    def send(self, request: HTTPRequest) -> HTTPResponse:  # pragma: no cover - interface
        """Perform one request and return the response.

        Raises:
            TransportError: when no HTTP response was produced.
        """
        raise NotImplementedError


class UrllibTransport(Transport):
    """The default transport, built on :mod:`urllib.request`.

    An HTTP error status is a response, not an exception, so a 404 comes back
    through the normal path and the client's error mapping handles it.
    """

    def __init__(self, opener: Optional[urllib.request.OpenerDirector] = None) -> None:
        #: urllib follows redirects by default; nothing else is needed.
        self._opener = opener or urllib.request.build_opener()

    def send(self, request: HTTPRequest) -> HTTPResponse:
        """Perform one request and return the response."""
        req = urllib.request.Request(
            request.url,
            data=request.body,
            headers=request.headers,
            method=request.method,
        )
        try:
            with self._opener.open(req, timeout=request.timeout) as response:
                return _read_response(response.status, response.headers.items(), response)
        except urllib.error.HTTPError as err:
            # An error status is still a response: read it so the client can
            # map the status and surface the server's message.
            with err:
                return _read_response(err.code, err.headers.items(), err)
        except urllib.error.URLError as err:
            raise TransportError("request failed", cause=err) from err
        except (TimeoutError, OSError) as err:
            raise TransportError("request failed", cause=err) from err


def _read_response(status: int, header_items: Any, stream: Any) -> HTTPResponse:
    """Build a response, lower-casing headers and bounding the body."""
    headers = {str(k).lower(): str(v) for k, v in header_items}
    body = stream.read(MAX_RESPONSE_BYTES)
    return HTTPResponse(status=status, headers=headers, body=body or b"")


def parse_retry_after(value: Optional[str]) -> Optional[float]:
    """Parse ``Retry-After``, which is either delta-seconds or an HTTP date.

    A date in the past, or an unparseable value, yields ``None``.
    """
    if not value:
        return None
    text = value.strip()
    try:
        seconds = int(text)
    except ValueError:
        pass
    else:
        return float(seconds) if seconds > 0 else None
    # parsedate_to_datetime raises on anything it cannot read, so a garbage
    # header must not escape as an exception from a header parser.
    try:
        parsed = email.utils.parsedate_to_datetime(text)
    except (TypeError, ValueError):
        return None
    if parsed is None:  # pragma: no cover - older versions returned None
        return None
    try:
        delta = parsed.timestamp() - time.time()
    except (OverflowError, OSError, ValueError):  # pragma: no cover - exotic dates
        return None
    return delta if delta > 0 else None


@dataclass
class RetryConfig:
    """The retry policy.

    By default only idempotent requests are retried: ``GET``, and ``DELETE``
    (whose per-ID semantics the API documents as idempotent). Retrying a
    ``POST`` or ``PATCH`` can duplicate writes, so those need ``retry_writes``
    set explicitly.
    """

    #: How many extra attempts a retryable failure gets. Zero disables retries.
    max_retries: int = DEFAULT_MAX_RETRIES
    #: The first backoff step, in seconds.
    base_delay: float = DEFAULT_RETRY_BASE_DELAY
    #: The backoff ceiling, in seconds.
    max_delay: float = DEFAULT_RETRY_MAX_DELAY
    #: Opt ``POST`` and ``PATCH`` into retries. Off by default because a
    #: retried create can duplicate records.
    retry_writes: bool = False
    #: Make the backoff deterministic. Jitter is on by default so a fleet of
    #: clients does not resynchronise after a rate limit.
    no_jitter: bool = False
    #: Ignore ``Retry-After`` and always use the computed backoff.
    ignore_retry_after: bool = False
    #: Override which failures are retried. It receives the attempt number
    #: (0-based) and the :class:`~wallet_sdk.errors.APIError`, and returns
    #: whether to try again. When ``None`` the default policy applies.
    should_retry: Optional[Callable[[int, Any], bool]] = None
    #: The sleep function, swapped out in tests.
    sleep: Callable[[float], None] = time.sleep
    #: The jitter source, swapped out in tests.
    random: Callable[[], float] = random.random

    #: The methods retried without opting in.
    IDEMPOTENT_METHODS = frozenset({"GET", "HEAD", "DELETE"})

    def allows(self, method: str, attempt: int, error: Any) -> bool:
        """Whether one failure should be retried."""
        if self.should_retry is not None:
            return bool(self.should_retry(attempt, error))
        if not getattr(error, "retryable", False):
            return False
        if method.upper() in self.IDEMPOTENT_METHODS:
            return True
        return self.retry_writes

    def backoff(self, attempt: int, error: Any = None) -> float:
        """The wait before the next attempt, in seconds.

        ``Retry-After`` wins when the server sent one; otherwise the delay
        grows exponentially with full jitter, capped at ``max_delay``.
        """
        retry_after = getattr(error, "retry_after", None) if error is not None else None
        if retry_after and not self.ignore_retry_after:
            return min(float(retry_after), self.max_delay)
        delay = min(self.base_delay * (2 ** attempt), self.max_delay)
        if delay <= 0:
            delay = self.max_delay
        if self.no_jitter:
            return delay
        # Full jitter within the upper half: a floor is kept while a
        # thundering herd still spreads out.
        half = delay / 2
        return half + self.random() * half
