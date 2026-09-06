"""Error formatting, the problems accumulator, and the transport contract."""

from __future__ import annotations

import pytest

from wallet_sdk import (
    APIError,
    DecodeError,
    ForbiddenError,
    HTTPRequest,
    HTTPResponse,
    InvalidRequestError,
    NotFoundError,
    Problem,
    RateLimitError,
    ServerError,
    Transport,
    TransportError,
    UnauthorizedError,
    UnexpectedStatusError,
    ValidationError,
    WalletError,
)
from wallet_sdk._problems import Problems, join_path
from wallet_sdk.client import (
    _extract_message,
    _looks_like_batch_envelope,
    _meta_from_response,
)
from wallet_sdk.errors import default_message_for_status, error_for_status


# ---------------------------------------------------------------------------
# APIError
# ---------------------------------------------------------------------------


def test_api_error_renders_every_part():
    error = RateLimitError(
        "slow down", operation="getRecords", status_code=429, retry_after=30
    )
    rendered = str(error)
    for fragment in ("getRecords", "429", "slow down", "retry after 30s"):
        assert fragment in rendered


def test_api_error_without_operation_or_status():
    cause = OSError("connection refused")
    error = TransportError("request failed", cause=cause)
    rendered = str(error)
    assert "request failed" in rendered
    assert "connection refused" in rendered
    assert error.__cause__ is cause
    assert error.cause is cause


def test_api_error_falls_back_to_the_class_name():
    assert "DecodeError" in str(DecodeError(""))


def test_api_error_truncates_a_huge_body():
    error = ServerError("boom", body="x" * 20000)
    assert len(error.body) == 8 * 1024


def test_error_hierarchy():
    for cls in (
        ValidationError, UnauthorizedError, ForbiddenError, NotFoundError,
        RateLimitError, ServerError, UnexpectedStatusError, DecodeError, TransportError,
    ):
        assert issubclass(cls, APIError)
        assert issubclass(cls, WalletError)
    assert issubclass(InvalidRequestError, WalletError)
    assert issubclass(InvalidRequestError, ValueError)
    # InvalidRequestError is deliberately NOT an APIError: nothing was sent.
    assert not issubclass(InvalidRequestError, APIError)


@pytest.mark.parametrize(
    ("cls", "retryable"),
    [
        (RateLimitError, True),
        (ServerError, True),
        (TransportError, True),
        (ValidationError, False),
        (UnauthorizedError, False),
        (ForbiddenError, False),
        (NotFoundError, False),
        (DecodeError, False),
        (UnexpectedStatusError, False),
    ],
)
def test_retryable_classification(cls, retryable):
    assert cls("x").retryable is retryable


@pytest.mark.parametrize(
    ("status", "cls"),
    [
        (400, ValidationError),
        (401, UnauthorizedError),
        (403, ForbiddenError),
        (404, NotFoundError),
        (429, RateLimitError),
        (500, ServerError),
        (503, ServerError),
        (418, UnexpectedStatusError),
        (302, UnexpectedStatusError),
    ],
)
def test_error_for_status(status, cls):
    assert error_for_status(status) is cls


def test_default_messages():
    assert "expired token" in default_message_for_status(401)
    assert "other app" in default_message_for_status(403)
    assert "rate limit" in default_message_for_status(429)
    assert "internal server error" in default_message_for_status(500)
    assert "418" in default_message_for_status(418)


# ---------------------------------------------------------------------------
# InvalidRequestError
# ---------------------------------------------------------------------------


def test_invalid_request_renders_one_problem():
    error = InvalidRequestError([Problem("limit", "must be between 1 and 200")])
    assert str(error) == "invalid request: limit: must be between 1 and 200"
    assert error.fields == ["limit"]
    assert error.has_field("limit")
    assert not error.has_field("offset")


def test_invalid_request_renders_many_problems():
    error = InvalidRequestError(
        [Problem("limit", "too big"), Problem("", "body is required")]
    )
    rendered = str(error)
    assert "2 problems" in rendered
    assert "limit: too big" in rendered
    assert "body is required" in rendered


def test_problem_equality_and_rendering():
    assert Problem("a", "one") == Problem("a", "one")
    assert Problem("a", "one") != Problem("b", "one")
    assert Problem("a", "one") != "a: one"
    assert str(Problem("a", "one")) == "a: one"
    assert str(Problem("", "one")) == "one"
    assert len({Problem("a", "one"), Problem("a", "one")}) == 1


# ---------------------------------------------------------------------------
# Problems accumulator
# ---------------------------------------------------------------------------


def test_problems_collapses_to_nothing_when_empty():
    problems = Problems()
    assert not problems
    assert len(problems) == 0
    problems.raise_if_any()  # does not raise


def test_problems_aggregation():
    problems = Problems()
    problems.add("a", "reason 1")
    problems.absorb(None)
    problems.absorb(InvalidRequestError([Problem("b", "reason 2")]))
    problems.absorb(ValueError("a plain error"), prefix="c")
    assert len(problems) == 3
    with pytest.raises(InvalidRequestError) as excinfo:
        problems.raise_if_any()
    assert excinfo.value.fields == ["a", "b", "c"]


def test_problems_prefixing():
    problems = Problems()
    problems.absorb(InvalidRequestError([Problem("name", "too long")]), prefix="items[0]")
    problems.absorb(InvalidRequestError([Problem("", "nothing to update")]), prefix="items[1]")
    problems.absorb(InvalidRequestError([Problem("[0]", "indexed")]), prefix="root")
    with pytest.raises(InvalidRequestError) as excinfo:
        problems.raise_if_any()
    assert excinfo.value.fields == ["items[0].name", "items[1]", "root[0]"]


def test_problems_capture_runs_a_nested_validator():
    def failing():
        raise InvalidRequestError([Problem("inner", "bad")])

    problems = Problems()
    problems.capture("outer", failing)
    problems.capture("outer", lambda: None)
    assert problems.items == [Problem("outer.inner", "bad")]


@pytest.mark.parametrize(
    ("prefix", "field", "expected"),
    [
        ("", "name", "name"),
        ("items[0]", "", "items[0]"),
        ("items[0]", "name", "items[0].name"),
        ("items", "[0]", "items[0]"),
        ("", "", ""),
    ],
)
def test_join_path(prefix, field, expected):
    assert join_path(prefix, field) == expected


# ---------------------------------------------------------------------------
# Response helpers
# ---------------------------------------------------------------------------


@pytest.mark.parametrize(
    ("decoded", "is_batch"),
    [
        ({"results": [], "summary": {}}, True),
        ({"results": []}, True),
        ({"summary": {"total": 1}}, True),
        ({"error": "invalid parameter"}, False),
        ({}, False),
        ([], False),
        (None, False),
        ("not json", False),
    ],
)
def test_looks_like_batch_envelope(decoded, is_batch):
    assert _looks_like_batch_envelope(decoded) is is_batch


@pytest.mark.parametrize(
    ("content_type", "body", "expected"),
    [
        ("application/json", b'{"error":"invalid parameter"}', "invalid parameter"),
        ("application/json", b'{"message":"something"}', "something"),
        ("application/json", b'{"error":"a","message":"b"}', "a"),
        ("text/plain", b"Forbidden", "Forbidden"),
        ("", b'{"error":"sniffed"}', "sniffed"),
        ("application/json", b"", ""),
        ("application/json", b"   \n ", ""),
        ("application/json", b"{broken", "{broken"),
        ("application/json", b'{"detail":"x"}', '{"detail":"x"}'),
    ],
)
def test_extract_message(content_type, body, expected):
    assert _extract_message(content_type, body) == expected


def test_extract_message_truncates():
    long_body = b"y" * 2000
    assert len(_extract_message("text/plain", long_body)) == 512


def test_meta_from_response_reads_lowercased_headers():
    response = HTTPResponse(
        status=200,
        headers={
            "x-ratelimit-limit": "500",
            "x-ratelimit-remaining": "0",
            "x-last-data-change-rev": "r9",
            "x-sync-in-progress": "TRUE",
            "retry-after": "5",
        },
    )
    meta = _meta_from_response(response, attempt_no=2)
    assert meta.rate_limit_limit == 500
    assert meta.rate_limit_exhausted
    assert meta.last_data_change_rev == "r9"
    assert meta.sync_in_progress is True
    assert meta.retry_after == 5.0
    assert meta.attempts == 2


def test_http_response_helpers():
    response = HTTPResponse(
        status=200, headers={"content-type": "application/json"}, body=b'{"a":1}'
    )
    assert response.content_type == "application/json"
    assert response.json() == {"a": 1}
    assert response.text() == '{"a":1}'
    # An undecodable byte is replaced rather than raising.
    assert HTTPResponse(status=200, body=b"\xff").text()
    assert HTTPResponse(status=200).content_type == ""


def test_transport_interface_is_abstract():
    with pytest.raises(NotImplementedError):
        Transport().send(HTTPRequest(method="GET", url="http://example.test"))


def test_a_transport_that_raises_something_unexpected_becomes_a_transport_error():
    class Broken(Transport):
        def send(self, request):
            raise RuntimeError("kaboom")

    from wallet_sdk import WalletClient

    client = WalletClient(token="t", transport=Broken())
    with pytest.raises(TransportError) as excinfo:
        client.list_accounts()
    assert isinstance(excinfo.value.cause, RuntimeError)


def test_a_custom_transport_can_stand_in_for_the_network():
    class Canned(Transport):
        def __init__(self):
            self.seen = []

        def send(self, request):
            self.seen.append(request)
            return HTTPResponse(
                status=200,
                headers={"content-type": "application/json"},
                body=b'{"accounts":[{"id":"a1"}]}',
            )

    from wallet_sdk import WalletClient

    transport = Canned()
    client = WalletClient(token="t", transport=transport)
    page = client.list_accounts(limit=5)
    assert [a.id for a in page.accounts] == ["a1"]
    assert transport.seen[0].url.endswith("/v1/api/accounts?limit=5")
    assert transport.seen[0].headers["Authorization"] == "Bearer t"


def test_close_is_forwarded_to_a_transport_that_wants_it():
    class Closable(Transport):
        def __init__(self):
            self.closed = False

        def send(self, request):  # pragma: no cover - not exercised
            raise AssertionError

        def close(self):
            self.closed = True

    from wallet_sdk import WalletClient

    transport = Closable()
    with WalletClient(token="t", transport=transport):
        pass
    assert transport.closed
