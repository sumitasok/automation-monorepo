"""Delete, references, and the safe delete that combines them."""

from __future__ import annotations

import json

import pytest
from conftest import MockResponse, assert_problem, make_ids, ok_batch

from wallet_sdk import (
    EntityType,
    InvalidRequestError,
    NotFoundError,
    ServerError,
    normalize_entity_type,
)
from wallet_sdk.enums import DELETABLE_ENTITY_TYPES, REFERENCEABLE_ENTITY_TYPES


def test_delete_happy_path(server, client):
    server.queue(
        MockResponse(
            status=200,
            body=ok_batch(
                {"inputIndex": 0, "id": "r1", "success": True},
                {"inputIndex": 1, "id": "r2", "success": True},
            ),
        )
    )
    response = client.delete(EntityType.RECORDS, ["r1", "r2"])
    assert response.summary.all_succeeded
    request = server.last
    assert request.method == "DELETE"
    assert request.path == "/v1/api/records"
    assert request.json() == {"ids": ["r1", "r2"]}


def test_delete_is_idempotent_per_id(server, client):
    # A repeated delete of an already-gone document succeeds while writing
    # nothing, which is why documentsWritten can trail succeeded.
    server.queue(
        MockResponse(
            status=200,
            body=json.dumps(
                {
                    "results": [{"inputIndex": 0, "id": "r1", "success": True}],
                    "summary": {
                        "total": 1, "succeeded": 1, "clientErrors": 0,
                        "serverErrors": 0, "documentsWritten": 0,
                    },
                }
            ),
        )
    )
    summary = client.delete(EntityType.RECORDS, ["r1"]).summary
    assert summary.succeeded == 1
    assert summary.documents_written == 0


def test_paired_transfer_rejection_is_per_item(server, client):
    server.queue(
        MockResponse(
            status=400,
            body=json.dumps(
                {
                    "results": [
                        {
                            "inputIndex": 0, "id": "r1", "success": False,
                            "error": "paired transfer: the other leg r2 must be deleted in the same request",
                            "errorType": "client_error", "fields": ["ids"],
                        }
                    ],
                    "summary": {
                        "total": 1, "succeeded": 0, "clientErrors": 1,
                        "serverErrors": 0, "documentsWritten": 0,
                    },
                }
            ),
        )
    )
    result = client.delete(EntityType.RECORDS, ["r1"]).results[0]
    assert not result.success
    assert "other leg r2" in result.error


@pytest.mark.parametrize(
    ("entity_type", "ids", "field"),
    [
        (EntityType.RECORDS, [], "ids"),
        (EntityType.RECORDS, make_ids(11), "ids"),
        (EntityType.RECORDS, ["r1", ""], "ids[1]"),
        (EntityType.RECORDS, ["r1", "r1"], "ids[1]"),
        ("", ["r1"], "type"),
        ("widgets", ["r1"], "type"),
        (EntityType.ORDERS, ["o1"], "type"),
    ],
)
def test_delete_validation(client, entity_type, ids, field):
    with pytest.raises(InvalidRequestError) as excinfo:
        client.delete(entity_type, ids)
    assert_problem(excinfo, field)


@pytest.mark.parametrize(
    "entity_type", [t for t in DELETABLE_ENTITY_TYPES if t is not EntityType.ORDERS]
)
def test_every_wallet_entity_type_is_deletable(server, client, entity_type):
    server.queue(
        MockResponse(status=200, body=ok_batch({"inputIndex": 0, "id": "x", "success": True}))
    )
    client.delete(entity_type, ["x"])
    assert server.last.path == f"/v1/api/{entity_type.value}"


def test_board_only_orders_is_caught_locally(server, client, make_client):
    with pytest.raises(InvalidRequestError) as excinfo:
        client.delete(EntityType.ORDERS, ["o1"])
    assert_problem(excinfo, "type")
    assert server.count == 0

    # Disabling validation lets the caller reach the server's 404.
    server.queue(MockResponse(status=404, body='{"error":"not found"}'))
    unchecked = make_client(validate=False)
    with pytest.raises(NotFoundError):
        unchecked.delete(EntityType.ORDERS, ["o1"])


def test_references_happy_path(server, client):
    server.queue(
        MockResponse(
            body=json.dumps(
                {
                    "550e8400-e29b-41d4-a716-446655440000": {
                        "records": {
                            "field": "categoryId", "ids": ["a1b2c3d4"],
                            "total": 3, "limit": 50, "hasMore": False,
                        },
                        "budgets": {
                            "field": "categoryIds", "ids": ["b2c3d4e5"],
                            "total": 1, "limit": 50, "hasMore": False,
                        },
                        "recordRules": {"field": "categoryId", "ids": [], "total": 0, "limit": 50},
                        "standingOrders": {"field": "categoryId", "ids": [], "total": 0, "limit": 50},
                    },
                    "missing-id": {"error": "not found"},
                    "wrong-type": {
                        "error": "type_mismatch",
                        "actualType": "account",
                        "message": "this id exists as an account, not a category",
                    },
                }
            )
        )
    )
    result = client.references(
        EntityType.CATEGORIES,
        ["550e8400-e29b-41d4-a716-446655440000", "missing-id", "wrong-type"],
    )
    assert server.last.path == "/v1/api/categories/references"
    assert "," in server.last.q("id")

    referenced = result["550e8400-e29b-41d4-a716-446655440000"]
    assert referenced.resolved
    assert referenced.total_references == 4
    assert referenced.records.field_name == "categoryId"
    assert set(referenced.referencing_kinds()) == {"records", "budgets"}

    missing = result["missing-id"]
    assert not missing.resolved and missing.error == "not found"

    mismatch = result["wrong-type"]
    assert mismatch.actual_type == "account" and mismatch.message

    assert set(result.unresolved()) == {"missing-id", "wrong-type"}
    assert result.safe_to_delete() == []
    assert len(result) == 3
    assert "missing-id" in result
    assert sorted(result) == sorted(result.entries)
    assert result.get("nope") is None


def test_references_safe_to_delete(server, client):
    server.queue(
        MockResponse(
            body=json.dumps(
                {
                    "free-1": {
                        "records": {"total": 0, "ids": [], "limit": 50},
                        "budgets": {"total": 0, "ids": [], "limit": 50},
                    },
                    "free-2": {"records": {"total": 0, "ids": [], "limit": 50}},
                    "busy": {"records": {"total": 7, "ids": ["r1"], "limit": 50, "hasMore": False}},
                }
            )
        )
    )
    result = client.references(EntityType.LABELS, ["free-1", "free-2", "busy"])
    assert result.safe_to_delete() == ["free-1", "free-2"]


@pytest.mark.parametrize(
    ("singular", "plural"),
    [
        ("category", EntityType.CATEGORIES),
        ("record", EntityType.RECORDS),
        ("account", EntityType.ACCOUNTS),
        ("label", EntityType.LABELS),
    ],
)
def test_references_normalises_singular_types(server, client, singular, plural):
    server.queue(MockResponse(body="{}"))
    client.references(singular, ["x"])
    assert server.last.path == f"/v1/api/{plural.value}/references"


@pytest.mark.parametrize(
    ("value", "expected"),
    [
        ("category", EntityType.CATEGORIES),
        ("CATEGORY", EntityType.CATEGORIES),
        ("records", EntityType.RECORDS),
        ("budgets", EntityType.BUDGETS),
        ("nonsense", "nonsense"),
        (EntityType.LABELS, EntityType.LABELS),
        (42, 42),
    ],
)
def test_normalize_entity_type(value, expected):
    assert normalize_entity_type(value) == expected


@pytest.mark.parametrize(
    ("entity_type", "ids", "field"),
    [
        (EntityType.RECORDS, [], "id"),
        (EntityType.RECORDS, make_ids(21), "id"),
        ("", ["r1"], "type"),
        (EntityType.BUDGETS, ["b1"], "type"),
        (EntityType.STANDING_ORDERS, ["s1"], "type"),
        (EntityType.RECORD_RULES, ["r1"], "type"),
        (EntityType.ORDERS, ["o1"], "type"),
    ],
)
def test_references_validation(client, entity_type, ids, field):
    with pytest.raises(InvalidRequestError) as excinfo:
        client.references(entity_type, ids)
    assert_problem(excinfo, field)


def test_references_accepts_exactly_twenty_ids(server, client):
    server.queue(MockResponse(body="{}"))
    client.references(EntityType.RECORDS, make_ids(20))
    assert server.count == 1


@pytest.mark.parametrize(
    ("entity_type", "needs_check"),
    [
        (EntityType.CATEGORIES, True),
        (EntityType.RECORDS, True),
        (EntityType.ACCOUNTS, True),
        (EntityType.LABELS, True),
        (EntityType.BUDGETS, False),
        (EntityType.STANDING_ORDERS, False),
        (EntityType.RECORD_RULES, False),
    ],
)
def test_which_types_can_be_referenced(entity_type, needs_check):
    assert (entity_type in REFERENCEABLE_ENTITY_TYPES) is needs_check


def test_delete_unreferenced_skips_referenced_ids(server, client):
    def handler(request, ordinal):
        if request.path.endswith("/references"):
            return MockResponse(
                body=json.dumps(
                    {
                        "free": {"records": {"total": 0, "ids": [], "limit": 50}},
                        "busy": {
                            "records": {"total": 2, "ids": ["r1", "r2"], "limit": 50},
                            "budgets": {"total": 1, "ids": ["b1"], "limit": 50},
                        },
                        "absent": {"error": "not found"},
                    }
                )
            )
        return MockResponse(
            status=200, body=ok_batch({"inputIndex": 0, "id": "free", "success": True})
        )

    server.respond_with(handler)
    response, refs, skipped = client.delete_unreferenced(
        EntityType.LABELS, ["free", "busy", "absent"]
    )
    assert refs is not None
    assert response.summary.succeeded == 1
    assert set(skipped) == {"busy", "absent"}
    assert "records" in skipped["busy"] and "budgets" in skipped["busy"]
    assert skipped["absent"] == "not found"
    # Only the free ID reached the delete.
    assert server.requests[1].json() == {"ids": ["free"]}


def test_delete_unreferenced_sends_no_delete_when_nothing_is_free(server, client):
    server.queue(
        MockResponse(body='{"busy":{"records":{"total":1,"ids":["r1"],"limit":50}}}')
    )
    response, refs, skipped = client.delete_unreferenced(EntityType.LABELS, ["busy"])
    assert response is None
    assert refs is not None
    assert set(skipped) == {"busy"}
    assert server.count == 1


def test_delete_unreferenced_skips_the_check_for_unreferenceable_types(server, client):
    server.queue(
        MockResponse(status=200, body=ok_batch({"inputIndex": 0, "id": "b1", "success": True}))
    )
    response, refs, skipped = client.delete_unreferenced(EntityType.BUDGETS, ["b1"])
    # Nothing references a budget, so no check should have run.
    assert refs is None and skipped == {}
    assert response is not None
    assert server.count == 1
    assert server.last.path == "/v1/api/budgets"


def test_delete_unreferenced_propagates_the_references_error(server, client):
    server.queue(MockResponse(status=500, body='{"error":"boom"}'))
    with pytest.raises(ServerError):
        client.delete_unreferenced(EntityType.RECORDS, ["r1"])


def test_delete_unreferenced_reports_a_missing_entry(server, client):
    # A server that answers with no entry for an id must not be treated as
    # "unreferenced, go ahead and delete".
    server.queue(MockResponse(body="{}"))
    response, refs, skipped = client.delete_unreferenced(EntityType.RECORDS, ["r1"])
    assert response is None
    assert skipped == {"r1": "no reference entry returned for this id"}
