"""Type-confusion and boundary cases the happy-path tests never reach.

The API is fed by real callers, and a real caller eventually passes a string
where a list belongs, or a Filter built the wrong way. Each of these must
produce a precise problem rather than a TypeError from deep inside the SDK.
"""

from __future__ import annotations

import pytest
from conftest import MockResponse, assert_problem

from wallet_sdk import (
    AmountInput,
    CreateBudgetRequest,
    CreateRecordRequest,
    Filter,
    FilterOp,
    InvalidRequestError,
    PatchRecordItem,
    Sort,
    asc,
    eq,
    gte,
    replace_ids,
)
from wallet_sdk._problems import Problems
from wallet_sdk.filters import (
    validate_range_filters,
    validate_sort,
    validate_text_filter,
)
from wallet_sdk.validation import (
    check_amount_value,
    check_currency_code,
    check_entity_type,
    check_id_list,
    check_limit,
    check_max_length,
    check_offset,
    check_period_format,
    check_spending_depth,
    check_usage_period,
)


def problems_from(fn, *args, **kwargs):
    """Run a checker and return the problems it recorded."""
    problems = Problems()
    fn(problems, *args, **kwargs)
    return problems.items


# ---------------------------------------------------------------------------
# Type confusion in scalars
# ---------------------------------------------------------------------------


@pytest.mark.parametrize("value", ["50", 3.5, True, [50]])
def test_limit_must_be_an_integer(client, value):
    with pytest.raises(InvalidRequestError) as excinfo:
        client.list_accounts(limit=value)
    assert_problem(excinfo, "limit")


@pytest.mark.parametrize("value", ["0", 1.5, True])
def test_offset_must_be_an_integer(client, value):
    with pytest.raises(InvalidRequestError) as excinfo:
        client.list_accounts(offset=value)
    assert_problem(excinfo, "offset")


def test_a_comma_joined_id_string_is_rejected(client):
    # Passing "a,b" would silently become the single id "a,b" once joined.
    with pytest.raises(InvalidRequestError) as excinfo:
        client.list_accounts(ids="a,b")
    assert_problem(excinfo, "id")


def test_a_non_string_id_is_rejected(client):
    with pytest.raises(InvalidRequestError) as excinfo:
        client.list_accounts(ids=["a", 42])
    assert_problem(excinfo, "id[1]")


def test_a_non_string_name_is_rejected():
    assert problems_from(check_max_length, "name", 42, 80)


@pytest.mark.parametrize("value", [None, "", "US", "USDX", "U5D", "us1", 42])
def test_currency_code_shape(value):
    assert problems_from(check_currency_code, "currencyCode", value)


@pytest.mark.parametrize("value", ["USD", "eur", "Czk"])
def test_currency_code_accepts_any_case(value):
    assert not problems_from(check_currency_code, "currencyCode", value)


@pytest.mark.parametrize("value", [None, "5", True, [5]])
def test_amount_value_must_be_a_number(value):
    assert problems_from(check_amount_value, "value", value)


def test_amount_value_accepts_an_integer():
    assert not problems_from(check_amount_value, "value", -5)


# ---------------------------------------------------------------------------
# Filters built the wrong way
# ---------------------------------------------------------------------------


def test_a_bare_string_is_not_a_filter(client):
    with pytest.raises(InvalidRequestError) as excinfo:
        client.list_accounts(name="checking")
    assert_problem(excinfo, "name")


def test_a_bare_string_in_a_range_list_is_rejected(client):
    with pytest.raises(InvalidRequestError) as excinfo:
        client.list_accounts(created_at=["gte.2024-01-01"])
    assert_problem(excinfo, "createdAt[0]")


def test_an_empty_range_value_is_rejected():
    assert problems_from(validate_range_filters, "createdAt", [Filter(FilterOp.GTE, "   ")])


def test_an_empty_text_value_is_rejected():
    assert problems_from(validate_text_filter, "name", Filter(FilterOp.CONTAINS, ""))


def test_a_bare_string_is_not_a_sort(client):
    with pytest.raises(InvalidRequestError) as excinfo:
        client.list_accounts(sort_by="-createdAt")
    assert_problem(excinfo, "sortBy")


def test_an_empty_sort_field_is_rejected():
    assert problems_from(validate_sort, "sortBy", Sort(""), ("createdAt",))


def test_a_none_sort_is_simply_unset():
    assert not problems_from(validate_sort, "sortBy", None, ("createdAt",))


def test_none_filters_are_simply_unset():
    assert not problems_from(validate_range_filters, "createdAt", None)
    assert not problems_from(validate_range_filters, "createdAt", [])
    assert not problems_from(validate_text_filter, "name", None)


# ---------------------------------------------------------------------------
# Period and usage-period parsing
# ---------------------------------------------------------------------------


@pytest.mark.parametrize("period", ["", None, 2026, "2026-1", "2026-00", "2026-W1", "20-26"])
def test_bad_budget_periods(period):
    assert problems_from(check_period_format, "period", period)


@pytest.mark.parametrize("period", ["2026", "2026-01", "2026-12", "2026-W01", "2026-W53"])
def test_good_budget_periods(period):
    assert not problems_from(check_period_format, "period", period)


@pytest.mark.parametrize("period", [None, "", 30, "days", "xdays"])
def test_bad_usage_periods(period):
    assert problems_from(check_usage_period, "period", period)


@pytest.mark.parametrize("value", ["current+", "current+-1", "everything", 5])
def test_bad_spending_depths(value):
    assert problems_from(check_spending_depth, "spending", value)


# ---------------------------------------------------------------------------
# Entity types
# ---------------------------------------------------------------------------


def test_entity_type_must_be_supplied():
    from wallet_sdk.enums import DELETABLE_ENTITY_TYPES

    assert problems_from(check_entity_type, "type", None, DELETABLE_ENTITY_TYPES)
    assert problems_from(check_entity_type, "type", "", DELETABLE_ENTITY_TYPES)


# ---------------------------------------------------------------------------
# Batch and nested-body edges
# ---------------------------------------------------------------------------


def test_id_list_accepts_exactly_the_maximum():
    assert not problems_from(check_id_list, "id", [f"id-{n}" for n in range(30)], 30)


def test_id_list_none_is_unset():
    assert not problems_from(check_id_list, "id", None, 30)


def test_a_record_without_an_amount_is_rejected(client):
    record = CreateRecordRequest(account_id="a", amount=None, record_date="2026-09-05T00:00:00Z")
    with pytest.raises(InvalidRequestError) as excinfo:
        client.create_records([record])
    assert_problem(excinfo, "records[0].amount")


def test_a_transfer_on_a_record_with_no_amount_still_validates_its_own_fields(client):
    from wallet_sdk import PairingMode, TransferInput

    record = CreateRecordRequest(
        account_id="a",
        amount=None,
        record_date="2026-09-05T00:00:00Z",
        transfer=TransferInput(pairing_mode=PairingMode.NEW),
    )
    with pytest.raises(InvalidRequestError) as excinfo:
        client.create_records([record])
    assert_problem(excinfo, "records[0].transfer.accountId")


def test_budget_limit_must_be_a_number():
    with pytest.raises(InvalidRequestError) as excinfo:
        CreateBudgetRequest(name="n", currency_code="USD", type="BUDGET_INTERVAL_WEEK", limit=None).validate()
    assert_problem(excinfo, "limit")


def test_patch_item_with_only_a_clear_is_a_real_update(client, server):
    from conftest import ok_batch
    from wallet_sdk import ClearableRecordField

    server.queue(
        MockResponse(status=200, body=ok_batch({"inputIndex": 0, "id": "r1", "success": True}))
    )
    client.patch_records([PatchRecordItem(id="r1", clear=[ClearableRecordField.NOTE])])
    assert server.count == 1


def test_an_empty_id_set_delta_is_rejected_on_every_field(client):
    from wallet_sdk import PatchBudgetItem, add_remove_ids

    for field in ("account_ids", "category_ids", "label_ids"):
        with pytest.raises(InvalidRequestError) as excinfo:
            client.patch_budgets([PatchBudgetItem(id="b1", **{field: add_remove_ids()})])
        camel = {"account_ids": "accountIds", "category_ids": "categoryIds", "label_ids": "labelIds"}[field]
        assert_problem(excinfo, f"items[0].{camel}")


def test_an_id_set_with_too_many_values_is_rejected(client):
    from wallet_sdk import PatchBudgetItem

    with pytest.raises(InvalidRequestError) as excinfo:
        client.patch_budgets(
            [PatchBudgetItem(id="b1", category_ids=replace_ids([f"c{n}" for n in range(31)]))]
        )
    assert_problem(excinfo, "items[0].categoryIds")


def test_an_id_set_delta_with_too_many_values_is_rejected(client):
    from wallet_sdk import PatchBudgetItem, add_remove_ids

    with pytest.raises(InvalidRequestError) as excinfo:
        client.patch_budgets(
            [PatchBudgetItem(id="b1", label_ids=add_remove_ids(add=[f"l{n}" for n in range(31)]))]
        )
    assert_problem(excinfo, "items[0].labelIds.add")
