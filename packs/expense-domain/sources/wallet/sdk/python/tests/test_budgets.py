"""Budgets: spending blocks, custom intervals, and limit overrides."""

from __future__ import annotations

import json
from datetime import date

import pytest
from conftest import MockResponse, assert_problem, long_text, make_ids, ok_batch

from wallet_sdk import (
    BudgetType,
    CreateBudgetRequest,
    InvalidRequestError,
    LimitOverrideInput,
    PatchBudgetItem,
    add_remove_ids,
    clear_ids,
    contains,
    eq,
    gte,
    lt,
    lte,
    replace_ids,
)
from wallet_sdk.enums import CREATABLE_BUDGET_TYPES


def test_list_budgets_serialises_every_parameter(server, client):
    server.queue(MockResponse(json_body={"budgets": []}))
    client.list_budgets(
        limit=10,
        offset=5,
        agent_hints=True,
        with_total=False,
        ids=["b1"],
        name=contains("Food"),
        currency_code="USD",
        closed=False,
        type=BudgetType.MONTH,
        label_id="l1",
        account_id="acc1",
        category_ids=["c1", "c2"],
        start_date=[gte("2024-01-01")],
        end_date=[lt("2024-12-31")],
        created_at=[gte("2024-01-01T00:00:00Z")],
        updated_at=[lte("2024-06-01T00:00:00Z")],
        spending="current+3",
    )
    request = server.last
    expected = {
        "limit": "10", "offset": "5", "agentHints": "true", "withTotal": "false",
        "id": "b1", "name": "contains.Food", "currencyCode": "USD", "closed": "false",
        "type": "BUDGET_INTERVAL_MONTH", "labelId": "l1", "accountId": "acc1",
        "categoryId": "c1,c2", "startDate": "gte.2024-01-01", "endDate": "lt.2024-12-31",
        "spending": "current+3",
    }
    for key, value in expected.items():
        assert request.q(key) == value, key


def test_list_budgets_decodes_the_spending_block(server, client):
    server.queue(
        MockResponse(
            body=json.dumps(
                {
                    "budgets": [
                        {
                            "id": "b1",
                            "name": "Monthly Food",
                            "type": "BUDGET_INTERVAL_MONTH",
                            "currencyCode": "USD",
                            "limit": 300,
                            "closed": False,
                            "startDate": "2026-01-01",
                            "endDate": "2027-01-01",
                            "accountIds": ["acc1"],
                            "categoryIds": ["c1"],
                            "labelIds": [],
                            "limitOverrides": [
                                {
                                    "limit": 400, "period": "2026-03",
                                    "periodStart": "2026-03-01", "periodCount": 2,
                                    "createdAt": "2026-02-01T00:00:00Z",
                                }
                            ],
                            "pastLimitOverrides": [],
                            "spending": {
                                "computedAt": "2026-09-06T00:00:00Z",
                                "current": {
                                    "period": "2026-09", "effectiveLimit": 300,
                                    "spent": 120.5, "remaining": 179.5, "progress": 0.4017,
                                    "overspent": None, "incomplete": True, "recordCount": 8,
                                    "periodStart": "2026-09-01", "periodEnd": "2026-09-30",
                                    "totalExpenses": 120.5, "totalIncomes": 0,
                                    "convertedCurrencies": ["EUR"],
                                    "excluded": {
                                        "total": 3, "transfers": 1, "debts": 0,
                                        "incomeCategories": 1, "unknownCategories": 1,
                                        "totalAmountSum": -45.2,
                                    },
                                },
                                "past": [
                                    {
                                        "period": "2026-08", "effectiveLimit": 300,
                                        "spent": 310, "overspent": 10, "remaining": None,
                                    }
                                ],
                            },
                        }
                    ]
                }
            )
        )
    )
    budget = client.list_budgets().budgets[0]
    assert budget.limit == 300
    assert budget.closed is False
    assert budget.start_date == date(2026, 1, 1)
    assert budget.limit_overrides[0].period_count == 2

    current = budget.spending.current
    assert current.spent == 120.5
    # A period under budget reports no overspend.
    assert current.overspent is None
    assert current.incomplete
    assert current.excluded.total == 3
    assert current.excluded.total_amount_sum == -45.2
    assert current.converted_currencies == ["EUR"]
    assert budget.spending.past[0].remaining is None


def test_closed_date_only_present_when_closed(server, client):
    server.queue(
        MockResponse(
            body='{"budgets":[{"id":"b1","closed":true,"closedDate":"2025-06-15"},'
            '{"id":"b2","closed":false}]}'
        )
    )
    closed, open_budget = client.list_budgets().budgets
    assert closed.closed_date == date(2025, 6, 15)
    assert open_budget.closed_date is None


def test_budget_all_is_readable_but_not_creatable(server, client):
    server.queue(MockResponse(body='{"budgets":[{"id":"b1","type":"BUDGET_ALL"}]}'))
    assert client.list_budgets().budgets[0].type is BudgetType.ALL
    assert BudgetType.ALL not in CREATABLE_BUDGET_TYPES

    with pytest.raises(InvalidRequestError) as excinfo:
        CreateBudgetRequest(
            name="n", currency_code="USD", type=BudgetType.ALL, limit=1
        ).validate()
    assert_problem(excinfo, "type")


@pytest.mark.parametrize(
    ("kwargs", "field"),
    [
        ({"spending": "current+0"}, "spending"),
        ({"spending": "all"}, "spending"),
        ({"spending": "current+x"}, "spending"),
        ({"type": "BUDGET_DAILY"}, "type"),
        ({"category_ids": make_ids(11)}, "categoryId"),
        ({"start_date": [gte("a"), lt("b"), eq("c")]}, "startDate"),
        ({"currency_code": "DOLLARS"}, "currencyCode"),
    ],
)
def test_list_budgets_validation(client, kwargs, field):
    with pytest.raises(InvalidRequestError) as excinfo:
        client.list_budgets(**kwargs)
    assert_problem(excinfo, field)


@pytest.mark.parametrize("value", ["none", "current", "current+1", "current+12", None])
def test_every_spending_depth_is_accepted(server, client, value):
    server.queue(MockResponse(json_body={"budgets": []}))
    client.list_budgets(spending=value)
    assert server.count == 1


def test_create_budget_happy_path(server, client):
    server.queue(
        MockResponse(
            status=201,
            body=json.dumps(
                {
                    "budget": {
                        "id": "b1", "name": "Monthly Food",
                        "type": "BUDGET_INTERVAL_MONTH", "limit": 300,
                    },
                    "summary": {
                        "total": 1, "succeeded": 1, "clientErrors": 0,
                        "serverErrors": 0, "documentsWritten": 1,
                    },
                }
            ),
        )
    )
    response = client.create_budget(
        CreateBudgetRequest(
            name="Monthly Food",
            currency_code="USD",
            type=BudgetType.MONTH,
            limit=300,
            category_ids=["550e8400-e29b-41d4-a716-446655440000"],
        )
    )
    assert response.budget.id == "b1"
    assert server.last.json()["type"] == "BUDGET_INTERVAL_MONTH"


def test_custom_budget_requires_both_dates():
    with pytest.raises(InvalidRequestError) as excinfo:
        CreateBudgetRequest(
            name="n", currency_code="USD", type=BudgetType.CUSTOM, limit=100
        ).validate()
    assert_problem(excinfo, "startDate")
    assert_problem(excinfo, "endDate")

    with pytest.raises(InvalidRequestError) as excinfo:
        CreateBudgetRequest(
            name="n", currency_code="USD", type=BudgetType.CUSTOM, limit=100,
            start_date=date(2026, 1, 1),
        ).validate()
    assert_problem(excinfo, "endDate")

    # A complete custom budget validates.
    CreateBudgetRequest(
        name="n", currency_code="USD", type=BudgetType.CUSTOM, limit=100,
        start_date=date(2026, 1, 1), end_date=date(2026, 6, 30),
    ).validate()


def test_reversed_dates_are_rejected():
    with pytest.raises(InvalidRequestError) as excinfo:
        CreateBudgetRequest(
            name="n", currency_code="USD", type=BudgetType.CUSTOM, limit=100,
            start_date=date(2026, 6, 30), end_date=date(2026, 1, 1),
        ).validate()
    assert_problem(excinfo, "endDate")


@pytest.mark.parametrize(
    ("kwargs", "field"),
    [
        ({"name": "", "currency_code": "USD", "type": BudgetType.WEEK, "limit": 1}, "name"),
        ({"name": long_text(81), "currency_code": "USD", "type": BudgetType.WEEK, "limit": 1}, "name"),
        ({"name": "n", "currency_code": "", "type": BudgetType.WEEK, "limit": 1}, "currencyCode"),
        ({"name": "n", "currency_code": "USD", "type": None, "limit": 1}, "type"),
        ({"name": "n", "currency_code": "USD", "type": BudgetType.WEEK, "limit": 0}, "limit"),
        ({"name": "n", "currency_code": "USD", "type": BudgetType.WEEK, "limit": -5}, "limit"),
        ({"name": "n", "currency_code": "USD", "type": BudgetType.WEEK, "limit": "lots"}, "limit"),
    ],
)
def test_create_budget_validation(kwargs, field):
    with pytest.raises(InvalidRequestError) as excinfo:
        CreateBudgetRequest(**kwargs).validate()
    assert_problem(excinfo, field)


@pytest.mark.parametrize(
    ("override", "field"),
    [
        (LimitOverrideInput(period="", limit=100), "items[0].limitOverrides[0].period"),
        (LimitOverrideInput(period="2026/03", limit=100), "items[0].limitOverrides[0].period"),
        (LimitOverrideInput(period="2026-W99", limit=100), "items[0].limitOverrides[0].period"),
        (LimitOverrideInput(period="2026-13", limit=100), "items[0].limitOverrides[0].period"),
        (LimitOverrideInput(period="2026-03"), "items[0].limitOverrides[0].limit"),
        (
            LimitOverrideInput(period="2026-03", limit=100, set_baseline=True),
            "items[0].limitOverrides[0].limit",
        ),
        (LimitOverrideInput(period="2026-03", limit=0), "items[0].limitOverrides[0].limit"),
        (
            LimitOverrideInput(period="2026-03", set_baseline=True, period_count=2),
            "items[0].limitOverrides[0].periodCount",
        ),
        (
            LimitOverrideInput(period="2026-03", limit=1, period_count=0),
            "items[0].limitOverrides[0].periodCount",
        ),
        (
            LimitOverrideInput(period="2026-03", limit=1, period_count=121),
            "items[0].limitOverrides[0].periodCount",
        ),
    ],
)
def test_limit_override_validation(client, override, field):
    with pytest.raises(InvalidRequestError) as excinfo:
        client.patch_budgets([PatchBudgetItem(id="b1", limit_overrides=[override])])
    assert_problem(excinfo, field)


@pytest.mark.parametrize("period", ["2026", "2026-03", "2026-W01", "2026-W53", "2026-w07"])
def test_every_period_shape_is_accepted(server, client, period):
    server.queue(
        MockResponse(status=200, body=ok_batch({"inputIndex": 0, "id": "b1", "success": True}))
    )
    client.patch_budgets(
        [PatchBudgetItem(id="b1", limit_overrides=[LimitOverrideInput(period=period, limit=100)])]
    )
    assert server.count == 1


@pytest.mark.parametrize("period", ["26-03", "2026-13", "2026-W0", "2026-W54", "202603", "2026-Www"])
def test_bad_period_shapes_are_rejected(client, period):
    with pytest.raises(InvalidRequestError):
        client.patch_budgets(
            [PatchBudgetItem(id="b1", limit_overrides=[LimitOverrideInput(period=period, limit=1)])]
        )


@pytest.mark.parametrize(
    ("items", "field"),
    [
        ([], "items"),
        ([PatchBudgetItem(id="", name="x")], "items[0].id"),
        ([PatchBudgetItem(id="b1")], "items[0]"),
        (
            [
                PatchBudgetItem(
                    id="b1",
                    limit_overrides=[LimitOverrideInput(period="2026-03", limit=1) for _ in range(51)],
                )
            ],
            "items[0].limitOverrides",
        ),
        ([PatchBudgetItem(id="b1", reset_limit=0)], "items[0].resetLimit"),
        ([PatchBudgetItem(id="b1", category_ids=add_remove_ids())], "items[0].categoryIds"),
        (
            [
                PatchBudgetItem(
                    id="b1",
                    limit_overrides=[
                        LimitOverrideInput(period="2026-03", limit=1),
                        LimitOverrideInput(period="2026-03", limit=2),
                    ],
                )
            ],
            "items[0].limitOverrides[1].period",
        ),
        (
            [PatchBudgetItem(id="b1", name="x"), PatchBudgetItem(id="b1", name="y")],
            "items[1].id",
        ),
    ],
)
def test_patch_budgets_validation(client, items, field):
    with pytest.raises(InvalidRequestError) as excinfo:
        client.patch_budgets(items)
    assert_problem(excinfo, field)


def test_patch_budget_sends_both_id_set_shapes(server, client):
    server.queue(
        MockResponse(status=200, body=ok_batch({"inputIndex": 0, "id": "b1", "success": True}))
    )
    client.patch_budgets(
        [
            PatchBudgetItem(
                id="b1",
                category_ids=replace_ids(["c1", "c2"]),
                account_ids=add_remove_ids(add=["acc1"], remove=["acc2"]),
                label_ids=clear_ids(),
                closed=True,
                reset_limit=500,
                limit_overrides=[
                    LimitOverrideInput(period="2026-10", limit=700, period_count=2),
                    LimitOverrideInput(period="2026-12", set_baseline=True),
                ],
            )
        ]
    )
    sent = server.last.json()[0]
    assert sent["categoryIds"] == ["c1", "c2"]
    assert sent["accountIds"] == {"add": ["acc1"], "remove": ["acc2"]}
    assert sent["labelIds"] == []
    assert sent["closed"] is True
    assert sent["resetLimit"] == 500
    assert sent["limitOverrides"] == [
        {"period": "2026-10", "limit": 700, "periodCount": 2},
        {"period": "2026-12", "setBaseline": True},
    ]
