"""Records: filters, transfers, the $clear array, and the default date window."""

from __future__ import annotations

import json
from datetime import datetime, timedelta, timezone

import pytest
from conftest import FIXED_NOW, MockResponse, assert_problem, long_text, make_ids, ok_batch

from wallet_sdk import (
    CATEGORY_UNKNOWN,
    CATEGORY_UNKNOWN_EXPENSE,
    CONVERT_TO_ACCOUNT,
    CONVERT_TO_BASE,
    AmountInput,
    ClearableRecordField,
    CounterAmountInput,
    CreateRecordRequest,
    DecodeError,
    InvalidRequestError,
    PairingMode,
    PatchRecordItem,
    PlaceInput,
    RecordSource,
    RecordState,
    RecordType,
    TransferInput,
    ValidationMode,
    add_remove_ids,
    amount_filter,
    asc,
    between,
    clear_ids,
    contains,
    eq,
    format_amount,
    gte,
    lt,
    lte,
    replace_ids,
)
from wallet_sdk.enums import FilterOp
from wallet_sdk.requests import IdSet


def valid_record(**overrides) -> CreateRecordRequest:
    """A create request that passes validation, for tests to vary one field of."""
    kwargs = dict(
        account_id="acc1",
        amount=AmountInput(value=-42.5),
        record_date=FIXED_NOW - timedelta(hours=1),
    )
    kwargs.update(overrides)
    return CreateRecordRequest(**kwargs)


# ---------------------------------------------------------------------------
# List
# ---------------------------------------------------------------------------


def test_list_records_serialises_every_parameter(server, client):
    server.queue(MockResponse(json_body={"records": []}))
    client.list_records(
        ids=["r1", "r2"],
        account_ids=["acc1", "acc2"],
        record_date=between("2024-01-01T00:00:00Z", "2024-04-01T00:00:00Z"),
        limit=200,
        offset=10,
        agent_hints=True,
        with_total=True,
        category_ids=[CATEGORY_UNKNOWN],
        label_id="lab1",
        note=contains("lunch"),
        counter_party=eq("Downtown Bistro"),
        amount=[amount_filter(FilterOp.LTE, -100.5)],
        created_at=[gte("2024-01-01T00:00:00Z")],
        updated_at=[lt("2024-05-01T00:00:00Z")],
        record_type=RecordType.EXPENSE,
        is_transfer=False,
        transfer_ids=["t1"],
        record_states=[RecordState.RECONCILED, RecordState.UNCLEARED],
        sources=[RecordSource.MCP, RecordSource.REST],
        sort_by=asc("recordDate"),
        convert_to="CZK",
    )
    request = server.last
    expected = {
        "id": "r1,r2",
        "accountId": "acc1,acc2",
        "limit": "200",
        "offset": "10",
        "agentHints": "true",
        "withTotal": "true",
        "categoryId": "unknown",
        "labelId": "lab1",
        "note": "contains.lunch",
        "counterParty": "eq.Downtown Bistro",
        "amount": "lte.-100.5",
        "recordType": "expense",
        "isTransfer": "false",
        "transferId": "t1",
        "recordState": "reconciled,uncleared",
        "source": "mcp,rest",
        "sortBy": "+recordDate",
        "convertTo": "CZK",
    }
    for key, value in expected.items():
        assert request.q(key) == value, key
    assert len(request.qs("recordDate")) == 2


@pytest.mark.parametrize(
    ("value", "expected"),
    [(100.50, "100.5"), (100.00, "100"), (-42.5, "-42.5"), (0.01, "0.01"), (1234.56, "1234.56"), (7, "7")],
)
def test_amount_filter_trims_trailing_zeroes(value, expected):
    assert format_amount(value) == expected


def test_list_records_reports_the_applied_default_window(server, client):
    server.queue(
        MockResponse(
            body=json.dumps(
                {
                    "records": [],
                    "appliedRecordDateFilters": [
                        "gte.2026-06-06T00:00:00Z",
                        "lt.2026-09-06T00:00:00Z",
                    ],
                }
            )
        )
    )
    page = client.list_records()
    assert len(page.applied_record_date_filters) == 2
    assert page.default_window_applied(requested=False)
    assert not page.default_window_applied(requested=True)


def test_id_lookup_bypasses_the_window(server, client):
    server.queue(MockResponse(body='{"records":[{"id":"r1"}],"appliedRecordDateFilters":null}'))
    page = client.list_records(ids=["r1"])
    assert page.applied_record_date_filters is None


def test_list_records_decodes_the_full_record_shape(server, client):
    server.queue(
        MockResponse(
            body=json.dumps(
                {
                    "limit": 30,
                    "offset": 0,
                    "records": [
                        {
                            "id": "550e8400-e29b-41d4-a716-446655440000",
                            "accountId": "-Account_abc123",
                            "accountName": "Checking Account",
                            "accountIsBankSync": False,
                            "amount": {"currencyCode": "USD", "value": -42.5},
                            "convertedAmount": {
                                "currencyCode": "CZK", "value": -1080.35,
                                "ratio": 25.42, "conversionPair": "USD:CZK",
                            },
                            "category": {
                                "id": "cat1", "name": "Health & beauty", "color": "#26c6da",
                                "group": {"id": "food_and_drinks", "name": "Food & Drinks"},
                            },
                            "counterParty": "Downtown Bistro",
                            "labels": [
                                {"id": "l1", "name": "Important", "color": "#d32f2f", "archived": False}
                            ],
                            "note": "Lunch at restaurant",
                            "photos": [
                                {
                                    "createdAt": "2021-03-24T09:28:41.200Z",
                                    "temporaryUrl": "https://example.test/p.jpg",
                                }
                            ],
                            "place": {
                                "address": "1 Main St", "latitude": 50.08,
                                "longitude": 14.42, "name": "Bistro",
                            },
                            "recordDate": "2025-03-15T14:30:00Z",
                            "recordState": "cleared",
                            "recordType": "expense",
                            "source": "rest",
                            "transfer": {
                                "transferId": "tr1",
                                "type": "paired",
                                "mirrorRecord": {
                                    "id": "r2", "accountId": "acc2", "counterParty": "Self",
                                    "note": "mirror",
                                    "amount": {"currencyCode": "USD", "value": 42.5},
                                },
                            },
                            "createdAt": "2021-03-24T09:28:41.200Z",
                            "updatedAt": "2021-03-25T09:28:41.200Z",
                        }
                    ],
                }
            )
        )
    )
    record = client.list_records().records[0]
    assert record.amount.value == -42.5 and record.amount.currency_code == "USD"
    assert record.converted_amount.ok and record.converted_amount.ratio == 25.42
    assert record.category.group.id == "food_and_drinks"
    assert [label.name for label in record.labels] == ["Important"]
    assert record.photos[0].temporary_url == "https://example.test/p.jpg"
    assert record.place.latitude == 50.08
    assert record.is_transfer and record.transfer.paired
    assert record.transfer.mirror_record.amount.value == 42.5
    assert record.source is RecordSource.REST
    assert record.record_state is RecordState.CLEARED
    assert record.record_type is RecordType.EXPENSE


def test_converted_amount_error_is_surfaced(server, client):
    server.queue(
        MockResponse(
            body=json.dumps(
                {
                    "records": [
                        {
                            "id": "r1",
                            "amount": {"value": 1},
                            "convertedAmount": {"currencyCode": "XYZ", "error": "no rate available"},
                        }
                    ]
                }
            )
        )
    )
    converted = client.list_records(convert_to="XYZ").records[0].converted_amount
    assert not converted.ok
    assert converted.error == "no rate available"


@pytest.mark.parametrize("state", [RecordState.VOID, RecordState.WAIT_FOR_ASSIGN])
def test_output_only_states_decode(server, client, state):
    server.queue(MockResponse(body=f'{{"records":[{{"id":"r1","recordState":"{state.value}"}}]}}'))
    assert client.list_records().records[0].record_state is state


def test_unknown_enum_value_still_decodes(server, client):
    # A new server-side enum member must not break an old client.
    server.queue(
        MockResponse(body='{"records":[{"id":"r1","source":"smartwatch","recordState":"quantum"}]}')
    )
    record = client.list_records().records[0]
    assert record.source == "smartwatch"
    assert record.record_state == "quantum"
    assert not RecordSource.is_valid("smartwatch")


@pytest.mark.parametrize(
    ("kwargs", "field"),
    [
        ({"account_ids": make_ids(11)}, "accountId"),
        ({"category_ids": make_ids(11)}, "categoryId"),
        ({"ids": make_ids(31)}, "id"),
        ({"record_date": [gte("a"), lt("b"), eq("c")]}, "recordDate"),
        ({"note": gte("x")}, "note"),
        ({"counter_party": lte("x")}, "counterParty"),
        ({"record_type": "refund"}, "recordType"),
        ({"record_states": [RecordState.VOID]}, "recordState[0]"),
        ({"sources": ["smartwatch"]}, "source[0]"),
        ({"sort_by": asc("note")}, "sortBy"),
        ({"convert_to": "EURO"}, "convertTo"),
        ({"amount": [contains("x")]}, "amount[0]"),
    ],
)
def test_list_records_validation(client, kwargs, field):
    with pytest.raises(InvalidRequestError) as excinfo:
        client.list_records(**kwargs)
    assert_problem(excinfo, field)


@pytest.mark.parametrize("value", [CONVERT_TO_BASE, CONVERT_TO_ACCOUNT, "EUR", "czk"])
def test_convert_to_accepts_magic_values_and_codes(server, client, value):
    server.queue(MockResponse(json_body={"records": []}))
    client.list_records(convert_to=value)
    assert server.last.q("convertTo") == value


# ---------------------------------------------------------------------------
# Create
# ---------------------------------------------------------------------------


def test_create_records_happy_path(server, client):
    server.queue(
        MockResponse(
            status=200,
            body=ok_batch(
                {
                    "inputIndex": 0, "id": "r1", "success": True,
                    "record": {
                        "id": "r1",
                        "amount": {"value": -42.5, "currencyCode": "USD"},
                        "recordType": "expense",
                    },
                }
            ),
        )
    )
    response = client.create_records(
        [
            valid_record(
                category_id=CATEGORY_UNKNOWN_EXPENSE,
                counter_party="Downtown Bistro",
                note="Lunch",
                label_ids=["l1"],
                record_state=RecordState.UNCLEARED,
                place=PlaceInput(name="Bistro", latitude=50.08, longitude=14.42),
            )
        ]
    )
    assert response.summary.all_succeeded
    assert response.results[0].record.id == "r1"

    # The body must be a bare array, as the endpoint expects.
    sent = server.last.json()
    assert isinstance(sent, list) and len(sent) == 1
    assert sent[0]["categoryId"] == CATEGORY_UNKNOWN_EXPENSE
    assert sent[0]["recordDate"] == "2026-09-06T11:00:00.000Z"
    assert sent[0]["amount"] == {"value": -42.5}
    # An unset currencyCode must be omitted so the account's applies.
    assert "currencyCode" not in sent[0]["amount"]


def test_create_record_single_convenience(server, client):
    server.queue(
        MockResponse(status=200, body=ok_batch({"inputIndex": 0, "id": "r1", "success": True}))
    )
    result, response = client.create_record(valid_record())
    assert result.id == "r1" and result.success
    assert response.summary.total == 1


def test_create_record_reports_a_failed_item_without_raising(server, client):
    server.queue(
        MockResponse(
            status=400,
            body=json.dumps(
                {
                    "results": [
                        {
                            "inputIndex": 0, "success": False, "error": "account not found",
                            "errorType": "client_error", "fields": ["accountId"],
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
    result, _ = client.create_record(valid_record())
    assert not result.success and result.is_client_error
    assert result.fields == ["accountId"]


def test_create_record_raises_when_the_server_returns_no_result(server, client):
    server.queue(
        MockResponse(
            status=200,
            body='{"results":[],"summary":{"total":1,"succeeded":0,"clientErrors":0,'
            '"serverErrors":0,"documentsWritten":0}}',
        )
    )
    with pytest.raises(DecodeError):
        client.create_record(valid_record())


def test_transfer_mirror_id_survives_return_data_false(server, client):
    server.queue(
        MockResponse(
            status=200,
            body=json.dumps(
                {
                    "results": [
                        {
                            "inputIndex": 0, "id": "r1", "success": True,
                            "createdMirrorRecordId": "r2",
                        }
                    ],
                    "summary": {
                        "total": 1, "succeeded": 1, "clientErrors": 0,
                        "serverErrors": 0, "documentsWritten": 2,
                    },
                }
            ),
        )
    )
    response = client.create_records(
        [valid_record(transfer=TransferInput(pairing_mode=PairingMode.NEW, account_id="acc2"))],
        return_data=False,
    )
    assert response.results[0].created_mirror_record_id == "r2"
    # documentsWritten exceeding total is expected: the mirror is a document
    # without being an item.
    assert response.summary.documents_written > response.summary.total
    assert server.last.q("returnData") == "false"


def test_create_record_sends_the_transfer_block_verbatim(server, client):
    server.queue(
        MockResponse(status=200, body=ok_batch({"inputIndex": 0, "id": "r1", "success": True}))
    )
    client.create_records(
        [
            valid_record(
                transfer=TransferInput(
                    pairing_mode=PairingMode.NEW,
                    account_id="acc2",
                    counter_amount=CounterAmountInput(value=40, currency_code="EUR"),
                )
            )
        ]
    )
    transfer = server.last.json()[0]["transfer"]
    assert transfer["pairingMode"] == "new"
    assert transfer["accountId"] == "acc2"
    assert transfer["counterAmount"] == {"value": 40, "currencyCode": "EUR"}


@pytest.mark.parametrize(
    ("record", "field"),
    [
        (valid_record(account_id=""), "records[0].accountId"),
        (valid_record(record_date=None), "records[0].recordDate"),
        (valid_record(record_date="not a date"), "records[0].recordDate"),
        (valid_record(record_date=FIXED_NOW + timedelta(hours=25)), "records[0].recordDate"),
        (valid_record(amount=AmountInput(value=0)), "records[0].amount.value"),
        (valid_record(amount=AmountInput(value=1, currency_code="US")), "records[0].amount.currencyCode"),
        (valid_record(note=long_text(256)), "records[0].note"),
        (valid_record(counter_party=long_text(256)), "records[0].counterParty"),
        (valid_record(record_state=RecordState.VOID), "records[0].recordState"),
        (valid_record(place=PlaceInput(latitude=91)), "records[0].place.latitude"),
        (valid_record(place=PlaceInput(longitude=-181)), "records[0].place.longitude"),
        (valid_record(place=PlaceInput(address=long_text(256))), "records[0].place.address"),
        (valid_record(place=PlaceInput(name=long_text(256))), "records[0].place.name"),
        (valid_record(label_ids=make_ids(31)), "records[0].labelIds"),
    ],
)
def test_create_records_validation(client, record, field):
    with pytest.raises(InvalidRequestError) as excinfo:
        client.create_records([record])
    assert_problem(excinfo, field)


def test_create_records_batch_size(client):
    with pytest.raises(InvalidRequestError) as excinfo:
        client.create_records([])
    assert_problem(excinfo, "records")

    with pytest.raises(InvalidRequestError) as excinfo:
        client.create_records([valid_record() for _ in range(51)])
    assert_problem(excinfo, "records")


@pytest.mark.parametrize("hours", [0, 1, 23, 24])
def test_record_date_inside_the_future_window_is_accepted(server, client, hours):
    server.queue(
        MockResponse(status=200, body=ok_batch({"inputIndex": 0, "id": "r1", "success": True}))
    )
    client.create_records([valid_record(record_date=FIXED_NOW + timedelta(hours=hours))])
    assert server.count == 1


def test_one_second_past_the_future_window_is_rejected(client):
    with pytest.raises(InvalidRequestError):
        client.create_records(
            [valid_record(record_date=FIXED_NOW + timedelta(hours=24, seconds=1))]
        )


def test_a_bare_date_is_sent_as_a_bare_date(server, client):
    # The API stores a date-only recordDate at 12:00:00Z; rewriting it as
    # midnight here would silently move the record half a day.
    from datetime import date

    server.queue(
        MockResponse(status=200, body=ok_batch({"inputIndex": 0, "id": "r1", "success": True}))
    )
    client.create_records([valid_record(record_date=date(2026, 9, 5))])
    assert server.last.json()[0]["recordDate"] == "2026-09-05"


@pytest.mark.parametrize(
    ("transfer", "field"),
    [
        (TransferInput(pairing_mode=None), "transfer.pairingMode"),
        (TransferInput(pairing_mode="clone"), "transfer.pairingMode"),
        (TransferInput(pairing_mode=PairingMode.NEW), "transfer.accountId"),
        (
            TransferInput(pairing_mode=PairingMode.NEW, account_id="a", record_id="r"),
            "transfer.recordId",
        ),
        (TransferInput(pairing_mode=PairingMode.EXISTING), "transfer.recordId"),
        (
            TransferInput(pairing_mode=PairingMode.EXISTING, record_id="r", account_id="a"),
            "transfer.accountId",
        ),
        (
            TransferInput(
                pairing_mode=PairingMode.EXISTING,
                record_id="r",
                counter_amount=CounterAmountInput(value=10, currency_code="EUR"),
            ),
            "transfer.counterAmount",
        ),
        (TransferInput(pairing_mode=PairingMode.UNPAIRED, account_id="a"), "transfer.accountId"),
        (TransferInput(pairing_mode=PairingMode.UNPAIRED, record_id="r"), "transfer.recordId"),
        (
            TransferInput(
                pairing_mode=PairingMode.NEW,
                account_id="a",
                counter_amount=CounterAmountInput(value=-10, currency_code="EUR"),
            ),
            "transfer.counterAmount.value",
        ),
        (
            TransferInput(
                pairing_mode=PairingMode.NEW,
                account_id="a",
                counter_amount=CounterAmountInput(value=0, currency_code="EUR"),
            ),
            "transfer.counterAmount.value",
        ),
        (
            TransferInput(
                pairing_mode=PairingMode.NEW,
                account_id="a",
                counter_amount=CounterAmountInput(value=10, currency_code=""),
            ),
            "transfer.counterAmount.currencyCode",
        ),
    ],
)
def test_transfer_validation(client, transfer, field):
    with pytest.raises(InvalidRequestError) as excinfo:
        client.create_records([valid_record(transfer=transfer)])
    assert_problem(excinfo, f"records[0].{field}")


@pytest.mark.parametrize(
    "transfer",
    [
        TransferInput(pairing_mode=PairingMode.NEW, account_id="acc2"),
        TransferInput(
            pairing_mode=PairingMode.NEW,
            account_id="acc2",
            counter_amount=CounterAmountInput(value=40, currency_code="EUR"),
        ),
        TransferInput(pairing_mode=PairingMode.EXISTING, record_id="r9"),
        TransferInput(pairing_mode=PairingMode.UNPAIRED),
        None,
    ],
)
def test_valid_transfer_combinations(server, client, transfer):
    server.queue(
        MockResponse(status=200, body=ok_batch({"inputIndex": 0, "id": "r1", "success": True}))
    )
    client.create_records([valid_record(transfer=transfer)])
    assert server.count == 1


# ---------------------------------------------------------------------------
# Patch
# ---------------------------------------------------------------------------


def test_patch_records_serialises_params(server, client):
    server.queue(
        MockResponse(status=200, body=ok_batch({"inputIndex": 0, "id": "r1", "success": True}))
    )
    client.patch_records(
        [PatchRecordItem(id="r1", note="updated")],
        validation=ValidationMode.STRICT,
        return_data=False,
    )
    assert server.last.q("validation") == "strict"
    assert server.last.q("returnData") == "false"


def test_patch_records_rejects_an_unknown_validation_mode(client):
    with pytest.raises(InvalidRequestError) as excinfo:
        client.patch_records([PatchRecordItem(id="r1", note="x")], validation="paranoid")
    assert_problem(excinfo, "validation")


@pytest.mark.parametrize(
    "item",
    [
        PatchRecordItem(id="r1", note="x", clear=[ClearableRecordField.NOTE]),
        PatchRecordItem(id="r1", category_id="c", clear=[ClearableRecordField.CATEGORY_ID]),
        PatchRecordItem(id="r1", label_ids=replace_ids(["l"]), clear=[ClearableRecordField.LABEL_IDS]),
        PatchRecordItem(id="r1", counter_party="x", clear=[ClearableRecordField.COUNTER_PARTY]),
        PatchRecordItem(id="r1", place=PlaceInput(), clear=[ClearableRecordField.PLACE]),
    ],
)
def test_clear_and_set_conflict(client, item):
    with pytest.raises(InvalidRequestError) as excinfo:
        client.patch_records([item])
    assert_problem(excinfo, "items[0].$clear[0]")


def test_clearing_transfer_alone_is_legal(server, client):
    # There is no transfer setter on a patch, so it can never conflict.
    server.queue(
        MockResponse(status=200, body=ok_batch({"inputIndex": 0, "id": "r1", "success": True}))
    )
    client.patch_records([PatchRecordItem(id="r1", clear=[ClearableRecordField.TRANSFER])])
    assert server.last.json()[0]["$clear"] == ["transfer"]


@pytest.mark.parametrize(
    ("item", "field"),
    [
        (PatchRecordItem(id="", note="x"), "items[0].id"),
        (PatchRecordItem(id="r1"), "items[0]"),
        (PatchRecordItem(id="r1", amount=AmountInput(value=0)), "items[0].amount.value"),
        (PatchRecordItem(id="r1", note=long_text(256)), "items[0].note"),
        (PatchRecordItem(id="r1", counter_party=long_text(256)), "items[0].counterParty"),
        (
            PatchRecordItem(id="r1", record_state=RecordState.WAIT_FOR_ASSIGN),
            "items[0].recordState",
        ),
        (PatchRecordItem(id="r1", clear=["amount"]), "items[0].$clear[0]"),
        (
            PatchRecordItem(
                id="r1", clear=[ClearableRecordField.NOTE, ClearableRecordField.NOTE]
            ),
            "items[0].$clear[1]",
        ),
        (PatchRecordItem(id="r1", label_ids=add_remove_ids()), "items[0].labelIds"),
        (PatchRecordItem(id="r1", record_date="not a date"), "items[0].recordDate"),
        (PatchRecordItem(id="r1", place=PlaceInput(latitude=200)), "items[0].place.latitude"),
    ],
)
def test_patch_record_validation(client, item, field):
    with pytest.raises(InvalidRequestError) as excinfo:
        client.patch_records([item])
    assert_problem(excinfo, field)


def test_patch_body_sends_both_id_set_shapes(server, client):
    server.queue(
        MockResponse(status=200, body=ok_batch({"inputIndex": 0, "id": "r1", "success": True}))
    )
    client.patch_records(
        [
            PatchRecordItem(
                id="r1",
                clear=[ClearableRecordField.NOTE, ClearableRecordField.PLACE],
                label_ids=add_remove_ids(add=["l1"], remove=["l2"]),
            )
        ]
    )
    sent = server.last.json()[0]
    assert sent["$clear"] == ["note", "place"]
    assert sent["labelIds"] == {"add": ["l1"], "remove": ["l2"]}


def test_strict_mode_400_decodes_to_the_envelope(server, client):
    server.queue(
        MockResponse(
            status=400,
            body=json.dumps(
                {
                    "results": [
                        {
                            "inputIndex": 0, "id": "r1", "success": False,
                            "error": "validated but not written", "errorType": "client_error",
                        },
                        {
                            "inputIndex": 1, "id": "r2", "success": False,
                            "error": "category not found", "errorType": "client_error",
                            "fields": ["categoryId"],
                        },
                    ],
                    "summary": {
                        "total": 2, "succeeded": 0, "clientErrors": 2,
                        "serverErrors": 0, "documentsWritten": 0,
                    },
                }
            ),
        )
    )
    response = client.patch_records(
        [PatchRecordItem(id="r1", note="a"), PatchRecordItem(id="r2", category_id="missing")],
        validation=ValidationMode.STRICT,
    )
    # Strict mode writes nothing.
    assert response.summary.documents_written == 0
    assert len(response.results) == 2


# ---------------------------------------------------------------------------
# IdSet
# ---------------------------------------------------------------------------


@pytest.mark.parametrize(
    ("id_set", "expected"),
    [
        (replace_ids(["a", "b"]), ["a", "b"]),
        (clear_ids(), []),
        (replace_ids(None), []),
        (add_remove_ids(add=["a"]), {"add": ["a"]}),
        (add_remove_ids(remove=["b"]), {"remove": ["b"]}),
        (add_remove_ids(add=["a"], remove=["b"]), {"add": ["a"], "remove": ["b"]}),
    ],
)
def test_id_set_encodes_both_documented_shapes(id_set, expected):
    assert id_set.to_json() == expected


def test_id_set_accessors():
    replacement = replace_ids(["a"])
    assert not replacement.is_delta
    assert replacement.values == ["a"]
    assert replacement.delta == {}

    delta = add_remove_ids(add=["a"], remove=["b"])
    assert delta.is_delta
    assert delta.values == []
    assert delta.delta == {"add": ["a"], "remove": ["b"]}

    assert replace_ids(["a"]) == replace_ids(["a"])
    assert replace_ids(["a"]) != add_remove_ids(add=["a"])
    assert replace_ids(["a"]) != "not an id set"
