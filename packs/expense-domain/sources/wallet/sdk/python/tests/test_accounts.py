"""Accounts: query serialisation, decoding, creation and batch updates."""

from __future__ import annotations

import json
from datetime import datetime, timezone

import pytest
from conftest import MockResponse, assert_problem, batch_body, long_text, make_ids

from wallet_sdk import (
    AccountType,
    BalanceMode,
    CreateAccountRequest,
    HintSeverity,
    HintType,
    InvalidRequestError,
    PatchAccountItem,
    UniversalColor,
    ValidationError,
    between,
    contains_i,
    desc,
    gte,
)
from wallet_sdk.enums import CREATABLE_ACCOUNT_TYPES


def test_list_accounts_serialises_every_parameter(server, client):
    server.queue(MockResponse(json_body={"accounts": []}))
    client.list_accounts(
        limit=50,
        offset=30,
        agent_hints=True,
        with_total=True,
        ids=["a1", "a2"],
        name=contains_i("check"),
        account_type=AccountType.CURRENT_ACCOUNT,
        currency_code="EUR",
        created_at=between("2024-01-01", "2024-04-01"),
        updated_at=[gte("2024-02-01T00:00:00Z")],
        archived=False,
        budget_id="b1",
        sort_by=desc("createdAt"),
    )

    request = server.last
    assert request.path == "/v1/api/accounts"
    assert request.method == "GET"
    expected = {
        "limit": "50",
        "offset": "30",
        "agentHints": "true",
        "withTotal": "true",
        "id": "a1,a2",
        "name": "contains-i.check",
        "accountType": "CurrentAccount",
        "currencyCode": "EUR",
        "archived": "false",
        "budgetId": "b1",
        "sortBy": "-createdAt",
    }
    for key, value in expected.items():
        assert request.q(key) == value, key
    # A range parameter sends one entry per condition, not a joined string.
    # A string bound passes through verbatim: a caller who writes a bare date
    # means a bare date, which the API reads as UTC midnight.
    assert request.qs("createdAt") == ["gte.2024-01-01", "lt.2024-04-01"]
    assert request.qs("updatedAt") == ["gte.2024-02-01T00:00:00Z"]


def test_list_accounts_omits_unset_parameters(server, client):
    server.queue(MockResponse(json_body={"accounts": []}))
    client.list_accounts()
    assert server.last.query == {}


def test_list_accounts_decodes_the_full_account_shape(server, client):
    server.queue(
        MockResponse(
            body=json.dumps(
                {
                    "limit": 30,
                    "offset": 0,
                    "nextOffset": 30,
                    "total": 42,
                    "accounts": [
                        {
                            "id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
                            "name": "My Checking Account",
                            "accountType": "CurrentAccount",
                            "archived": False,
                            "bankAccountNumber": "CZ65 0800 0000 1920 0014 5399",
                            "color": "#d32f2f",
                            "currencyCode": "USD",
                            "excludeFromStats": False,
                            "isBankSync": True,
                            "isInvestmentAccount": False,
                            "createdAt": "2021-03-24T09:28:41.200Z",
                            "updatedAt": "2021-03-25T09:28:41.200Z",
                            "balance": {
                                "balanceMode": "creditCardBank",
                                "balanceModeFormula": "creditBalance = -(initial + totalIncomes - totalExpenses)",
                                "currencyCode": "USD",
                                "currentBalance": 3343.75,
                                "rawCurrentBalance": 3300.00,
                                "creditLimit": 5000,
                                "creditBalance": -1656.25,
                                "availableCredit": 3343.75,
                                "formula": "initial + totalIncomes - totalExpenses",
                                "initial": 1000,
                            },
                            "recordStats": {
                                "recordCount": 42,
                                "totalExpenses": 12890.75,
                                "totalIncomes": 15234.5,
                                "lastUpdatedAt": "2025-03-20T14:22:31.456Z",
                                "recordDate": {
                                    "min": "2024-01-15T00:00:00.000Z",
                                    "max": "2025-03-20T00:00:00.000Z",
                                },
                                "createdAt": {
                                    "min": "2024-01-15T10:30:00.000Z",
                                    "max": "2025-03-20T14:22:00.000Z",
                                },
                            },
                        }
                    ],
                    "agentHints": [
                        {
                            "type": "pagination.has_more",
                            "severity": "instruction",
                            "text": "More results available",
                            "action": {"url": "https://example.test/next"},
                            "data": {"remaining": 12},
                        }
                    ],
                }
            )
        )
    )
    page = client.list_accounts()

    assert page.has_more and page.next_offset == 30
    assert page.total == 42

    account = page.accounts[0]
    assert account.account_type is AccountType.CURRENT_ACCOUNT
    assert account.is_bank_sync is True
    assert account.balance.balance_mode is BalanceMode.CREDIT_CARD_BANK
    assert account.balance.current_balance == 3343.75
    # The write-only initialBalance reads back here.
    assert account.balance.initial == 1000
    assert account.record_stats.record_count == 42
    assert account.record_stats.record_date.min == "2024-01-15T00:00:00.000Z"
    assert account.created_at == datetime(2021, 3, 24, 9, 28, 41, 200000, tzinfo=timezone.utc)

    hint = page.agent_hints.first(HintType.PAGINATION_HAS_MORE)
    assert hint is not None
    assert hint.severity is HintSeverity.INSTRUCTION
    assert hint.action.url == "https://example.test/next"
    assert hint.data == {"remaining": 12}
    assert page.agent_hints.by_type(HintType.RESULT_EMPTY) == []


def test_null_balance_and_stats_decode_to_none(server, client):
    server.queue(
        MockResponse(
            body='{"accounts":[{"id":"a1","balance":null,"recordStats":null,"currencyCode":""}]}'
        )
    )
    account = client.list_accounts().accounts[0]
    assert account.balance is None
    assert account.record_stats is None
    # The documented empty string when the currency cannot be resolved.
    assert account.currency_code == ""


@pytest.mark.parametrize(
    ("kwargs", "field"),
    [
        ({"limit": 0}, "limit"),
        ({"limit": 201}, "limit"),
        ({"offset": -1}, "offset"),
        ({"ids": make_ids(31)}, "id"),
        ({"ids": ["a", "  "]}, "id[1]"),
        ({"ids": ["a", "a"]}, "id[1]"),
        ({"ids": ["a,b"]}, "id[0]"),
        ({"ids": "a,b"}, "id"),
        ({"name": gte("x")}, "name"),
        ({"created_at": [gte("a"), gte("b"), gte("c")]}, "createdAt"),
        ({"created_at": [contains_i("x")]}, "createdAt[0]"),
        ({"account_type": "Crypto"}, "accountType"),
        ({"currency_code": "EURO"}, "currencyCode"),
        ({"sort_by": desc("name")}, "sortBy"),
    ],
)
def test_list_accounts_validation(client, kwargs, field):
    with pytest.raises(InvalidRequestError) as excinfo:
        client.list_accounts(**kwargs)
    assert_problem(excinfo, field)


@pytest.mark.parametrize("account_type", list(AccountType))
def test_every_account_type_is_filterable(server, client, account_type):
    server.queue(MockResponse(json_body={"accounts": []}))
    client.list_accounts(account_type=account_type)
    assert server.last.q("accountType") == account_type.value


def test_create_account_happy_path(server, client):
    server.queue(
        MockResponse(
            status=201,
            body=json.dumps(
                {
                    "account": {
                        "id": "a1",
                        "name": "My new account",
                        "accountType": "General",
                        "currencyCode": "EUR",
                        "balance": {"initial": 0, "currencyCode": "EUR"},
                    },
                    "summary": {
                        "total": 1, "succeeded": 1, "clientErrors": 0,
                        "serverErrors": 0, "documentsWritten": 1,
                    },
                }
            ),
        )
    )
    response = client.create_account(
        CreateAccountRequest(
            name="My new account",
            account_type=AccountType.GENERAL,
            currency_code="EUR",
            initial_balance=0,
            color=UniversalColor.TURQUOISE,
            exclude_from_stats=True,
        )
    )
    assert response.meta.status_code == 201
    assert response.account.id == "a1"
    assert response.summary.all_succeeded

    sent = server.last.json()
    # initialBalance must be present even at zero, since the API requires it.
    assert sent["initialBalance"] == 0
    assert sent["color"] == "Turquoise"
    assert sent["excludeFromStats"] is True
    # Optional fields left unset must not appear at all.
    assert "bankAccountNumber" not in sent


@pytest.mark.parametrize(
    ("kwargs", "field"),
    [
        ({"name": "", "account_type": AccountType.CASH, "currency_code": "USD"}, "name"),
        ({"name": long_text(81), "account_type": AccountType.CASH, "currency_code": "USD"}, "name"),
        ({"name": "n", "account_type": None, "currency_code": "USD"}, "accountType"),
        ({"name": "n", "account_type": AccountType.CREDIT_CARD, "currency_code": "USD"}, "accountType"),
        ({"name": "n", "account_type": "Crypto", "currency_code": "USD"}, "accountType"),
        ({"name": "n", "account_type": AccountType.CASH, "currency_code": ""}, "currencyCode"),
        ({"name": "n", "account_type": AccountType.CASH, "currency_code": "US"}, "currencyCode"),
        ({"name": "n", "account_type": AccountType.CASH, "currency_code": "U5D"}, "currencyCode"),
        (
            {
                "name": "n", "account_type": AccountType.CASH, "currency_code": "USD",
                "bank_account_number": long_text(81),
            },
            "bankAccountNumber",
        ),
        (
            {"name": "n", "account_type": AccountType.CASH, "currency_code": "USD", "color": "Beige"},
            "color",
        ),
    ],
)
def test_create_account_validation(kwargs, field):
    with pytest.raises(InvalidRequestError) as excinfo:
        CreateAccountRequest(**kwargs).validate()
    assert_problem(excinfo, field)


@pytest.mark.parametrize("account_type", CREATABLE_ACCOUNT_TYPES)
def test_every_creatable_account_type_is_accepted(account_type):
    CreateAccountRequest(name="n", account_type=account_type, currency_code="USD").validate()


@pytest.mark.parametrize(
    "account_type",
    [t for t in AccountType if t not in CREATABLE_ACCOUNT_TYPES],
)
def test_read_only_account_types_are_rejected(account_type):
    with pytest.raises(InvalidRequestError):
        CreateAccountRequest(name="n", account_type=account_type, currency_code="USD").validate()


@pytest.mark.parametrize("colour", list(UniversalColor))
def test_every_universal_colour_is_accepted(colour):
    CreateAccountRequest(
        name="n", account_type=AccountType.CASH, currency_code="USD", color=colour
    ).validate()


@pytest.mark.parametrize(
    ("status", "partial", "all_ok"),
    [(200, False, True), (207, True, False), (400, False, False), (500, False, False)],
)
def test_patch_accounts_batch_outcomes(server, client, status, partial, all_ok):
    server.queue(MockResponse(status=status, body=batch_body(status)))
    response = client.patch_accounts(
        [PatchAccountItem(id="a1", name="one"), PatchAccountItem(id="a2", name="two")]
    )
    assert response.meta.status_code == status
    assert response.summary.partial is partial
    assert response.summary.all_succeeded is all_ok
    assert [r.input_index for r in response.results] == [0, 1]


def test_patch_accounts_mixed_result_carries_per_item_detail(server, client):
    server.queue(
        MockResponse(
            status=207,
            body=json.dumps(
                {
                    "results": [
                        {
                            "inputIndex": 0, "id": "a1", "success": True,
                            "account": {"id": "a1", "name": "Renamed"},
                        },
                        {
                            "inputIndex": 1, "id": "a2", "success": False,
                            "error": "account not found", "errorType": "client_error",
                            "fields": ["id"],
                        },
                    ],
                    "summary": {
                        "total": 2, "succeeded": 1, "clientErrors": 1,
                        "serverErrors": 0, "documentsWritten": 1,
                    },
                }
            ),
        )
    )
    response = client.patch_accounts(
        [PatchAccountItem(id="a1", name="Renamed"), PatchAccountItem(id="a2", name="Nope")],
        return_data=True,
    )
    ok, bad = response.results
    assert ok.success and ok.account.name == "Renamed"
    assert not bad.success and bad.is_client_error and bad.fields == ["id"]
    assert response.summary.failed == 1 and response.summary.partial

    assert [r.id for r in response.succeeded] == ["a1"]
    assert [r.id for r in response.failed] == ["a2"]
    assert [r.id for r in response.client_errors] == ["a2"]
    assert response.server_errors == []
    assert response.result_for(1) is bad
    assert response.result_for(99) is None
    assert server.last.q("returnData") == "true"
    assert server.last.method == "PATCH"


def test_patch_accounts_return_data_false_omits_bodies(server, client):
    server.queue(
        MockResponse(
            status=200,
            body=json.dumps(
                {
                    "results": [{"inputIndex": 0, "id": "a1", "success": True}],
                    "summary": {
                        "total": 1, "succeeded": 1, "clientErrors": 0,
                        "serverErrors": 0, "documentsWritten": 1,
                    },
                }
            ),
        )
    )
    response = client.patch_accounts([PatchAccountItem(id="a1", archived=True)], return_data=False)
    assert server.last.q("returnData") == "false"
    assert response.results[0].account is None
    # The id and success flag must survive return_data=False.
    assert response.results[0].success and response.results[0].id == "a1"


def test_patch_accounts_plain_400_is_a_validation_error(server, client):
    # A 400 whose body is a bare validation error, not a batch envelope, must
    # raise rather than decode to an empty envelope.
    server.queue(MockResponse(status=400, body='{"error":"invalid parameter"}'))
    with pytest.raises(ValidationError) as excinfo:
        client.patch_accounts([PatchAccountItem(id="a1", name="x")])
    assert excinfo.value.message == "invalid parameter"


def test_patch_accounts_batch_500_is_not_retried(server, make_client, sleeper):
    from wallet_sdk import RetryConfig

    server.queue(MockResponse(status=500, body=batch_body(500)))
    client = make_client(
        retry=RetryConfig(max_retries=3, retry_writes=True, base_delay=0.001, sleep=sleeper)
    )
    response = client.patch_accounts(
        [PatchAccountItem(id="a1", name="x"), PatchAccountItem(id="a2", name="y")]
    )
    # A batch envelope carries per-item detail and must not be retried away.
    assert server.count == 1
    assert response.summary.server_errors == 2


@pytest.mark.parametrize(
    ("items", "field"),
    [
        ([], "items"),
        ([PatchAccountItem(id=f"id-{n}", archived=True) for n in range(11)], "items"),
        ([PatchAccountItem(id="", name="x")], "items[0].id"),
        ([PatchAccountItem(id="a1")], "items[0]"),
        ([PatchAccountItem(id="a1", name=long_text(81))], "items[0].name"),
        ([PatchAccountItem(id="a1", name="  ")], "items[0].name"),
        ([PatchAccountItem(id="a1", color="Beige")], "items[0].color"),
        ([PatchAccountItem(id="a1", bank_account_number=long_text(81))], "items[0].bankAccountNumber"),
        (
            [PatchAccountItem(id="a1", archived=True), PatchAccountItem(id="a1", archived=False)],
            "items[1].id",
        ),
    ],
)
def test_patch_accounts_validation(client, items, field):
    with pytest.raises(InvalidRequestError) as excinfo:
        client.patch_accounts(items)
    assert_problem(excinfo, field)


def test_empty_bank_account_number_clears_the_value(server, client):
    # An empty string is meaningful here, so validation must let it through.
    server.queue(
        MockResponse(
            status=200,
            body=json.dumps(
                {
                    "results": [{"inputIndex": 0, "id": "a1", "success": True}],
                    "summary": {
                        "total": 1, "succeeded": 1, "clientErrors": 0,
                        "serverErrors": 0, "documentsWritten": 1,
                    },
                }
            ),
        )
    )
    client.patch_accounts([PatchAccountItem(id="a1", bank_account_number="")])
    assert server.last.json() == [{"id": "a1", "bankAccountNumber": ""}]
