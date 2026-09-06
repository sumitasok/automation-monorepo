"""Goals, standing orders, occurrences and API usage — plus a sweep of the
error path across every endpoint."""

from __future__ import annotations

import json
from datetime import datetime, timezone

import pytest
from conftest import MockResponse, assert_problem

from wallet_sdk import (
    AccountType,
    AmountInput,
    BudgetType,
    CreateAccountRequest,
    CreateBudgetRequest,
    CreateLabelRequest,
    CreateRecordRequest,
    CreateSubcategoryRequest,
    EntityType,
    ForbiddenError,
    GoalState,
    Granularity,
    InvalidRequestError,
    PatchAccountItem,
    PatchBudgetItem,
    PatchCategoryItem,
    PatchLabelItem,
    PatchRecordItem,
    RecordType,
    Reminder,
    UnauthorizedError,
    contains,
    contains_i,
    eq,
    gte,
    lt,
)
from conftest import FIXED_NOW


def test_list_goals(server, client):
    server.queue(
        MockResponse(
            body=json.dumps(
                {
                    "goals": [
                        {
                            "id": "g1",
                            "name": "New laptop",
                            "note": "for work",
                            "color": "#673ab7",
                            "state": "active",
                            "stateUpdatedAt": "2025-06-15T10:30:00.000Z",
                            "desiredDate": "2025-12-31T00:00:00.000Z",
                            "initialAmount": {"currencyCode": "USD", "value": 500},
                            "targetAmount": {"currencyCode": "USD", "value": 10000},
                            "createdAt": "2024-01-01T00:00:00Z",
                        }
                    ]
                }
            )
        )
    )
    goal = client.list_goals(
        note=contains_i("work"), name=contains("laptop"), limit=10
    ).goals[0]
    assert goal.state is GoalState.ACTIVE
    assert goal.target_amount.value == 10000
    assert goal.initial_amount.currency_code == "USD"
    assert goal.desired_date == datetime(2025, 12, 31, tzinfo=timezone.utc)

    assert server.last.q("note") == "contains-i.work"
    assert server.last.q("name") == "contains.laptop"


@pytest.mark.parametrize(
    ("kwargs", "field"),
    [({"note": gte("x")}, "note"), ({"limit": 500}, "limit"), ({"name": lt("x")}, "name")],
)
def test_list_goals_validation(client, kwargs, field):
    with pytest.raises(InvalidRequestError) as excinfo:
        client.list_goals(**kwargs)
    assert_problem(excinfo, field)


@pytest.mark.parametrize("state", list(GoalState))
def test_all_goal_states_decode(server, client, state):
    server.queue(MockResponse(body=f'{{"goals":[{{"id":"g1","state":"{state.value}"}}]}}'))
    assert client.list_goals().goals[0].state is state


def test_list_standing_orders(server, client):
    server.queue(
        MockResponse(
            body=json.dumps(
                {
                    "standingOrders": [
                        {
                            "id": "so1",
                            "name": "Rent",
                            "accountId": "acc1",
                            "amount": 1234.56,
                            "currencyCode": "USD",
                            "categoryId": "c1",
                            "counterParty": "Landlord",
                            "note": "monthly",
                            "dueDate": "2025-12-01T00:00:00Z",
                            "dueDateNotificationEnabled": True,
                            "threeDaysBeforeNotificationEnabled": True,
                            "reminder": "3_days",
                            "generateFromDate": "2025-01-01",
                            "manualPayment": False,
                            "paymentCount": 12,
                            "recurrenceRule": "FREQ=MONTHLY;BYMONTHDAY=1",
                            "type": "expense",
                            "labels": [{"id": "l1", "name": "housing"}],
                            "createdAt": "2024-01-01T00:00:00Z",
                        }
                    ]
                }
            )
        )
    )
    order = client.list_standing_orders(currency_code="usd", label_id="l1").standing_orders[0]
    assert order.reminder is Reminder.THREE_DAYS
    assert order.type is RecordType.EXPENSE
    assert order.amount == 1234.56
    assert order.payment_count == 12
    assert order.recurrence_rule == "FREQ=MONTHLY;BYMONTHDAY=1"
    assert order.due_date_notification_enabled
    assert order.three_days_before_notification_enabled
    assert [label.name for label in order.labels] == ["housing"]


def test_standing_order_currency_validation(client):
    with pytest.raises(InvalidRequestError) as excinfo:
        client.list_standing_orders(currency_code="DOLLARS")
    assert_problem(excinfo, "currencyCode")


def test_list_standing_order_items(server, client):
    server.queue(
        MockResponse(
            body=json.dumps(
                {
                    "standingOrderItems": [
                        {
                            "id": "i1",
                            "standingOrderId": "so1",
                            "originalDate": "2025-01-01T00:00:00Z",
                            "alignedDate": "2025-01-03T00:00:00Z",
                            "paidDate": "2025-01-03T00:00:00Z",
                            "paidFromAppDate": "2025-01-03T00:00:00Z",
                            "dismissed": False,
                            "recordIds": ["r1", "r2"],
                        },
                        {
                            "id": "i2",
                            "standingOrderId": "so1",
                            "originalDate": "2025-02-01T00:00:00Z",
                            "dismissed": True,
                        },
                    ]
                }
            )
        )
    )
    page = client.list_standing_order_items(
        standing_order_id="so1",
        original_date=[gte("2025-01-01T00:00:00Z"), lt("2025-03-01T00:00:00Z")],
        paid_date=[gte("2025-01-01T00:00:00Z")],
        dismissed=False,
        record_id="r1",
    )
    paid, unpaid = page.standing_order_items
    assert paid.paid and paid.record_ids == ["r1", "r2"]
    assert not unpaid.paid and unpaid.dismissed

    request = server.last
    assert request.path == "/v1/api/standing-orders/items"
    assert len(request.qs("originalDate")) == 2
    assert request.q("standingOrderId") == "so1"
    assert request.q("dismissed") == "false"
    assert request.q("recordId") == "r1"


def test_standing_order_items_range_validation(client):
    with pytest.raises(InvalidRequestError) as excinfo:
        client.list_standing_order_items(paid_date=[gte("a"), lt("b"), eq("c")])
    assert_problem(excinfo, "paidDate")


# ---------------------------------------------------------------------------
# API usage
# ---------------------------------------------------------------------------


def test_api_usage_happy_path(server, client):
    server.queue(
        MockResponse(
            body=json.dumps(
                {
                    "period": "7days",
                    "granularity": "daily",
                    "from": "2026-08-30T00:00:00Z",
                    "to": "2026-09-06T00:00:00Z",
                    "total": 123,
                    "usage": [
                        {"from": "2026-08-30T00:00:00Z", "to": "2026-08-31T00:00:00Z", "total": 50},
                        {"from": "2026-09-01T00:00:00Z", "to": "2026-09-02T00:00:00Z", "total": 73},
                    ],
                }
            )
        )
    )
    stats = client.api_usage("7days")
    assert stats.granularity is Granularity.DAILY
    assert stats.total == 123
    # Zero-usage buckets are omitted, so the entries need not be contiguous.
    assert len(stats.usage) == 2
    assert stats.usage[1].from_ == datetime(2026, 9, 1, tzinfo=timezone.utc)

    assert server.last.path == "/v1/api/api-usage/stats"
    assert server.last.q("period") == "7days"


@pytest.mark.parametrize(
    "period", ["1days", "366days", "1weeks", "53weeks", "1months", "13months"]
)
def test_valid_usage_periods(server, client, period):
    server.queue(MockResponse(json_body={"period": period, "usage": []}))
    client.api_usage(period)
    assert server.count == 1


@pytest.mark.parametrize(
    "period",
    ["", "367days", "54weeks", "14months", "0days", "-5days", "30", "thirtydays", "30day", "1years"],
)
def test_invalid_usage_periods(server, client, period):
    with pytest.raises(InvalidRequestError) as excinfo:
        client.api_usage(period)
    assert_problem(excinfo, "period")
    assert server.count == 0


# ---------------------------------------------------------------------------
# Error path sweeps
# ---------------------------------------------------------------------------


READ_CALLS = {
    "accounts": lambda c: c.list_accounts(),
    "budgets": lambda c: c.list_budgets(),
    "categories": lambda c: c.list_categories(),
    "goals": lambda c: c.list_goals(),
    "labels": lambda c: c.list_labels(),
    "records": lambda c: c.list_records(),
    "record_rules": lambda c: c.list_record_rules(),
    "standing_orders": lambda c: c.list_standing_orders(),
    "standing_order_items": lambda c: c.list_standing_order_items(),
    "api_usage": lambda c: c.api_usage("7days"),
    "references": lambda c: c.references(EntityType.RECORDS, ["r1"]),
}


@pytest.mark.parametrize("name", sorted(READ_CALLS))
def test_every_read_endpoint_surfaces_a_401(server, client, name):
    server.queue(MockResponse(status=401, body='{"error":"Invalid or expired token"}'))
    with pytest.raises(UnauthorizedError):
        READ_CALLS[name](client)


WRITE_CALLS = {
    "create_account": lambda c: c.create_account(
        CreateAccountRequest(name="n", account_type=AccountType.CASH, currency_code="USD")
    ),
    "patch_accounts": lambda c: c.patch_accounts([PatchAccountItem(id="a1", archived=True)]),
    "create_budget": lambda c: c.create_budget(
        CreateBudgetRequest(name="n", currency_code="USD", type=BudgetType.MONTH, limit=1)
    ),
    "patch_budgets": lambda c: c.patch_budgets([PatchBudgetItem(id="b1", name="x")]),
    "create_subcategory": lambda c: c.create_subcategory(
        CreateSubcategoryRequest(name="n", parent_id="p")
    ),
    "patch_categories": lambda c: c.patch_categories([PatchCategoryItem(id="c1", name="x")]),
    "create_label": lambda c: c.create_label(CreateLabelRequest(name="n")),
    "patch_labels": lambda c: c.patch_labels([PatchLabelItem(id="l1", name="x")]),
    "create_records": lambda c: c.create_records(
        [
            CreateRecordRequest(
                account_id="acc1", amount=AmountInput(value=-1), record_date=FIXED_NOW
            )
        ]
    ),
    "patch_records": lambda c: c.patch_records([PatchRecordItem(id="r1", note="x")]),
    "delete": lambda c: c.delete(EntityType.RECORDS, ["r1"]),
}


@pytest.mark.parametrize("name", sorted(WRITE_CALLS))
def test_every_write_endpoint_surfaces_a_403(server, client, name):
    forbidden = "Forbidden - the token belongs to the other app."
    server.queue(MockResponse(status=403, body=forbidden, content_type="text/plain"))
    with pytest.raises(ForbiddenError) as excinfo:
        WRITE_CALLS[name](client)
    # The text/plain body comes through verbatim.
    assert excinfo.value.message == forbidden


@pytest.mark.parametrize("name", sorted(READ_CALLS))
def test_every_read_endpoint_records_meta(server, client, name):
    server.queue(
        MockResponse(json_body={}, headers={"X-RateLimit-Remaining": "9", "X-Last-Data-Change-Rev": "r7"})
    )
    READ_CALLS[name](client)
    assert client.last_meta.rate_limit_remaining == 9
    assert client.last_meta.last_data_change_rev == "r7"
