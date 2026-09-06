"""Filters, sorting, enums, conversion helpers and summary arithmetic."""

from __future__ import annotations

from datetime import date, datetime, timezone

import pytest

from wallet_sdk import (
    AccountType,
    BatchSummary,
    BudgetType,
    Cardinality,
    ClearableRecordField,
    EntityType,
    ErrorType,
    Filter,
    FilterOp,
    GoalState,
    Granularity,
    ItemResult,
    PairingMode,
    Pagination,
    RecordSource,
    RecordState,
    RecordType,
    Reminder,
    ResettableCategoryField,
    ResponseMeta,
    Sort,
    UniversalColor,
    ValidationMode,
    amount_filter,
    asc,
    between,
    between_dates,
    contains,
    contains_i,
    date_filter,
    desc,
    eq,
    format_amount,
    format_date,
    format_time,
    gt,
    gte,
    lt,
    lte,
    on_date,
    parse_filter,
    time_filter,
)
from wallet_sdk._convert import (
    enum_or_raw,
    format_date_value,
    format_datetime,
    parse_date,
    parse_datetime,
)
from wallet_sdk.enums import (
    CREATABLE_ACCOUNT_TYPES,
    CREATABLE_BUDGET_TYPES,
    WRITABLE_RECORD_STATES,
)
from wallet_sdk.models import (
    EntityReferences,
    MirrorRecordEmbed,
    ReferenceResult,
    StandingOrderItem,
    TransferOutput,
)


# ---------------------------------------------------------------------------
# Timestamp decoding
# ---------------------------------------------------------------------------


@pytest.mark.parametrize(
    ("raw", "expected"),
    [
        ("2021-03-24T09:28:41.200Z", datetime(2021, 3, 24, 9, 28, 41, 200000, tzinfo=timezone.utc)),
        ("2021-03-24T09:28:41Z", datetime(2021, 3, 24, 9, 28, 41, tzinfo=timezone.utc)),
        ("2021-03-24T09:28:41", datetime(2021, 3, 24, 9, 28, 41, tzinfo=timezone.utc)),
        ("2021-03-24", datetime(2021, 3, 24, tzinfo=timezone.utc)),
        ("2021-03-24T09:28:41.123456Z", datetime(2021, 3, 24, 9, 28, 41, 123456, tzinfo=timezone.utc)),
    ],
)
def test_parse_datetime_accepts_every_format_the_api_emits(raw, expected):
    assert parse_datetime(raw) == expected


def test_parse_datetime_normalises_offsets_to_utc():
    parsed = parse_datetime("2021-03-24T09:28:41+02:00")
    assert parsed == datetime(2021, 3, 24, 7, 28, 41, tzinfo=timezone.utc)


@pytest.mark.parametrize("raw", [None, "", "   ", "yesterday", "24/03/2021", 12345, True, {"t": 1}])
def test_parse_datetime_returns_none_for_anything_unusable(raw):
    assert parse_datetime(raw) is None


def test_parse_datetime_passes_through_aware_and_naive_datetimes():
    aware = datetime(2024, 1, 1, tzinfo=timezone.utc)
    assert parse_datetime(aware) is aware
    naive = datetime(2024, 1, 1)
    assert parse_datetime(naive) == aware
    assert parse_datetime(date(2024, 1, 1)) == aware


@pytest.mark.parametrize(
    ("raw", "expected"),
    [
        ("2025-06-15", date(2025, 6, 15)),
        ("2025-06-15T23:45:00Z", date(2025, 6, 15)),
        (datetime(2025, 6, 15, 23, 45, tzinfo=timezone.utc), date(2025, 6, 15)),
        (date(2025, 6, 15), date(2025, 6, 15)),
        (None, None),
        ("nonsense", None),
    ],
)
def test_parse_date_keeps_only_the_calendar_day(raw, expected):
    assert parse_date(raw) == expected


def test_format_datetime_is_canonical_utc_milliseconds():
    moment = datetime(2021, 3, 24, 9, 28, 41, 200000, tzinfo=timezone.utc)
    assert format_datetime(moment) == "2021-03-24T09:28:41.200Z"
    # A naive datetime is read as UTC.
    assert format_datetime(datetime(2021, 3, 24, 9, 28, 41, 200000)) == "2021-03-24T09:28:41.200Z"
    assert format_datetime(None) is None
    assert format_datetime("already a string") == "already a string"
    assert format_datetime(date(2021, 3, 24)) == "2021-03-24T00:00:00.000Z"


def test_format_datetime_rejects_an_unencodable_type():
    with pytest.raises(TypeError):
        format_datetime(42)


def test_format_date_value():
    assert format_date_value(date(2026, 1, 1)) == "2026-01-01"
    assert format_date_value(datetime(2026, 1, 1, 23, tzinfo=timezone.utc)) == "2026-01-01"
    assert format_date_value("2026-01-01") == "2026-01-01"
    assert format_date_value(None) is None
    with pytest.raises(TypeError):
        format_date_value(42)


# ---------------------------------------------------------------------------
# Filters
# ---------------------------------------------------------------------------


@pytest.mark.parametrize(
    ("built", "wire"),
    [
        (eq("x"), "eq.x"),
        (gt("x"), "gt.x"),
        (gte("x"), "gte.x"),
        (lt("x"), "lt.x"),
        (lte("x"), "lte.x"),
        (contains("x"), "contains.x"),
        (contains_i("x"), "contains-i.x"),
        (eq(100.5), "eq.100.5"),
        (gte(7), "gte.7"),
    ],
)
def test_filter_wire_forms(built, wire):
    assert str(built) == wire


@pytest.mark.parametrize(
    ("op", "is_range", "is_text"),
    [
        (FilterOp.EQ, True, True),
        (FilterOp.GT, True, False),
        (FilterOp.GTE, True, False),
        (FilterOp.LT, True, False),
        (FilterOp.LTE, True, False),
        (FilterOp.CONTAINS, False, True),
        (FilterOp.CONTAINS_I, False, True),
    ],
)
def test_filter_operator_classification(op, is_range, is_text):
    assert op.is_range is is_range
    assert op.is_text is is_text


@pytest.mark.parametrize(
    "wire",
    ["eq.x", "gt.1", "gte.2024-01-01", "lt.2024-02-01", "lte.9", "contains.hello world", "contains-i.HELLO"],
)
def test_parse_filter_round_trips(wire):
    assert str(parse_filter(wire)) == wire


def test_parse_filter_prefers_the_longest_prefix():
    # A naive split on the first dot would mangle a dotted value.
    parsed = parse_filter("contains-i.a.b.c")
    assert parsed.op is FilterOp.CONTAINS_I
    assert parsed.value == "a.b.c"

    parsed = parse_filter("gte.2024-01-01T00:00:00Z")
    assert parsed.op is FilterOp.GTE
    assert parsed.value == "2024-01-01T00:00:00Z"


@pytest.mark.parametrize("wire", ["bare", "unknown.x", "", "2024-01-01"])
def test_parse_filter_rejects_a_missing_prefix(wire):
    with pytest.raises(ValueError):
        parse_filter(wire)


def test_filter_equality_and_hashing():
    assert eq("x") == eq("x")
    assert eq("x") != gt("x")
    assert eq("x") != "eq.x"
    assert len({eq("x"), eq("x"), gt("x")}) == 2


def test_format_helpers():
    moment = datetime(2024, 3, 15, 14, 30, tzinfo=timezone.utc)
    assert format_time(moment) == "2024-03-15T14:30:00Z"
    assert format_date(moment) == "2024-03-15"
    assert format_time(date(2024, 3, 15)) == "2024-03-15T00:00:00Z"
    assert format_time("passed through") == "passed through"
    assert format_date("passed through") == "passed through"


def test_between_builds_a_half_open_range():
    start = datetime(2024, 1, 1, tzinfo=timezone.utc)
    end = datetime(2024, 4, 1, tzinfo=timezone.utc)
    assert [str(f) for f in between(start, end)] == [
        "gte.2024-01-01T00:00:00Z",
        "lt.2024-04-01T00:00:00Z",
    ]
    assert [str(f) for f in between_dates(start, end)] == ["gte.2024-01-01", "lt.2024-04-01"]
    # on_date uses eq., which the API expands to the whole UTC day.
    assert [str(f) for f in on_date(start)] == ["eq.2024-01-01"]


def test_typed_filter_builders():
    moment = datetime(2024, 1, 1, tzinfo=timezone.utc)
    assert str(time_filter(FilterOp.GTE, moment)) == "gte.2024-01-01T00:00:00Z"
    assert str(date_filter(FilterOp.LT, moment)) == "lt.2024-01-01"
    assert str(amount_filter(FilterOp.LTE, -100.50)) == "lte.-100.5"
    assert format_amount("verbatim") == "verbatim"


# ---------------------------------------------------------------------------
# Sorting
# ---------------------------------------------------------------------------


def test_sort_wire_forms():
    assert str(asc("recordDate")) == "+recordDate"
    assert str(desc("amount")) == "-amount"
    assert asc("a") == Sort("a")
    assert asc("a") != desc("a")
    assert asc("a") != "not a sort"
    assert len({asc("a"), asc("a"), desc("a")}) == 2


# ---------------------------------------------------------------------------
# Enums
# ---------------------------------------------------------------------------


@pytest.mark.parametrize(
    "enum_cls",
    [
        AccountType, UniversalColor, Cardinality, BudgetType, RecordState, RecordType,
        RecordSource, PairingMode, GoalState, Reminder, ErrorType, Granularity,
        EntityType, ValidationMode, ClearableRecordField, ResettableCategoryField, FilterOp,
    ],
)
def test_every_enum_member_validates(enum_cls):
    for member in enum_cls:
        assert enum_cls.is_valid(member)
        assert enum_cls.is_valid(member.value)
        # A member compares equal to its wire form and renders as it.
        assert member == member.value
        assert str(member) == member.value


@pytest.mark.parametrize(
    ("enum_cls", "value"),
    [
        (AccountType, "Crypto"),
        (UniversalColor, "Beige"),
        (Cardinality, "maybe"),
        (RecordSource, "smartwatch"),
        (ClearableRecordField, "amount"),
        (ResettableCategoryField, "parentId"),
    ],
)
def test_unknown_enum_values_are_invalid(enum_cls, value):
    assert not enum_cls.is_valid(value)


def test_enum_or_raw_keeps_unknown_values():
    assert enum_or_raw(RecordSource, "rest") is RecordSource.REST
    assert enum_or_raw(RecordSource, "smartwatch") == "smartwatch"
    assert enum_or_raw(RecordSource, None) is None
    assert enum_or_raw(RecordSource, 42) == 42


def test_creatable_subsets_are_strictly_smaller():
    assert set(CREATABLE_ACCOUNT_TYPES) < set(AccountType)
    assert set(CREATABLE_BUDGET_TYPES) < set(BudgetType)
    assert set(WRITABLE_RECORD_STATES) < set(RecordState)
    assert BudgetType.ALL not in CREATABLE_BUDGET_TYPES
    assert RecordState.VOID not in WRITABLE_RECORD_STATES
    assert RecordState.WAIT_FOR_ASSIGN not in WRITABLE_RECORD_STATES


# ---------------------------------------------------------------------------
# Summaries and model helpers
# ---------------------------------------------------------------------------


@pytest.mark.parametrize(
    ("summary", "failed", "all_ok", "all_failed", "partial"),
    [
        (BatchSummary(total=3, succeeded=3), 0, True, False, False),
        (BatchSummary(total=3, client_errors=3), 3, False, True, False),
        (BatchSummary(total=2, server_errors=2), 2, False, True, False),
        (BatchSummary(total=3, succeeded=1, client_errors=1, server_errors=1), 2, False, False, True),
        (BatchSummary(), 0, False, False, False),
    ],
)
def test_batch_summary_helpers(summary, failed, all_ok, all_failed, partial):
    assert summary.failed == failed
    assert summary.all_succeeded is all_ok
    assert summary.all_failed is all_failed
    assert summary.partial is partial


def test_batch_summary_tolerates_a_missing_payload():
    assert BatchSummary.from_dict(None) == BatchSummary()
    assert BatchSummary.from_dict("not an object") == BatchSummary()


def test_item_error_classification():
    assert ItemResult(error_type=ErrorType.CLIENT).is_client_error
    assert not ItemResult(error_type=ErrorType.CLIENT).is_server_error
    assert ItemResult(error_type=ErrorType.SERVER).is_server_error
    # A successful item has no classification.
    assert not ItemResult().is_client_error
    assert not ItemResult().is_server_error


def test_pagination_has_more():
    assert Pagination(next_offset=30).has_more
    assert not Pagination().has_more


def test_transfer_output_paired():
    assert TransferOutput(transfer_id="t1", mirror_record=MirrorRecordEmbed(id="r2")).paired
    assert not TransferOutput(transfer_id="t1").paired
    # A severed pair keeps its transferId but has no usable mirror.
    assert not TransferOutput(transfer_id="t1", mirror_record=MirrorRecordEmbed()).paired


def test_standing_order_item_paid():
    assert StandingOrderItem(paid_date=datetime.now(timezone.utc)).paid
    assert StandingOrderItem(record_ids=["r1"]).paid
    assert not StandingOrderItem().paid


def test_entity_references_totals():
    entry = EntityReferences(
        records=ReferenceResult(total=3),
        budgets=ReferenceResult(total=1),
        standing_orders=ReferenceResult(total=0),
        other=ReferenceResult(total=2),
    )
    assert entry.total_references == 6
    assert entry.resolved
    assert set(entry.referencing_kinds()) == {"records", "budgets", "other"}

    unresolved = EntityReferences(error="not found")
    assert not unresolved.resolved
    assert unresolved.total_references == 0
    assert unresolved.referencing_kinds() == []


def test_response_meta_defaults():
    meta = ResponseMeta()
    assert meta.status_code == 0
    assert not meta.rate_limit_exhausted
    assert meta.headers == {}


@pytest.mark.parametrize(
    ("value", "expected"),
    [
        (1e20, "100000000000000000000"),
        (1e-7, "0.0000001"),
        (0.0, "0"),
        (-0.5, "-0.5"),
        (1e16, "10000000000000000"),
    ],
)
def test_format_amount_never_uses_exponent_notation(value, expected):
    # The amount filter reads decimals; "1e+20" would be rejected by the API.
    rendered = format_amount(value)
    assert "e" not in rendered.lower()
    assert rendered == expected
