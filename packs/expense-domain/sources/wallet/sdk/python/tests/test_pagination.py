"""Auto-pagination: page walking, early exit, and the loop guards."""

from __future__ import annotations

import itertools
import json

import pytest
from conftest import MockResponse

from wallet_sdk import ServerError, UnauthorizedError


def page_server(server, key: str, total: int, page_size: int):
    """Serve ``total`` items in pages of ``page_size``, with a next offset."""

    def handler(request, ordinal):
        offset = int(request.q("offset") or 0)
        end = min(offset + page_size, total)
        body = {
            "limit": page_size,
            "offset": offset,
            key: [{"id": f"item-{n}"} for n in range(offset, end)],
        }
        if end < total:
            body["nextOffset"] = end
        return MockResponse(body=json.dumps(body))

    server.respond_with(handler)
    return server


def test_iter_records_walks_every_page(server, client):
    page_server(server, "records", total=25, page_size=10)
    ids = [r.id for r in client.iter_records(limit=10)]

    assert len(ids) == 25
    assert ids[0] == "item-0" and ids[-1] == "item-24"
    assert server.count == 3
    # Each page carries the advancing offset while keeping the other params.
    assert [r.q("offset") for r in server.requests] == ["0", "10", "20"]
    assert all(r.q("limit") == "10" for r in server.requests)


def test_iteration_is_lazy(server, client):
    page_server(server, "records", total=100, page_size=10)
    iterator = client.iter_records()
    # Nothing is fetched until the first item is pulled.
    assert server.count == 0
    next(iterator)
    assert server.count == 1


def test_early_exit_stops_fetching(server, client):
    page_server(server, "records", total=100, page_size=10)
    taken = list(itertools.islice(client.iter_records(), 15))
    assert len(taken) == 15
    # The walk stops after the page that satisfied the slice.
    assert server.count == 2


def test_walk_stops_on_a_single_page(server, client):
    server.queue(MockResponse(body='{"limit":30,"offset":0,"records":[{"id":"r1"}]}'))
    assert len(list(client.iter_records())) == 1
    assert server.count == 1


def test_walk_stops_on_an_empty_result(server, client):
    server.queue(MockResponse(body='{"limit":30,"offset":0,"records":[]}'))
    assert list(client.iter_records()) == []
    assert server.count == 1


def test_walk_refuses_to_loop_on_a_non_advancing_next_offset(server, client):
    # A server that keeps answering with the same offset would spin forever.
    server.queue(
        MockResponse(body='{"limit":10,"offset":0,"nextOffset":0,"records":[{"id":"r1"}]}')
    )
    assert len(list(client.iter_records())) == 1
    assert server.count == 1


def test_walk_stops_on_an_empty_page_that_still_advertises_more(server, client):
    server.queue(MockResponse(body='{"limit":10,"offset":0,"nextOffset":10,"records":[]}'))
    assert list(client.iter_records()) == []
    assert server.count == 1


def test_walk_propagates_an_error_mid_walk(server, client):
    def handler(request, ordinal):
        if ordinal == 0:
            return MockResponse(
                body='{"limit":1,"offset":0,"nextOffset":1,"records":[{"id":"r1"}]}'
            )
        return MockResponse(status=500, body='{"error":"boom"}')

    server.respond_with(handler)
    seen = []
    with pytest.raises(ServerError):
        for record in client.iter_records():
            seen.append(record.id)
    assert seen == ["r1"]


def test_walk_starts_from_the_given_offset(server, client):
    page_server(server, "records", total=25, page_size=10)
    ids = [r.id for r in client.iter_records(limit=10, offset=20)]
    assert ids == [f"item-{n}" for n in range(20, 25)]


def test_walk_does_not_mutate_the_callers_arguments(server, client):
    page_server(server, "records", total=20, page_size=10)
    kwargs = {"limit": 10, "offset": 0}
    list(client.iter_records(**kwargs))
    assert kwargs == {"limit": 10, "offset": 0}


WALKERS = {
    "accounts": ("iter_accounts", "accounts"),
    "budgets": ("iter_budgets", "budgets"),
    "categories": ("iter_categories", "categories"),
    "labels": ("iter_labels", "labels"),
    "goals": ("iter_goals", "goals"),
    "record_rules": ("iter_record_rules", "recordRules"),
    "standing_orders": ("iter_standing_orders", "standingOrders"),
    "standing_order_items": ("iter_standing_order_items", "standingOrderItems"),
    "records": ("iter_records", "records"),
}


@pytest.mark.parametrize("name", sorted(WALKERS))
def test_every_list_endpoint_has_a_walker(server, client, name):
    method_name, key = WALKERS[name]
    page_server(server, key, total=7, page_size=3)
    items = list(getattr(client, method_name)())
    assert len(items) == 7
    assert server.count == 3


def test_walk_propagates_validation_errors_eagerly(client):
    from wallet_sdk import InvalidRequestError

    # A generator defers everything, so the failure surfaces on the first pull.
    with pytest.raises(InvalidRequestError):
        next(client.iter_records(limit=0))


def test_walk_surfaces_auth_errors(server, client):
    server.queue(MockResponse(status=401, body='{"error":"nope"}'))
    with pytest.raises(UnauthorizedError):
        list(client.iter_records())
