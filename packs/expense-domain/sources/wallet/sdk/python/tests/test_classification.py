"""Categories, labels and record rules."""

from __future__ import annotations

import json

import pytest
from conftest import MockResponse, assert_problem, long_text, ok_batch

from wallet_sdk import (
    CATEGORY_TRANSFER,
    CATEGORY_UNCATEGORIZED,
    CATEGORY_UNKNOWN_EXPENSE,
    CATEGORY_UNKNOWN_INCOME,
    Cardinality,
    CreateLabelRequest,
    CreateSubcategoryRequest,
    InvalidRequestError,
    PatchCategoryItem,
    PatchLabelItem,
    ResettableCategoryField,
    UniversalColor,
    asc,
    contains,
    contains_i,
    desc,
    eq,
    gte,
    lt,
    lte,
)


# ---------------------------------------------------------------------------
# Categories
# ---------------------------------------------------------------------------


def test_list_categories_serialises_every_parameter(server, client):
    server.queue(MockResponse(json_body={"categories": []}))
    client.list_categories(
        limit=100,
        offset=0,
        agent_hints=False,
        with_total=True,
        ids=["c1"],
        name=eq("Groceries"),
        created_at=[gte("2024-01-01T00:00:00Z")],
        updated_at=[lt("2025-01-01T00:00:00Z")],
        custom_category=True,
        cardinality=Cardinality.MUST,
        archived=False,
        budget_id="b1",
        sort_by=asc("updatedAt"),
    )
    request = server.last
    expected = {
        "limit": "100", "offset": "0", "agentHints": "false", "withTotal": "true",
        "id": "c1", "name": "eq.Groceries", "customCategory": "true",
        "cardinality": "must", "archived": "false", "budgetId": "b1", "sortBy": "+updatedAt",
    }
    for key, value in expected.items():
        assert request.q(key) == value, key


def test_list_categories_decodes_system_and_custom_shapes(server, client):
    server.queue(
        MockResponse(
            body=json.dumps(
                {
                    "categories": [
                        {
                            "id": "5c5c07d0-0014-8000-8000-000000000000",
                            "name": "Groceries",
                            "systemId": "food_and_drinks__groceries",
                            "customCategory": False,
                            "customName": False,
                            "enabled": True,
                            "color": "#26c6da",
                            "cardinality": "need",
                            "group": {"id": "food_and_drinks", "name": "Food & Drinks"},
                            "createdAt": None,
                            "updatedAt": None,
                        },
                        {
                            "id": "custom-1",
                            "name": "Weekend Dining",
                            "customCategory": True,
                            "customName": True,
                            "parentId": "5c5c07d0-0014-8000-8000-000000000000",
                            "parentName": "Groceries",
                            "cardinality": "want",
                            "archived": True,
                            "createdAt": "2024-05-01T10:00:00Z",
                            "updatedAt": "2024-06-01T10:00:00Z",
                        },
                    ]
                }
            )
        )
    )
    system, custom = client.list_categories().categories
    assert system.system_id == "food_and_drinks__groceries"
    assert not system.custom_category
    assert system.group.name == "Food & Drinks"
    # A system category's timestamps are nullable and decode to None.
    assert system.created_at is None and system.updated_at is None

    assert custom.parent_id and custom.parent_name == "Groceries"
    assert custom.archived
    assert custom.cardinality is Cardinality.WANT


def test_fixed_category_ids_match_the_registry():
    assert CATEGORY_UNKNOWN_INCOME == "5c5c32c8-0082-8000-8000-000000000000"
    assert CATEGORY_UNKNOWN_EXPENSE == "5c5c32c9-0082-8000-8000-000000000000"
    assert CATEGORY_UNCATEGORIZED == "5c5c4e23-00c8-8000-8000-000000000000"
    assert CATEGORY_TRANSFER == "5c5c4e21-00c8-8000-8000-000000000000"


def test_create_subcategory_happy_path(server, client):
    server.queue(
        MockResponse(
            status=201,
            body=json.dumps(
                {
                    "category": {
                        "id": "c9", "name": "Weekend Dining",
                        "customCategory": True, "parentId": "p1",
                    },
                    "summary": {
                        "total": 1, "succeeded": 1, "clientErrors": 0,
                        "serverErrors": 0, "documentsWritten": 1,
                    },
                }
            ),
        )
    )
    response = client.create_subcategory(
        CreateSubcategoryRequest(
            name="Weekend Dining",
            parent_id="p1",
            cardinality=Cardinality.WANT,
            color=UniversalColor.RED,
        )
    )
    assert response.category.custom_category
    assert server.last.path == "/v1/api/categories/custom"
    sent = server.last.json()
    assert sent["cardinality"] == "want"
    assert sent["color"] == "Red"


@pytest.mark.parametrize(
    ("kwargs", "field"),
    [
        ({"name": "", "parent_id": "p1"}, "name"),
        ({"name": long_text(81), "parent_id": "p1"}, "name"),
        ({"name": "n", "parent_id": ""}, "parentId"),
        ({"name": "n", "parent_id": "p", "cardinality": "maybe"}, "cardinality"),
        ({"name": "n", "parent_id": "p", "color": "Beige"}, "color"),
    ],
)
def test_create_subcategory_validation(kwargs, field):
    with pytest.raises(InvalidRequestError) as excinfo:
        CreateSubcategoryRequest(**kwargs).validate()
    assert_problem(excinfo, field)


@pytest.mark.parametrize(
    "item",
    [
        PatchCategoryItem(id="c1", name="x", reset=[ResettableCategoryField.NAME]),
        PatchCategoryItem(
            id="c1", cardinality=Cardinality.MUST, reset=[ResettableCategoryField.CARDINALITY]
        ),
        PatchCategoryItem(id="c1", color=UniversalColor.RED, reset=[ResettableCategoryField.COLOR]),
    ],
)
def test_reset_and_set_conflict(client, item):
    with pytest.raises(InvalidRequestError) as excinfo:
        client.patch_categories([item])
    assert_problem(excinfo, "items[0].reset[0]")


@pytest.mark.parametrize(
    ("items", "field"),
    [
        ([], "items"),
        ([PatchCategoryItem(id=f"id-{n}", name="x") for n in range(11)], "items"),
        ([PatchCategoryItem(id="", name="x")], "items[0].id"),
        ([PatchCategoryItem(id="c1")], "items[0]"),
        ([PatchCategoryItem(id="c1", reset=["parentId"])], "items[0].reset[0]"),
        (
            [
                PatchCategoryItem(
                    id="c1",
                    reset=[ResettableCategoryField.NAME, ResettableCategoryField.NAME],
                )
            ],
            "items[0].reset[1]",
        ),
        ([PatchCategoryItem(id="c1", cardinality="maybe")], "items[0].cardinality"),
    ],
)
def test_patch_categories_validation(client, items, field):
    with pytest.raises(InvalidRequestError) as excinfo:
        client.patch_categories(items)
    assert_problem(excinfo, field)


def test_patch_categories_sends_the_reset_array(server, client):
    server.queue(
        MockResponse(status=200, body=ok_batch({"inputIndex": 0, "id": "c1", "success": True}))
    )
    client.patch_categories(
        [PatchCategoryItem(id="c1", reset=[ResettableCategoryField.NAME, ResettableCategoryField.COLOR])],
        return_data=True,
    )
    assert server.last.json()[0]["reset"] == ["name", "color"]
    assert server.last.q("returnData") == "true"


def test_cardinality_none_clears_rather_than_failing(server, client):
    server.queue(
        MockResponse(status=200, body=ok_batch({"inputIndex": 0, "id": "c1", "success": True}))
    )
    client.patch_categories([PatchCategoryItem(id="c1", cardinality=Cardinality.NONE)])
    assert server.last.json()[0]["cardinality"] == "none"


# ---------------------------------------------------------------------------
# Labels
# ---------------------------------------------------------------------------


def test_list_labels_serialises_every_parameter(server, client):
    server.queue(MockResponse(json_body={"labels": []}))
    client.list_labels(
        limit=20,
        ids=["l1"],
        name=contains_i("vac"),
        created_at=[gte("2024-01-01T00:00:00Z")],
        updated_at=[lte("2025-01-01T00:00:00Z")],
        archived=True,
        record_id="r1",
        budget_id="b1",
        standing_order_id="so1",
        record_rule_id="rr1",
        sort_by=desc("updatedAt"),
    )
    request = server.last
    expected = {
        "limit": "20", "id": "l1", "name": "contains-i.vac", "archived": "true",
        "recordId": "r1", "budgetId": "b1", "standingOrderId": "so1",
        "recordRuleId": "rr1", "sortBy": "-updatedAt",
    }
    for key, value in expected.items():
        assert request.q(key) == value, key


def test_create_label_happy_path(server, client):
    server.queue(
        MockResponse(
            status=201,
            body=json.dumps(
                {
                    "label": {"id": "l1", "name": "vacation", "color": "#d32f2f", "archived": False},
                    "summary": {
                        "total": 1, "succeeded": 1, "clientErrors": 0,
                        "serverErrors": 0, "documentsWritten": 1,
                    },
                }
            ),
        )
    )
    response = client.create_label(CreateLabelRequest(name="vacation", color=UniversalColor.BLUE))
    assert response.label.id == "l1"
    # The API answers with hex colours even though writes take names.
    assert response.label.color == "#d32f2f"
    assert server.last.json() == {"name": "vacation", "color": "Blue"}


@pytest.mark.parametrize(
    ("kwargs", "field"),
    [
        ({"name": ""}, "name"),
        ({"name": "   "}, "name"),
        ({"name": long_text(81)}, "name"),
        ({"name": "n", "color": "Beige"}, "color"),
    ],
)
def test_create_label_validation(kwargs, field):
    with pytest.raises(InvalidRequestError) as excinfo:
        CreateLabelRequest(**kwargs).validate()
    assert_problem(excinfo, field)


@pytest.mark.parametrize(
    ("items", "field"),
    [
        ([], "items"),
        ([PatchLabelItem(id="", name="x")], "items[0].id"),
        ([PatchLabelItem(id="l1")], "items[0]"),
        ([PatchLabelItem(id="l1", name=long_text(81))], "items[0].name"),
        ([PatchLabelItem(id="l1", color="Beige")], "items[0].color"),
        (
            [PatchLabelItem(id="l1", archived=True), PatchLabelItem(id="l1", archived=False)],
            "items[1].id",
        ),
    ],
)
def test_patch_labels_validation(client, items, field):
    with pytest.raises(InvalidRequestError) as excinfo:
        client.patch_labels(items)
    assert_problem(excinfo, field)


def test_patch_labels_archive(server, client):
    server.queue(
        MockResponse(
            status=200,
            body=ok_batch(
                {
                    "inputIndex": 0, "id": "l1", "success": True,
                    "label": {"id": "l1", "archived": True},
                }
            ),
        )
    )
    response = client.patch_labels([PatchLabelItem(id="l1", archived=True)])
    assert response.results[0].label.archived


# ---------------------------------------------------------------------------
# Record rules
# ---------------------------------------------------------------------------


def test_list_record_rules(server, client):
    server.queue(
        MockResponse(
            body=json.dumps(
                {
                    "recordRules": [
                        {
                            "id": "rr1",
                            "name": "Coffee",
                            "keywords": ["starbucks", "costa"],
                            "category": {"id": "c1", "name": "Coffee"},
                            "labels": [{"id": "l1", "name": "treat"}],
                            "fromAccountId": "acc1",
                            "toAccountId": "acc2",
                            "createdAt": "2024-01-01T00:00:00Z",
                        }
                    ]
                }
            )
        )
    )
    rule = client.list_record_rules(label_id="l1", name=contains("Cof"), limit=5).record_rules[0]
    assert rule.keywords == ["starbucks", "costa"]
    assert rule.category.name == "Coffee"
    assert [label.name for label in rule.labels] == ["treat"]
    assert rule.from_account_id == "acc1" and rule.to_account_id == "acc2"

    request = server.last
    assert request.path == "/v1/api/record-rules"
    assert request.q("labelId") == "l1"
    assert request.q("name") == "contains.Cof"
    assert request.q("limit") == "5"


def test_list_record_rules_validation(client):
    with pytest.raises(InvalidRequestError) as excinfo:
        client.list_record_rules(name=gte("x"))
    assert_problem(excinfo, "name")
