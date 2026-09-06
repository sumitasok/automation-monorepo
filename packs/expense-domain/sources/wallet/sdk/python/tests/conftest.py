"""The mock API harness: a real HTTP server the SDK talks to over a socket.

Every test here exercises the whole stack — URL building, headers, the urllib
transport, status mapping and decoding — rather than stubbing the transport
out, so a serialisation bug cannot hide behind a mock.
"""

from __future__ import annotations

import json
import sys
import threading
from dataclasses import dataclass, field
from datetime import datetime, timezone
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Any, Callable, Dict, List, Optional, Tuple
from urllib.parse import parse_qs, urlsplit

import pytest

# Make the package importable without installing it.
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from wallet_sdk import RetryConfig, WalletClient  # noqa: E402
from wallet_sdk.errors import InvalidRequestError  # noqa: E402

#: The clock every test shares, so nothing depends on wall time.
FIXED_NOW = datetime(2026, 9, 6, 12, 0, 0, tzinfo=timezone.utc)


@dataclass
class CapturedRequest:
    """Everything the mock server saw, so a test can assert on the wire form."""

    method: str
    path: str
    #: Query parameters, each key mapped to every value sent under it — a range
    #: filter sends the same key twice.
    query: Dict[str, List[str]]
    headers: Dict[str, str]
    body: bytes

    def json(self) -> Any:
        """The request body parsed as JSON."""
        return json.loads(self.body.decode("utf-8"))

    def q(self, key: str) -> Optional[str]:
        """The first value sent under ``key``, or ``None``."""
        values = self.query.get(key)
        return values[0] if values else None

    def qs(self, key: str) -> List[str]:
        """Every value sent under ``key``."""
        return list(self.query.get(key, ()))


@dataclass
class MockResponse:
    """One canned reply."""

    status: int = 200
    #: Sent verbatim when set.
    body: Optional[str] = None
    #: Serialised and sent when ``body`` is unset.
    json_body: Any = None
    headers: Dict[str, str] = field(default_factory=dict)
    content_type: str = "application/json"


class MockServer:
    """A live HTTP server that replays canned responses and records requests."""

    def __init__(self) -> None:
        self._lock = threading.Lock()
        self._queue: List[MockResponse] = []
        self._handler: Optional[Callable[[CapturedRequest, int], MockResponse]] = None
        self.requests: List[CapturedRequest] = []
        self._httpd = ThreadingHTTPServer(("127.0.0.1", 0), self._make_handler())
        # Daemon threads and a tight poll interval keep teardown instant; the
        # default half-second poll would dominate the suite's runtime.
        self._httpd.daemon_threads = True
        self._thread = threading.Thread(
            target=self._httpd.serve_forever, kwargs={"poll_interval": 0.01}, daemon=True
        )
        self._thread.start()

    @property
    def url(self) -> str:
        """The base URL a client should be pointed at."""
        host, port = self._httpd.server_address[:2]
        return f"http://{host}:{port}"

    def queue(self, *responses: MockResponse) -> "MockServer":
        """Replay these responses in order.

        When the queue runs dry the last one repeats, so a retry test need not
        predict the exact attempt count.
        """
        with self._lock:
            self._queue = list(responses)
            self._handler = None
        return self

    def respond_with(self, handler: Callable[[CapturedRequest, int], MockResponse]) -> "MockServer":
        """Drive replies from a callable, for tests that branch on the request.

        The handler receives the captured request and its 0-based ordinal.
        """
        with self._lock:
            self._handler = handler
            self._queue = []
        return self

    @property
    def count(self) -> int:
        """How many requests were received."""
        with self._lock:
            return len(self.requests)

    @property
    def last(self) -> CapturedRequest:
        """The most recent request."""
        with self._lock:
            assert self.requests, "no requests were received by the mock server"
            return self.requests[-1]

    @property
    def first(self) -> CapturedRequest:
        """The first request."""
        with self._lock:
            assert self.requests, "no requests were received by the mock server"
            return self.requests[0]

    def close(self) -> None:
        """Stop the server and join its thread."""
        self._httpd.shutdown()
        self._httpd.server_close()
        self._thread.join(timeout=5)

    def _next_response(self, captured: CapturedRequest) -> MockResponse:
        with self._lock:
            ordinal = len(self.requests)
            self.requests.append(captured)
            if self._handler is not None:
                handler = self._handler
            else:
                handler = None
                if not self._queue:
                    return MockResponse(status=200, body="{}")
                if len(self._queue) == 1:
                    return self._queue[0]
                return self._queue.pop(0)
        return handler(captured, ordinal)

    def _make_handler(self):
        server = self

        class Handler(BaseHTTPRequestHandler):
            protocol_version = "HTTP/1.1"

            def log_message(self, *args: Any) -> None:
                """Keep the test output clean."""

            def _serve(self) -> None:
                parts = urlsplit(self.path)
                length = int(self.headers.get("Content-Length") or 0)
                body = self.rfile.read(length) if length else b""
                captured = CapturedRequest(
                    method=self.command,
                    path=parts.path,
                    query=parse_qs(parts.query, keep_blank_values=True),
                    headers={k.lower(): v for k, v in self.headers.items()},
                    body=body,
                )
                response = server._next_response(captured)
                payload = b""
                if response.body is not None:
                    payload = response.body.encode("utf-8")
                elif response.json_body is not None:
                    payload = json.dumps(response.json_body).encode("utf-8")

                self.send_response(response.status)
                self.send_header("Content-Type", response.content_type)
                self.send_header("Content-Length", str(len(payload)))
                for key, value in response.headers.items():
                    self.send_header(key, value)
                self.end_headers()
                if payload:
                    self.wfile.write(payload)

            do_GET = _serve
            do_POST = _serve
            do_PATCH = _serve
            do_DELETE = _serve
            do_PUT = _serve

        return Handler


class SleepRecorder:
    """Stands in for the backoff sleep, recording delays without spending time."""

    def __init__(self) -> None:
        self.delays: List[float] = []

    def __call__(self, seconds: float) -> None:
        self.delays.append(seconds)


@pytest.fixture
def server() -> MockServer:
    """A live mock API server, torn down after the test."""
    mock = MockServer()
    try:
        yield mock
    finally:
        mock.close()


@pytest.fixture
def sleeper() -> SleepRecorder:
    """A sleep stub that records what the retry policy asked for."""
    return SleepRecorder()


@pytest.fixture
def make_client(server: MockServer, sleeper: SleepRecorder):
    """Build a client pointed at the mock server, with retries off by default."""

    def _make(**overrides: Any) -> WalletClient:
        retry = overrides.pop("retry", None)
        if retry is None:
            retry = RetryConfig(max_retries=0, sleep=sleeper, random=lambda: 0.5)
        kwargs: Dict[str, Any] = {
            "token": "test-token",
            "base_url": server.url,
            "retry": retry,
            "now": lambda: FIXED_NOW,
        }
        kwargs.update(overrides)
        return WalletClient(**kwargs)

    return _make


@pytest.fixture
def client(make_client) -> WalletClient:
    """The default client for the mock server."""
    return make_client()


def batch_body(status: int) -> str:
    """A two-item batch envelope matching the given status."""
    bodies = {
        200: {
            "results": [
                {"inputIndex": 0, "id": "a1", "success": True},
                {"inputIndex": 1, "id": "a2", "success": True},
            ],
            "summary": {
                "total": 2, "succeeded": 2, "clientErrors": 0,
                "serverErrors": 0, "documentsWritten": 2,
            },
        },
        207: {
            "results": [
                {"inputIndex": 0, "id": "a1", "success": True},
                {
                    "inputIndex": 1, "id": "a2", "success": False,
                    "error": "nope", "errorType": "client_error",
                },
            ],
            "summary": {
                "total": 2, "succeeded": 1, "clientErrors": 1,
                "serverErrors": 0, "documentsWritten": 1,
            },
        },
        400: {
            "results": [
                {"inputIndex": 0, "id": "a1", "success": False, "error": "bad", "errorType": "client_error"},
                {"inputIndex": 1, "id": "a2", "success": False, "error": "bad", "errorType": "client_error"},
            ],
            "summary": {
                "total": 2, "succeeded": 0, "clientErrors": 2,
                "serverErrors": 0, "documentsWritten": 0,
            },
        },
        500: {
            "results": [
                {"inputIndex": 0, "id": "a1", "success": False, "error": "boom", "errorType": "server_error"},
                {"inputIndex": 1, "id": "a2", "success": False, "error": "boom", "errorType": "server_error"},
            ],
            "summary": {
                "total": 2, "succeeded": 0, "clientErrors": 0,
                "serverErrors": 2, "documentsWritten": 0,
            },
        },
    }
    return json.dumps(bodies[status])


def assert_problem(excinfo: Any, field: str) -> None:
    """Assert that a validation failure named the given field path."""
    error = excinfo.value
    assert isinstance(error, InvalidRequestError)
    assert error.has_field(field), f"expected a problem on {field!r}, got {error.fields}\n{error}"


def make_ids(count: int) -> List[str]:
    """A list of ``count`` distinct identifiers."""
    return [f"id-{n}" for n in range(count)]


def long_text(length: int) -> str:
    """A string of ``length`` characters, for length-bound tests."""
    return "x" * length


def ok_batch(*results: Dict[str, Any]) -> str:
    """A 200 batch envelope wrapping the given per-item results."""
    return json.dumps(
        {
            "results": list(results),
            "summary": {
                "total": len(results),
                "succeeded": sum(1 for r in results if r.get("success")),
                "clientErrors": 0,
                "serverErrors": 0,
                "documentsWritten": len(results),
            },
        }
    )
