"""Transport, headers, error mapping, retries and metadata."""

from __future__ import annotations

import json
import time
from datetime import datetime, timezone

import pytest
from conftest import MockResponse, SleepRecorder, assert_problem

from wallet_sdk import (
    DEFAULT_BASE_URL,
    DEFAULT_USER_AGENT,
    APIError,
    DecodeError,
    ForbiddenError,
    InvalidRequestError,
    NotFoundError,
    RateLimitError,
    RetryConfig,
    ServerError,
    TransportError,
    UnauthorizedError,
    UnexpectedStatusError,
    ValidationError,
    WalletClient,
    WalletError,
)
from wallet_sdk.transport import parse_retry_after


# ---------------------------------------------------------------------------
# Construction
# ---------------------------------------------------------------------------


@pytest.mark.parametrize(
    ("kwargs", "field"),
    [
        ({"token": ""}, "token"),
        ({"token": "   "}, "token"),
        ({"token": None}, "token"),
        ({"token": "t", "base_url": "ftp://example.test"}, "base_url"),
        ({"token": "t", "base_url": "https://"}, "base_url"),
        ({"token": "t", "base_url": ""}, "base_url"),
    ],
)
def test_construction_validates_config(kwargs, field):
    with pytest.raises(InvalidRequestError) as excinfo:
        WalletClient(**kwargs)
    assert_problem(excinfo, field)


def test_construction_defaults():
    client = WalletClient(token="t")
    assert client.base_url == DEFAULT_BASE_URL
    assert client.last_meta.status_code == 0


def test_invalid_request_is_also_a_value_error():
    # Code that already guards against bad arguments keeps working.
    with pytest.raises(ValueError):
        WalletClient(token="")
    with pytest.raises(WalletError):
        WalletClient(token="")


def test_client_works_as_a_context_manager(server, make_client):
    server.queue(MockResponse(json_body={"accounts": []}))
    with make_client() as client:
        client.list_accounts()
    assert server.count == 1


# ---------------------------------------------------------------------------
# Request shape
# ---------------------------------------------------------------------------


def test_base_url_path_prefix_is_preserved(server, make_client):
    server.queue(MockResponse(json_body={"accounts": []}))
    client = make_client(base_url=server.url + "/proxy/wallet/")
    client.list_accounts()
    assert server.last.path == "/proxy/wallet/v1/api/accounts"


def test_request_carries_auth_and_default_headers(server, make_client):
    server.queue(MockResponse(json_body={"accounts": []}))
    client = make_client(user_agent="my-agent/2.0", headers={"X-Trace-Id": "abc123"})
    client.list_accounts()

    headers = server.last.headers
    assert headers["authorization"] == "Bearer test-token"
    assert headers["accept"] == "application/json"
    assert headers["user-agent"] == "my-agent/2.0"
    assert headers["x-trace-id"] == "abc123"
    # A GET has no body, so it must not claim a content type.
    assert "content-type" not in headers


def test_default_user_agent_is_sent(server, client):
    server.queue(MockResponse(json_body={"accounts": []}))
    client.list_accounts()
    assert server.last.headers["user-agent"] == DEFAULT_USER_AGENT


def test_custom_headers_cannot_override_authorization(server, make_client):
    server.queue(MockResponse(json_body={"accounts": []}))
    client = make_client(headers={"Authorization": "Bearer attacker"})
    client.list_accounts()
    assert server.last.headers["authorization"] == "Bearer test-token"


def test_post_sets_json_content_type(server, client):
    from wallet_sdk import CreateLabelRequest

    server.queue(MockResponse(status=201, json_body={"label": {"id": "l1"}}))
    client.create_label(CreateLabelRequest(name="vacation"))
    assert server.last.headers["content-type"] == "application/json"
    assert server.last.json() == {"name": "vacation"}


# ---------------------------------------------------------------------------
# Response metadata
# ---------------------------------------------------------------------------


def test_response_meta_parses_every_documented_header(server, client):
    server.queue(
        MockResponse(
            json_body={"accounts": []},
            headers={
                "X-RateLimit-Limit": "500",
                "X-RateLimit-Remaining": "487",
                "X-Last-Data-Change-At": "2024-01-28T14:23:45.123Z",
                "X-Last-Data-Change-Rev": "r1234",
                "X-Sync-In-Progress": "true",
            },
        )
    )
    page = client.list_accounts()
    meta = page.meta
    assert meta.status_code == 200
    assert meta.rate_limit_limit == 500
    assert meta.rate_limit_remaining == 487
    assert meta.last_data_change_rev == "r1234"
    assert meta.sync_in_progress is True
    assert meta.last_data_change_at == datetime(2024, 1, 28, 14, 23, 45, 123000, tzinfo=timezone.utc)
    assert meta.attempts == 1
    # last_meta mirrors the most recent response.
    assert client.last_meta.last_data_change_rev == "r1234"


def test_response_meta_tolerates_malformed_headers(server, client):
    server.queue(
        MockResponse(
            json_body={"accounts": []},
            headers={
                "X-RateLimit-Limit": "not-a-number",
                "X-Last-Data-Change-At": "yesterday",
                "X-Sync-In-Progress": "FALSE",
            },
        )
    )
    meta = client.list_accounts().meta
    assert meta.rate_limit_limit is None
    assert meta.last_data_change_at is None
    assert meta.sync_in_progress is False


def test_rate_limit_exhausted():
    from wallet_sdk import ResponseMeta

    assert ResponseMeta(rate_limit_remaining=0).rate_limit_exhausted
    assert not ResponseMeta(rate_limit_remaining=3).rate_limit_exhausted
    assert not ResponseMeta().rate_limit_exhausted


# ---------------------------------------------------------------------------
# Error mapping
# ---------------------------------------------------------------------------


@pytest.mark.parametrize(
    ("status", "body", "content_type", "exc", "message"),
    [
        (400, '{"error":"invalid parameter"}', "application/json", ValidationError, "invalid parameter"),
        (401, '{"error":"Invalid or expired token"}', "application/json", UnauthorizedError, "Invalid or expired token"),
        (403, "Forbidden - the token belongs to the other app.", "text/plain", ForbiddenError, "Forbidden - the token belongs to the other app."),
        (404, '{"error":"not found"}', "application/json", NotFoundError, "not found"),
        (429, '{"error":"Rate limit exceeded."}', "application/json", RateLimitError, "Rate limit exceeded."),
        (500, '{"error":"Internal Error"}', "application/json", ServerError, "Internal Error"),
        (503, '{"error":"unavailable"}', "application/json", ServerError, "unavailable"),
        (418, '{"error":"teapot"}', "application/json", UnexpectedStatusError, "teapot"),
    ],
)
def test_error_status_mapping(server, client, status, body, content_type, exc, message):
    server.queue(MockResponse(status=status, body=body, content_type=content_type))
    with pytest.raises(exc) as excinfo:
        client.list_accounts()
    error = excinfo.value
    assert error.status_code == status
    assert error.message == message
    assert error.body == body
    assert error.operation == "getAccounts"
    assert "getAccounts" in str(error)
    # Every wire error is an APIError, and every APIError a WalletError.
    assert isinstance(error, APIError)
    assert isinstance(error, WalletError)


@pytest.mark.parametrize(
    ("status", "body", "content_type", "fragment"),
    [
        (401, "", "application/json", "invalid or expired token"),
        (403, "", "text/plain", "token belongs to the other app"),
        (429, "", "application/json", "rate limit exceeded"),
        (500, "", "application/json", "internal server error"),
        (502, "<html><body>Bad Gateway</body></html>", "text/html", "<html>"),
        (400, '{"detail":"unknown shape"}', "application/json", "unknown shape"),
    ],
)
def test_error_message_falls_back_when_body_is_empty_or_opaque(
    server, client, status, body, content_type, fragment
):
    server.queue(MockResponse(status=status, body=body, content_type=content_type))
    with pytest.raises(APIError) as excinfo:
        client.list_accounts()
    assert fragment in excinfo.value.message


def test_rate_limit_error_carries_retry_after(server, client):
    server.queue(
        MockResponse(status=429, body='{"error":"slow down"}', headers={"Retry-After": "42"})
    )
    with pytest.raises(RateLimitError) as excinfo:
        client.list_accounts()
    assert excinfo.value.retry_after == 42.0
    assert "retry after 42s" in str(excinfo.value)


def test_retry_after_accepts_an_http_date(server, client):
    import email.utils

    future = email.utils.formatdate(time.time() + 90, usegmt=True)
    server.queue(MockResponse(status=429, body='{"error":"slow"}', headers={"Retry-After": future}))
    with pytest.raises(RateLimitError) as excinfo:
        client.list_accounts()
    assert 60 <= excinfo.value.retry_after <= 95


@pytest.mark.parametrize("value", ["", None, "garbage", "-5", "0"])
def test_retry_after_rejects_unusable_values(value):
    assert parse_retry_after(value) is None


def test_retry_after_in_the_past_is_ignored():
    import email.utils

    past = email.utils.formatdate(time.time() - 3600, usegmt=True)
    assert parse_retry_after(past) is None


def test_malformed_json_body_is_a_decode_error(server, client):
    server.queue(MockResponse(body='{"accounts": [ this is not json'))
    with pytest.raises(DecodeError) as excinfo:
        client.list_accounts()
    assert "not json" in excinfo.value.body
    assert excinfo.value.cause is not None


def test_empty_success_body_decodes_to_an_empty_page(server, client):
    server.queue(MockResponse(status=200, body=""))
    page = client.list_accounts()
    assert page.accounts == []
    assert page.meta.status_code == 200


def test_unknown_json_fields_are_ignored(server, client):
    server.queue(
        MockResponse(
            body=json.dumps(
                {
                    "accounts": [{"id": "a1", "name": "Checking", "brandNewField": {"nested": True}}],
                    "limit": 30,
                    "offset": 0,
                    "someFutureEnvelopeField": 123,
                }
            )
        )
    )
    page = client.list_accounts()
    assert [a.id for a in page.accounts] == ["a1"]
    # The unmodelled field is still reachable through raw.
    assert page.accounts[0].raw["brandNewField"] == {"nested": True}


def test_a_non_object_body_decodes_to_an_empty_envelope(server, client):
    server.queue(MockResponse(body="[1, 2, 3]"))
    page = client.list_accounts()
    assert page.accounts == []


def test_transport_error_is_typed(make_client, server):
    client = make_client(base_url="http://127.0.0.1:1")  # nothing is listening
    with pytest.raises(TransportError) as excinfo:
        client.list_accounts()
    assert excinfo.value.status_code is None
    assert excinfo.value.cause is not None


def test_timeout_is_a_transport_error(server, make_client):
    def slow(request, ordinal):
        time.sleep(1.0)
        return MockResponse(json_body={})

    server.respond_with(slow)
    client = make_client(timeout=0.15)
    with pytest.raises(TransportError):
        client.list_accounts()


# ---------------------------------------------------------------------------
# Retry policy
# ---------------------------------------------------------------------------


def test_retries_rate_limit_then_succeeds(server, make_client, sleeper):
    def handler(request, ordinal):
        if ordinal < 2:
            return MockResponse(status=429, body='{"error":"slow"}', headers={"Retry-After": "1"})
        return MockResponse(json_body={"accounts": [{"id": "a1"}]})

    server.respond_with(handler)
    client = make_client(retry=RetryConfig(max_retries=3, sleep=sleeper, random=lambda: 0.5))
    page = client.list_accounts()

    assert [a.id for a in page.accounts] == ["a1"]
    assert server.count == 3
    assert page.meta.attempts == 3
    # Retry-After wins over the computed backoff.
    assert sleeper.delays == [1.0, 1.0]


def test_retries_exhaust_and_raise_the_last_error(server, make_client, sleeper):
    server.queue(MockResponse(status=500, body='{"error":"boom"}'))
    client = make_client(
        retry=RetryConfig(max_retries=2, base_delay=0.001, no_jitter=True, sleep=sleeper)
    )
    with pytest.raises(ServerError) as excinfo:
        client.list_accounts()
    assert excinfo.value.message == "boom"
    assert server.count == 3  # the initial attempt plus two retries


@pytest.mark.parametrize("status", [400, 401, 403, 404])
def test_non_retryable_statuses_are_not_retried(server, make_client, sleeper, status):
    server.queue(MockResponse(status=status, body='{"error":"no"}'))
    client = make_client(
        retry=RetryConfig(max_retries=5, base_delay=0.001, no_jitter=True, sleep=sleeper)
    )
    with pytest.raises(APIError):
        client.list_accounts()
    assert server.count == 1


def test_writes_are_not_retried_by_default(server, make_client, sleeper):
    from wallet_sdk import CreateLabelRequest

    server.queue(MockResponse(status=500, body='{"error":"boom"}'))
    client = make_client(
        retry=RetryConfig(max_retries=3, base_delay=0.001, no_jitter=True, sleep=sleeper)
    )
    with pytest.raises(ServerError):
        client.create_label(CreateLabelRequest(name="x"))
    assert server.count == 1


def test_writes_are_retried_when_opted_in(server, make_client, sleeper):
    from wallet_sdk import CreateLabelRequest

    def handler(request, ordinal):
        if ordinal == 0:
            return MockResponse(status=500, body='{"error":"boom"}')
        return MockResponse(status=201, json_body={"label": {"id": "l1"}})

    server.respond_with(handler)
    client = make_client(
        retry=RetryConfig(
            max_retries=3, retry_writes=True, base_delay=0.001, no_jitter=True, sleep=sleeper
        )
    )
    response = client.create_label(CreateLabelRequest(name="x"))
    assert response.label.id == "l1"
    assert server.count == 2
    # The retried POST must re-send the same body, not an empty one.
    for request in server.requests:
        assert request.json() == {"name": "x"}


def test_delete_is_retried_because_it_is_idempotent(server, make_client, sleeper):
    from wallet_sdk import EntityType

    server.queue(MockResponse(status=500, body='{"error":"boom"}'))
    client = make_client(
        retry=RetryConfig(max_retries=2, base_delay=0.001, no_jitter=True, sleep=sleeper)
    )
    with pytest.raises(ServerError):
        client.delete(EntityType.LABELS, ["l1"])
    assert server.count == 3


def test_transport_errors_are_retried(make_client, sleeper):
    client = make_client(
        base_url="http://127.0.0.1:1",
        retry=RetryConfig(max_retries=2, base_delay=0.001, no_jitter=True, sleep=sleeper),
    )
    with pytest.raises(TransportError):
        client.list_accounts()
    assert len(sleeper.delays) == 2


def test_retry_disabled_entirely(server, make_client, sleeper):
    server.queue(MockResponse(status=503, body='{"error":"boom"}'))
    client = make_client(retry=RetryConfig(max_retries=0, sleep=sleeper))
    with pytest.raises(ServerError):
        client.list_accounts()
    assert server.count == 1


def test_custom_retry_predicate(server, make_client, sleeper):
    seen = []

    def predicate(attempt, error):
        seen.append((attempt, type(error).__name__))
        return isinstance(error, NotFoundError) and attempt < 1

    server.queue(MockResponse(status=404, body='{"error":"nope"}'))
    client = make_client(
        retry=RetryConfig(
            max_retries=2, base_delay=0.001, no_jitter=True, sleep=sleeper, should_retry=predicate
        )
    )
    with pytest.raises(NotFoundError):
        client.list_accounts()
    assert server.count == 2
    assert seen[0] == (0, "NotFoundError")


def test_backoff_grows_exponentially_and_is_capped():
    retry = RetryConfig(base_delay=0.1, max_delay=1.0, no_jitter=True)
    assert [retry.backoff(n) for n in (0, 1, 2, 3, 10)] == [0.1, 0.2, 0.4, 0.8, 1.0]


def test_backoff_jitter_stays_in_the_upper_half():
    floor = RetryConfig(base_delay=1.0, max_delay=60.0, random=lambda: 0.0)
    ceiling = RetryConfig(base_delay=1.0, max_delay=60.0, random=lambda: 1.0)
    assert floor.backoff(0) == 0.5
    assert ceiling.backoff(0) == 1.0


def test_retry_after_is_capped_by_max_delay():
    retry = RetryConfig(base_delay=1.0, max_delay=5.0, no_jitter=True)
    error = RateLimitError("slow", retry_after=3600)
    assert retry.backoff(0, error) == 5.0


def test_retry_after_can_be_ignored():
    retry = RetryConfig(base_delay=0.01, max_delay=60.0, no_jitter=True, ignore_retry_after=True)
    error = RateLimitError("slow", retry_after=3600)
    assert retry.backoff(0, error) == 0.01


# ---------------------------------------------------------------------------
# Validation switch
# ---------------------------------------------------------------------------


def test_validation_failure_sends_nothing(server, client):
    with pytest.raises(InvalidRequestError) as excinfo:
        client.list_accounts(limit=0)
    assert_problem(excinfo, "limit")
    assert server.count == 0


def test_skip_validation_lets_the_server_decide(server, make_client):
    server.queue(MockResponse(status=400, body='{"error":"limit must be between 1 and 200"}'))
    client = make_client(validate=False)
    with pytest.raises(ValidationError):
        client.list_accounts(limit=9999)
    assert server.count == 1
    assert server.last.q("limit") == "9999"


def test_validation_reports_every_problem_at_once(client):
    with pytest.raises(InvalidRequestError) as excinfo:
        client.list_accounts(limit=0, offset=-1, currency_code="EURO")
    error = excinfo.value
    assert len(error.problems) >= 3
    assert {"limit", "offset", "currencyCode"} <= set(error.fields)
    assert "3 problems" in str(error) or "problems" in str(error)
