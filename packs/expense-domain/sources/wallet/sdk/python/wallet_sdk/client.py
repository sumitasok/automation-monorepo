"""The Wallet API client: every documented operation, on one object."""

from __future__ import annotations

import json
from datetime import date, datetime, timezone
from typing import Any, Callable, Dict, Iterable, Iterator, List, Mapping, Optional, Sequence, Tuple, Union
from urllib.parse import urlencode, urlsplit

from ._convert import parse_datetime
from ._problems import Problems
from .enums import (
    WRITABLE_RECORD_STATES,
    AccountType,
    BudgetType,
    Cardinality,
    EntityType,
    RecordSource,
    RecordState,
    RecordType,
    ValidationMode,
    normalize_entity_type,
    names_of,
    DELETABLE_ENTITY_TYPES,
    REFERENCEABLE_ENTITY_TYPES,
)
from .errors import (
    APIError,
    DecodeError,
    InvalidRequestError,
    Problem,
    RateLimitError,
    ServerError,
    TransportError,
    default_message_for_status,
    error_for_status,
)
from .filters import (
    COMMON_SORT_FIELDS,
    RECORD_SORT_FIELDS,
    Filter,
    Sort,
    validate_range_filters,
    validate_sort,
    validate_text_filter,
)
from .models import (
    AccountsPage,
    APIUsageStats,
    BudgetsPage,
    CategoriesPage,
    CreateAccountResponse,
    CreateBudgetResponse,
    CreateCategoryResponse,
    CreateLabelResponse,
    CreateRecordResult,
    CreateRecordsResponse,
    DeleteResponse,
    GoalsPage,
    LabelsPage,
    PatchAccountsResponse,
    PatchBudgetsResponse,
    PatchCategoriesResponse,
    PatchLabelsResponse,
    PatchRecordsResponse,
    RecordRulesPage,
    RecordsPage,
    ReferencesResult,
    ResponseMeta,
    StandingOrderItemsPage,
    StandingOrdersPage,
)
from .requests import (
    CreateAccountRequest,
    CreateBudgetRequest,
    CreateLabelRequest,
    CreateRecordRequest,
    CreateSubcategoryRequest,
    PatchAccountItem,
    PatchBudgetItem,
    PatchCategoryItem,
    PatchLabelItem,
    PatchRecordItem,
)
from .transport import (
    DEFAULT_TIMEOUT,
    HTTPRequest,
    HTTPResponse,
    RetryConfig,
    Transport,
    UrllibTransport,
    parse_retry_after,
)
from .validation import (
    MAX_ACCOUNT_ID_FILTER_VALUES,
    MAX_CATEGORY_ID_FILTER_VALUES,
    MAX_CREATE_RECORDS,
    MAX_DELETE_IDS,
    MAX_ID_FILTER_VALUES,
    MAX_PATCH_ITEMS,
    MAX_REFERENCE_IDS,
    check_batch_size,
    check_entity_type,
    check_enum,
    check_cardinality,
    check_id_list,
    check_limit,
    check_offset,
    check_optional_currency_code,
    check_spending_depth,
    check_usage_period,
)

__all__ = ["WalletClient", "DEFAULT_BASE_URL", "DEFAULT_USER_AGENT", "API_PREFIX"]

#: The production Wallet API server.
DEFAULT_BASE_URL = "https://rest.budgetbakers.com/wallet"

#: The version segment every operation sits under.
API_PREFIX = "/v1/api"

#: Identifies this SDK in request logs.
DEFAULT_USER_AGENT = "wallet-python-sdk/1.0 (+https://rest.budgetbakers.com/wallet)"

#: The statuses on which a batch endpoint answers with its own envelope.
_BATCH_STATUSES = frozenset({200, 207, 400, 500})

#: Bounds an auto-pagination walk, so a server that keeps answering with a
#: next offset cannot spin the caller forever.
_MAX_AUTO_PAGES = 10_000

#: Magic values for the ``convert_to`` parameter.
CONVERT_TO_BASE = "base"
CONVERT_TO_ACCOUNT = "account"


class WalletClient:
    """A client for the BudgetBakers Wallet REST API.

    Args:
        token: The bearer JWT minted for the Wallet flavour of the API. A Board
            token on a Wallet path yields 403.
        base_url: Overrides the API server. A path component is preserved, so a
            proxy mounted at ``/api/wallet`` works.
        timeout: Bounds each attempt, in seconds.
        retry: The retry policy. Defaults to three retries on rate limits,
            server errors and transport faults, for idempotent methods only.
        transport: Overrides the HTTP layer. Anything with a ``send`` method.
        user_agent: Overrides the ``User-Agent`` header.
        headers: Extra headers added to every request. ``Authorization``,
            ``Accept`` and ``Content-Type`` are always set by the client and
            cannot be overridden here.
        validate: Set ``False`` to skip the SDK's own checks and let the server
            decide. Useful when the API gains a value this SDK does not know.
        now: Overrides the clock, for tests.

    Raises:
        InvalidRequestError: when the token is missing or the base URL unusable.

    Example:
        >>> client = WalletClient(token="...")                # doctest: +SKIP
        >>> page = client.list_accounts(limit=50)             # doctest: +SKIP
        >>> for record in client.iter_records(                # doctest: +SKIP
        ...     record_date=between("2024-01-01", "2024-04-01")
        ... ):
        ...     print(record.amount.value)
    """

    def __init__(
        self,
        token: str,
        *,
        base_url: str = DEFAULT_BASE_URL,
        timeout: Optional[float] = DEFAULT_TIMEOUT,
        retry: Optional[RetryConfig] = None,
        transport: Optional[Transport] = None,
        user_agent: str = DEFAULT_USER_AGENT,
        headers: Optional[Mapping[str, str]] = None,
        validate: bool = True,
        now: Optional[Callable[[], datetime]] = None,
    ) -> None:
        problems = Problems()
        if not isinstance(token, str) or not token.strip():
            problems.add("token", "is required")
        parsed = urlsplit(base_url or "")
        if parsed.scheme not in ("http", "https"):
            problems.add("base_url", f"must be http or https, got {parsed.scheme or base_url!r}")
        elif not parsed.netloc:
            problems.add("base_url", "must include a host")
        problems.raise_if_any()

        self._token = token
        self._base_url = base_url.rstrip("/")
        self._timeout = timeout
        self._retry = retry if retry is not None else RetryConfig()
        self._transport = transport or UrllibTransport()
        self._user_agent = user_agent
        self._extra_headers = dict(headers or {})
        self._validate = validate
        self._now = now or (lambda: datetime.now(timezone.utc))
        self._last_meta = ResponseMeta()

    # -- introspection ------------------------------------------------------

    @property
    def base_url(self) -> str:
        """The configured server, without the ``/v1/api`` prefix."""
        return self._base_url

    @property
    def last_meta(self) -> ResponseMeta:
        """The metadata of the most recent response, errors included.

        The zero value means nothing has been sent yet.
        """
        return self._last_meta

    # -- context manager ----------------------------------------------------

    def __enter__(self) -> "WalletClient":
        return self

    def __exit__(self, *exc_info: Any) -> None:
        self.close()

    def close(self) -> None:
        """Release the transport, if it holds anything."""
        closer = getattr(self._transport, "close", None)
        if callable(closer):
            closer()

    # -- request plumbing ---------------------------------------------------

    def _url(self, path: str, query: Optional[List[Tuple[str, str]]] = None) -> str:
        url = f"{self._base_url}{API_PREFIX}{path}"
        if query:
            url = f"{url}?{urlencode(query)}"
        return url

    def _request(
        self,
        operation: str,
        method: str,
        path: str,
        *,
        query: Optional[List[Tuple[str, str]]] = None,
        body: Any = None,
        batch: bool = False,
    ) -> Tuple[Any, ResponseMeta]:
        """Send one request, with retries, and return its decoded body and meta."""
        payload: Optional[bytes] = None
        if body is not None:
            payload = json.dumps(body).encode("utf-8")

        headers = dict(self._extra_headers)
        headers.update(
            {
                "Authorization": f"Bearer {self._token}",
                "Accept": "application/json",
                "User-Agent": self._user_agent,
            }
        )
        if payload is not None:
            headers["Content-Type"] = "application/json"

        request = HTTPRequest(
            method=method,
            url=self._url(path, query),
            headers=headers,
            body=payload,
            timeout=self._timeout,
        )

        attempt = 0
        while True:
            try:
                return self._attempt(operation, request, batch, attempt + 1)
            except APIError as err:
                if attempt >= self._retry.max_retries or not self._retry.allows(method, attempt, err):
                    raise
                self._retry.sleep(self._retry.backoff(attempt, err))
                attempt += 1

    def _attempt(
        self, operation: str, request: HTTPRequest, batch: bool, attempt_no: int
    ) -> Tuple[Any, ResponseMeta]:
        try:
            response = self._transport.send(request)
        except TransportError as err:
            err.operation = err.operation or operation
            raise
        except Exception as err:  # pragma: no cover - a transport that misbehaves
            raise TransportError("request failed", operation=operation, cause=err) from err

        meta = _meta_from_response(response, attempt_no)
        self._last_meta = meta
        return self._handle(operation, response, meta, batch), meta

    def _handle(self, operation: str, response: HTTPResponse, meta: ResponseMeta, batch: bool) -> Any:
        status = response.status

        # A batch endpoint answers 200, 207, 400 and 500 with the same
        # envelope. Decode those rather than raising — but only when the body
        # really is a batch envelope, because a malformed request still yields
        # a plain ValidationError with the same 400.
        if batch and status in _BATCH_STATUSES:
            decoded = _try_json(response)
            if _looks_like_batch_envelope(decoded):
                return decoded

        if 200 <= status < 300:
            if not response.body.strip():
                return {}
            try:
                return response.json()
            except ValueError as err:
                raise DecodeError(
                    "cannot decode response body",
                    operation=operation,
                    status_code=status,
                    body=response.text(),
                    meta=meta,
                    cause=err,
                ) from err

        raise _api_error(operation, response, meta)

    # -- helpers ------------------------------------------------------------

    def _problems(self) -> Optional[Problems]:
        """A fresh accumulator, or ``None`` when validation is disabled."""
        return Problems() if self._validate else None

    @staticmethod
    def _finish(problems: Optional[Problems]) -> None:
        if problems is not None:
            problems.raise_if_any()

    # =======================================================================
    # Accounts
    # =======================================================================

    def list_accounts(
        self,
        *,
        limit: Optional[int] = None,
        offset: Optional[int] = None,
        agent_hints: Optional[bool] = None,
        with_total: Optional[bool] = None,
        ids: Optional[Sequence[str]] = None,
        name: Optional[Filter] = None,
        account_type: Optional[Any] = None,
        currency_code: Optional[str] = None,
        created_at: Optional[Sequence[Filter]] = None,
        updated_at: Optional[Sequence[Filter]] = None,
        archived: Optional[bool] = None,
        budget_id: Optional[str] = None,
        sort_by: Optional[Sort] = None,
    ) -> AccountsPage:
        """Return one page of financial accounts.

        Operation ``getAccounts``, ``GET /v1/api/accounts``.

        Args:
            limit: Page size, 1-200. The server defaults to 30.
            offset: Items to skip, for pagination.
            agent_hints: Ask for structured hints alongside the data.
            with_total: Ask for a total count, at the cost of an extra query.
            ids: Filter by identifier, up to 30.
            name: Filter by name; requires a text prefix.
            account_type: Filter by type, case-insensitively.
            currency_code: Filter by ISO 4217 code, case-insensitively.
            created_at: Filter by creation timestamp; up to two conditions.
            updated_at: Filter by last sync timestamp; up to two conditions.
            archived: ``True`` for archived only, ``False`` for active only.
            budget_id: Only the accounts a given budget tracks.
            sort_by: Order the page. Only ``createdAt`` and ``updatedAt`` sort.

        Returns:
            The page, with ``meta`` carrying the response headers.

        Raises:
            InvalidRequestError: when a documented constraint is violated.
            APIError: on any non-2xx the server returns.
        """
        problems = self._problems()
        if problems is not None:
            check_limit(problems, limit)
            check_offset(problems, offset)
            check_id_list(problems, "id", ids, MAX_ID_FILTER_VALUES)
            validate_text_filter(problems, "name", name)
            validate_range_filters(problems, "createdAt", created_at)
            validate_range_filters(problems, "updatedAt", updated_at)
            check_enum(problems, "accountType", account_type, tuple(AccountType), "an account type")
            check_optional_currency_code(problems, "currencyCode", currency_code)
            validate_sort(problems, "sortBy", sort_by, COMMON_SORT_FIELDS)
            self._finish(problems)

        query = _Query()
        query.add_int("limit", limit)
        query.add_int("offset", offset)
        query.add_bool("agentHints", agent_hints)
        query.add_bool("withTotal", with_total)
        query.add_csv("id", ids)
        query.add_filter("name", name)
        query.add_enum("accountType", account_type)
        query.add_str("currencyCode", currency_code)
        query.add_filters("createdAt", created_at)
        query.add_filters("updatedAt", updated_at)
        query.add_bool("archived", archived)
        query.add_str("budgetId", budget_id)
        query.add_sort("sortBy", sort_by)

        data, meta = self._request("getAccounts", "GET", "/accounts", query=query.pairs)
        page = AccountsPage.from_dict(_as_dict(data))
        page.meta = meta
        return page

    def create_account(self, request: CreateAccountRequest) -> CreateAccountResponse:
        """Create a financial account.

        Operation ``createAccount``, ``POST /v1/api/accounts``. Answers 201.

        Args:
            request: The account to create.

        Returns:
            The created account and its write summary.

        Raises:
            InvalidRequestError: when a documented constraint is violated.
            APIError: on any non-2xx the server returns.
        """
        if self._validate:
            request.validate()
        data, meta = self._request("createAccount", "POST", "/accounts", body=request.to_dict())
        response = CreateAccountResponse.from_dict(_as_dict(data))
        response.meta = meta
        return response

    def patch_accounts(
        self,
        items: Sequence[PatchAccountItem],
        *,
        return_data: Optional[bool] = None,
    ) -> PatchAccountsResponse:
        """Update up to 10 accounts in one call.

        Operation ``patchAccounts``, ``PATCH /v1/api/accounts``.

        The response envelope is returned for 200, 207, 400 and 500 alike;
        inspect ``summary`` and ``results``, correlating each result to its
        input through ``input_index``.

        Args:
            items: The updates, 1 to 10 of them.
            return_data: When ``False``, each result omits the updated account
                but keeps its id, success flag and error.

        Returns:
            The batch envelope.

        Raises:
            InvalidRequestError: when a documented constraint is violated.
            APIError: on a 401, 403, 429, or a 400 that is not a batch envelope.
        """
        self._validate_patch_batch(items)
        query = _Query()
        query.add_bool("returnData", return_data)
        data, meta = self._request(
            "patchAccounts", "PATCH", "/accounts",
            query=query.pairs, body=[i.to_dict() for i in items], batch=True,
        )
        response = PatchAccountsResponse.from_dict(_as_dict(data))
        response.meta = meta
        return response

    # =======================================================================
    # Records
    # =======================================================================

    def list_records(
        self,
        *,
        ids: Optional[Sequence[str]] = None,
        account_ids: Optional[Sequence[str]] = None,
        record_date: Optional[Sequence[Filter]] = None,
        limit: Optional[int] = None,
        offset: Optional[int] = None,
        agent_hints: Optional[bool] = None,
        with_total: Optional[bool] = None,
        category_ids: Optional[Sequence[str]] = None,
        label_id: Optional[str] = None,
        note: Optional[Filter] = None,
        counter_party: Optional[Filter] = None,
        amount: Optional[Sequence[Filter]] = None,
        created_at: Optional[Sequence[Filter]] = None,
        updated_at: Optional[Sequence[Filter]] = None,
        record_type: Optional[Any] = None,
        is_transfer: Optional[bool] = None,
        transfer_ids: Optional[Sequence[str]] = None,
        record_states: Optional[Sequence[Any]] = None,
        sources: Optional[Sequence[Any]] = None,
        sort_by: Optional[Sort] = None,
        convert_to: Optional[str] = None,
    ) -> RecordsPage:
        """Return one page of financial records.

        Operation ``getRecords``, ``GET /v1/api/records``.

        Note the default window: with no ``record_date`` bound the server
        applies an implicit three-month filter and reports it back in
        ``applied_record_date_filters``. Any single bound overrides it, and an
        ID lookup bypasses it entirely.

        Args:
            ids: Filter by identifier, up to 30. Bypasses the default window.
            account_ids: Filter by account, up to 10.
            record_date: Filter by transaction date; up to two conditions.
            limit: Page size, 1-200. The server defaults to 30.
            offset: Items to skip, for pagination.
            agent_hints: Ask for structured hints alongside the data.
            with_total: Ask for a total count, at the cost of an extra query.
            category_ids: Filter by category, up to 10. ``CATEGORY_UNKNOWN``
                expands to the three unknown categories.
            label_id: Only records carrying this label.
            note: Filter by note; requires a text prefix.
            counter_party: Filter by payee or payer; requires a text prefix.
            amount: Filter on ``amount.value``; up to two conditions.
            created_at: Filter by creation timestamp; up to two conditions.
            updated_at: Filter by last sync timestamp; up to two conditions.
            record_type: Filter on the amount sign.
            is_transfer: ``True`` for transfers only, ``False`` to exclude them.
                Independent of ``record_type``.
            transfer_ids: Every record sharing a transfer identity.
            record_states: Filter by clearing state; only the three writable
                states are accepted.
            sources: Filter by the client that created the record.
            sort_by: Order the page. ``recordDate``, ``amount``, ``createdAt``
                and ``updatedAt`` sort; the server defaults to ``-recordDate``.
            convert_to: Convert amounts to a currency, or to ``base`` or
                ``account``.

        Returns:
            The page, with ``meta`` carrying the response headers.

        Raises:
            InvalidRequestError: when a documented constraint is violated.
            APIError: on any non-2xx the server returns.
        """
        problems = self._problems()
        if problems is not None:
            check_limit(problems, limit)
            check_offset(problems, offset)
            check_id_list(problems, "id", ids, MAX_ID_FILTER_VALUES)
            check_id_list(problems, "accountId", account_ids, MAX_ACCOUNT_ID_FILTER_VALUES)
            check_id_list(problems, "categoryId", category_ids, MAX_CATEGORY_ID_FILTER_VALUES)
            check_id_list(problems, "transferId", transfer_ids, MAX_ID_FILTER_VALUES)
            validate_range_filters(problems, "recordDate", record_date)
            validate_range_filters(problems, "amount", amount)
            validate_range_filters(problems, "createdAt", created_at)
            validate_range_filters(problems, "updatedAt", updated_at)
            validate_text_filter(problems, "note", note)
            validate_text_filter(problems, "counterParty", counter_party)
            check_enum(problems, "recordType", record_type, tuple(RecordType), "a record type")
            for index, state in enumerate(record_states or ()):
                if state not in WRITABLE_RECORD_STATES:
                    problems.add(
                        f"recordState[{index}]",
                        f"{str(state)!r} cannot be filtered on; "
                        f"allowed: {names_of(WRITABLE_RECORD_STATES)}",
                    )
            for index, source in enumerate(sources or ()):
                if not RecordSource.is_valid(source):
                    problems.add(
                        f"source[{index}]",
                        f"{str(source)!r} is not a source; allowed: {names_of(tuple(RecordSource))}",
                    )
            validate_sort(problems, "sortBy", sort_by, RECORD_SORT_FIELDS)
            if convert_to is not None and convert_to not in (CONVERT_TO_BASE, CONVERT_TO_ACCOUNT):
                check_optional_currency_code(problems, "convertTo", convert_to)
            self._finish(problems)

        query = _Query()
        query.add_csv("id", ids)
        query.add_csv("accountId", account_ids)
        query.add_filters("recordDate", record_date)
        query.add_int("limit", limit)
        query.add_int("offset", offset)
        query.add_bool("agentHints", agent_hints)
        query.add_bool("withTotal", with_total)
        query.add_csv("categoryId", category_ids)
        query.add_str("labelId", label_id)
        query.add_filter("note", note)
        query.add_filter("counterParty", counter_party)
        query.add_filters("amount", amount)
        query.add_filters("createdAt", created_at)
        query.add_filters("updatedAt", updated_at)
        query.add_enum("recordType", record_type)
        query.add_bool("isTransfer", is_transfer)
        query.add_csv("transferId", transfer_ids)
        query.add_enum_csv("recordState", record_states)
        query.add_enum_csv("source", sources)
        query.add_sort("sortBy", sort_by)
        query.add_str("convertTo", convert_to)

        data, meta = self._request("getRecords", "GET", "/records", query=query.pairs)
        page = RecordsPage.from_dict(_as_dict(data))
        page.meta = meta
        return page

    def create_records(
        self,
        records: Sequence[CreateRecordRequest],
        *,
        return_data: Optional[bool] = None,
    ) -> CreateRecordsResponse:
        """Create up to 50 transaction records in one call.

        Operation ``createRecords``, ``POST /v1/api/records``.

        The response envelope is returned for 200, 207, 400 and 500 alike;
        inspect ``summary`` and ``results``, correlating each result to its
        input through ``input_index``.

        Args:
            records: The records to create, 1 to 50 of them.
            return_data: When ``False``, each result omits the created record
                but keeps its input index, id, success flag and error.
                ``created_mirror_record_id`` survives either way.

        Returns:
            The batch envelope.

        Raises:
            InvalidRequestError: when a documented constraint is violated.
            APIError: on a 401, 403, 429, or a 400 that is not a batch envelope.
        """
        if self._validate:
            problems = Problems()
            check_batch_size(problems, "records", len(records), MAX_CREATE_RECORDS)
            now = self._now()
            for index, record in enumerate(records):
                record.validate(problems, f"records[{index}]", now=now)
            problems.raise_if_any()

        query = _Query()
        query.add_bool("returnData", return_data)
        data, meta = self._request(
            "createRecords", "POST", "/records",
            query=query.pairs, body=[r.to_dict() for r in records], batch=True,
        )
        response = CreateRecordsResponse.from_dict(_as_dict(data))
        response.meta = meta
        return response

    def create_record(
        self,
        record: CreateRecordRequest,
        *,
        return_data: Optional[bool] = None,
    ) -> Tuple[CreateRecordResult, CreateRecordsResponse]:
        """Create one record, the single-item form of :meth:`create_records`.

        A failed item is data, not an exception: check ``result.success``
        exactly as with the batch form.

        Args:
            record: The record to create.
            return_data: When ``False``, the result omits the created record.

        Returns:
            A ``(result, response)`` pair.

        Raises:
            InvalidRequestError: when a documented constraint is violated.
            DecodeError: when the server returns no per-item result.
            APIError: on a 401, 403, 429, or a 400 that is not a batch envelope.
        """
        response = self.create_records([record], return_data=return_data)
        if not response.results:
            raise DecodeError(
                "server returned no per-item result for a one-record batch",
                operation="createRecords",
                status_code=response.meta.status_code,
                meta=response.meta,
            )
        return response.results[0], response

    def patch_records(
        self,
        items: Sequence[PatchRecordItem],
        *,
        validation: Optional[Any] = None,
        return_data: Optional[bool] = None,
    ) -> PatchRecordsResponse:
        """Update up to 10 records in one call.

        Operation ``patchRecords``, ``PATCH /v1/api/records``.

        Args:
            items: The updates, 1 to 10 of them.
            validation: ``strict`` validates everything first and writes
                nothing on any client error; the server defaults to ``lenient``.
            return_data: When ``False``, each result omits the updated record.

        Returns:
            The batch envelope, for 200, 207, 400 and 500 alike.

        Raises:
            InvalidRequestError: when a documented constraint is violated.
            APIError: on a 401, 403, 429, or a 400 that is not a batch envelope.
        """
        problems = self._problems()
        if problems is not None:
            check_enum(problems, "validation", validation, tuple(ValidationMode), "a validation mode")
            check_batch_size(problems, "items", len(items), MAX_PATCH_ITEMS)
            _validate_items(problems, items)
            self._finish(problems)

        query = _Query()
        query.add_enum("validation", validation)
        query.add_bool("returnData", return_data)
        data, meta = self._request(
            "patchRecords", "PATCH", "/records",
            query=query.pairs, body=[i.to_dict() for i in items], batch=True,
        )
        response = PatchRecordsResponse.from_dict(_as_dict(data))
        response.meta = meta
        return response

    # =======================================================================
    # Budgets
    # =======================================================================

    def list_budgets(
        self,
        *,
        limit: Optional[int] = None,
        offset: Optional[int] = None,
        agent_hints: Optional[bool] = None,
        with_total: Optional[bool] = None,
        ids: Optional[Sequence[str]] = None,
        name: Optional[Filter] = None,
        currency_code: Optional[str] = None,
        closed: Optional[bool] = None,
        type: Optional[Any] = None,
        label_id: Optional[str] = None,
        account_id: Optional[str] = None,
        category_ids: Optional[Sequence[str]] = None,
        start_date: Optional[Sequence[Filter]] = None,
        end_date: Optional[Sequence[Filter]] = None,
        created_at: Optional[Sequence[Filter]] = None,
        updated_at: Optional[Sequence[Filter]] = None,
        spending: Optional[str] = None,
    ) -> BudgetsPage:
        """Return one page of budgets, newest first by default.

        Operation ``getBudgets``, ``GET /v1/api/budgets``.

        Args:
            limit: Page size, 1-200.
            offset: Items to skip, for pagination.
            agent_hints: Ask for structured hints alongside the data.
            with_total: Ask for a total count.
            ids: Filter by identifier, up to 30.
            name: Filter by name; requires a text prefix.
            currency_code: Filter by ISO 4217 code.
            closed: ``True`` for closed only, ``False`` for open only.
            type: Filter by budget interval.
            label_id: Budgets carrying this label.
            account_id: Budgets tracking this account.
            category_ids: Budgets tracking these categories, up to 10.
            start_date: Filter by budget start; up to two conditions.
            end_date: Filter by budget end; up to two conditions.
            created_at: Filter by creation timestamp; up to two conditions.
            updated_at: Filter by last sync timestamp; up to two conditions.
            spending: Spending depth: ``none`` (the REST default), ``current``,
                or ``current+N`` for N past periods.

        Returns:
            The page, with ``meta`` carrying the response headers.

        Raises:
            InvalidRequestError: when a documented constraint is violated.
            APIError: on any non-2xx the server returns.
        """
        problems = self._problems()
        if problems is not None:
            check_limit(problems, limit)
            check_offset(problems, offset)
            check_id_list(problems, "id", ids, MAX_ID_FILTER_VALUES)
            check_id_list(problems, "categoryId", category_ids, MAX_CATEGORY_ID_FILTER_VALUES)
            validate_text_filter(problems, "name", name)
            check_optional_currency_code(problems, "currencyCode", currency_code)
            check_enum(problems, "type", type, tuple(BudgetType), "a budget type")
            validate_range_filters(problems, "startDate", start_date)
            validate_range_filters(problems, "endDate", end_date)
            validate_range_filters(problems, "createdAt", created_at)
            validate_range_filters(problems, "updatedAt", updated_at)
            check_spending_depth(problems, "spending", spending)
            self._finish(problems)

        query = _Query()
        query.add_int("limit", limit)
        query.add_int("offset", offset)
        query.add_bool("agentHints", agent_hints)
        query.add_bool("withTotal", with_total)
        query.add_csv("id", ids)
        query.add_filter("name", name)
        query.add_str("currencyCode", currency_code)
        query.add_bool("closed", closed)
        query.add_enum("type", type)
        query.add_str("labelId", label_id)
        query.add_str("accountId", account_id)
        query.add_csv("categoryId", category_ids)
        query.add_filters("startDate", start_date)
        query.add_filters("endDate", end_date)
        query.add_filters("createdAt", created_at)
        query.add_filters("updatedAt", updated_at)
        query.add_str("spending", spending)

        data, meta = self._request("getBudgets", "GET", "/budgets", query=query.pairs)
        page = BudgetsPage.from_dict(_as_dict(data))
        page.meta = meta
        return page

    def create_budget(self, request: CreateBudgetRequest) -> CreateBudgetResponse:
        """Create a budget.

        Operation ``createBudget``, ``POST /v1/api/budgets``. Answers 201.

        Args:
            request: The budget to create. ``BUDGET_CUSTOM`` requires both
                dates, and the type is immutable afterwards.

        Returns:
            The created budget and its write summary.

        Raises:
            InvalidRequestError: when a documented constraint is violated.
            APIError: on any non-2xx the server returns.
        """
        if self._validate:
            request.validate()
        data, meta = self._request("createBudget", "POST", "/budgets", body=request.to_dict())
        response = CreateBudgetResponse.from_dict(_as_dict(data))
        response.meta = meta
        return response

    def patch_budgets(
        self,
        items: Sequence[PatchBudgetItem],
        *,
        return_data: Optional[bool] = None,
    ) -> PatchBudgetsResponse:
        """Update up to 10 budgets in one call.

        Operation ``patchBudgets``, ``PATCH /v1/api/budgets``.

        Args:
            items: The updates, 1 to 10 of them.
            return_data: When ``False``, each result omits the updated budget.

        Returns:
            The batch envelope, for 200, 207, 400 and 500 alike.

        Raises:
            InvalidRequestError: when a documented constraint is violated.
            APIError: on a 401, 403, 429, or a 400 that is not a batch envelope.
        """
        self._validate_patch_batch(items)
        query = _Query()
        query.add_bool("returnData", return_data)
        data, meta = self._request(
            "patchBudgets", "PATCH", "/budgets",
            query=query.pairs, body=[i.to_dict() for i in items], batch=True,
        )
        response = PatchBudgetsResponse.from_dict(_as_dict(data))
        response.meta = meta
        return response

    # =======================================================================
    # Categories
    # =======================================================================

    def list_categories(
        self,
        *,
        limit: Optional[int] = None,
        offset: Optional[int] = None,
        agent_hints: Optional[bool] = None,
        with_total: Optional[bool] = None,
        ids: Optional[Sequence[str]] = None,
        name: Optional[Filter] = None,
        created_at: Optional[Sequence[Filter]] = None,
        updated_at: Optional[Sequence[Filter]] = None,
        custom_category: Optional[bool] = None,
        cardinality: Optional[Any] = None,
        archived: Optional[bool] = None,
        budget_id: Optional[str] = None,
        sort_by: Optional[Sort] = None,
    ) -> CategoriesPage:
        """Return one page of categories.

        Operation ``getCategories``, ``GET /v1/api/categories``.

        Args:
            limit: Page size, 1-200.
            offset: Items to skip, for pagination.
            agent_hints: Ask for structured hints alongside the data.
            with_total: Ask for a total count.
            ids: Filter by identifier, up to 30.
            name: Filter by name; requires a text prefix.
            created_at: Filter by creation timestamp; up to two conditions.
            updated_at: Filter by last sync timestamp; up to two conditions.
            custom_category: ``True`` for custom subcategories, ``False`` for
                system categories.
            cardinality: Filter by spending nature. A category with none of its
                own inherits the base category's, so filtering by an inherited
                value also matches the inheritors.
            archived: ``True`` for archived only, ``False`` for active only.
            budget_id: Only the categories a given budget tracks.
            sort_by: Order the page. Only ``createdAt`` and ``updatedAt`` sort.

        Returns:
            The page, with ``meta`` carrying the response headers.

        Raises:
            InvalidRequestError: when a documented constraint is violated.
            APIError: on any non-2xx the server returns.
        """
        problems = self._problems()
        if problems is not None:
            check_limit(problems, limit)
            check_offset(problems, offset)
            check_id_list(problems, "id", ids, MAX_ID_FILTER_VALUES)
            validate_text_filter(problems, "name", name)
            validate_range_filters(problems, "createdAt", created_at)
            validate_range_filters(problems, "updatedAt", updated_at)
            check_cardinality(problems, "cardinality", cardinality)
            validate_sort(problems, "sortBy", sort_by, COMMON_SORT_FIELDS)
            self._finish(problems)

        query = _Query()
        query.add_int("limit", limit)
        query.add_int("offset", offset)
        query.add_bool("agentHints", agent_hints)
        query.add_bool("withTotal", with_total)
        query.add_csv("id", ids)
        query.add_filter("name", name)
        query.add_filters("createdAt", created_at)
        query.add_filters("updatedAt", updated_at)
        query.add_bool("customCategory", custom_category)
        query.add_enum("cardinality", cardinality)
        query.add_bool("archived", archived)
        query.add_str("budgetId", budget_id)
        query.add_sort("sortBy", sort_by)

        data, meta = self._request("getCategories", "GET", "/categories", query=query.pairs)
        page = CategoriesPage.from_dict(_as_dict(data))
        page.meta = meta
        return page

    def create_subcategory(self, request: CreateSubcategoryRequest) -> CreateCategoryResponse:
        """Create a custom subcategory under a system category.

        Operation ``createCategory``, ``POST /v1/api/categories/custom``.
        Answers 201.

        Args:
            request: The subcategory to create.

        Returns:
            The created category and its write summary.

        Raises:
            InvalidRequestError: when a documented constraint is violated.
            APIError: on any non-2xx the server returns.
        """
        if self._validate:
            request.validate()
        data, meta = self._request(
            "createCategory", "POST", "/categories/custom", body=request.to_dict()
        )
        response = CreateCategoryResponse.from_dict(_as_dict(data))
        response.meta = meta
        return response

    def patch_categories(
        self,
        items: Sequence[PatchCategoryItem],
        *,
        return_data: Optional[bool] = None,
    ) -> PatchCategoriesResponse:
        """Update up to 10 categories in one call.

        Operation ``patchCategories``, ``PATCH /v1/api/categories``.

        Args:
            items: The updates, 1 to 10 of them.
            return_data: When ``False``, each result omits the updated category.

        Returns:
            The batch envelope, for 200, 207, 400 and 500 alike.

        Raises:
            InvalidRequestError: when a documented constraint is violated.
            APIError: on a 401, 403, 429, or a 400 that is not a batch envelope.
        """
        self._validate_patch_batch(items)
        query = _Query()
        query.add_bool("returnData", return_data)
        data, meta = self._request(
            "patchCategories", "PATCH", "/categories",
            query=query.pairs, body=[i.to_dict() for i in items], batch=True,
        )
        response = PatchCategoriesResponse.from_dict(_as_dict(data))
        response.meta = meta
        return response

    # =======================================================================
    # Labels
    # =======================================================================

    def list_labels(
        self,
        *,
        limit: Optional[int] = None,
        offset: Optional[int] = None,
        agent_hints: Optional[bool] = None,
        with_total: Optional[bool] = None,
        ids: Optional[Sequence[str]] = None,
        name: Optional[Filter] = None,
        created_at: Optional[Sequence[Filter]] = None,
        updated_at: Optional[Sequence[Filter]] = None,
        archived: Optional[bool] = None,
        record_id: Optional[str] = None,
        budget_id: Optional[str] = None,
        standing_order_id: Optional[str] = None,
        record_rule_id: Optional[str] = None,
        sort_by: Optional[Sort] = None,
    ) -> LabelsPage:
        """Return one page of labels, name-ascending by default.

        Operation ``getLabels``, ``GET /v1/api/labels``.

        Args:
            limit: Page size, 1-200.
            offset: Items to skip, for pagination.
            agent_hints: Ask for structured hints alongside the data.
            with_total: Ask for a total count.
            ids: Filter by identifier, up to 30.
            name: Filter by name; requires a text prefix.
            created_at: Filter by creation timestamp; up to two conditions.
            updated_at: Filter by last sync timestamp; up to two conditions.
            archived: ``True`` for archived only, ``False`` for active only.
            record_id: Only the labels a given record uses.
            budget_id: Only the labels a given budget uses.
            standing_order_id: Only the labels a given standing order uses.
            record_rule_id: Only the labels a given record rule uses.
            sort_by: Order the page. Only ``createdAt`` and ``updatedAt`` sort.

        Returns:
            The page, with ``meta`` carrying the response headers.

        Raises:
            InvalidRequestError: when a documented constraint is violated.
            APIError: on any non-2xx the server returns.
        """
        problems = self._problems()
        if problems is not None:
            check_limit(problems, limit)
            check_offset(problems, offset)
            check_id_list(problems, "id", ids, MAX_ID_FILTER_VALUES)
            validate_text_filter(problems, "name", name)
            validate_range_filters(problems, "createdAt", created_at)
            validate_range_filters(problems, "updatedAt", updated_at)
            validate_sort(problems, "sortBy", sort_by, COMMON_SORT_FIELDS)
            self._finish(problems)

        query = _Query()
        query.add_int("limit", limit)
        query.add_int("offset", offset)
        query.add_bool("agentHints", agent_hints)
        query.add_bool("withTotal", with_total)
        query.add_csv("id", ids)
        query.add_filter("name", name)
        query.add_filters("createdAt", created_at)
        query.add_filters("updatedAt", updated_at)
        query.add_bool("archived", archived)
        query.add_str("recordId", record_id)
        query.add_str("budgetId", budget_id)
        query.add_str("standingOrderId", standing_order_id)
        query.add_str("recordRuleId", record_rule_id)
        query.add_sort("sortBy", sort_by)

        data, meta = self._request("getLabels", "GET", "/labels", query=query.pairs)
        page = LabelsPage.from_dict(_as_dict(data))
        page.meta = meta
        return page

    def create_label(self, request: CreateLabelRequest) -> CreateLabelResponse:
        """Create a label.

        Operation ``createLabel``, ``POST /v1/api/labels``. Answers 201.

        Args:
            request: The label to create.

        Returns:
            The created label and its write summary.

        Raises:
            InvalidRequestError: when a documented constraint is violated.
            APIError: on any non-2xx the server returns.
        """
        if self._validate:
            request.validate()
        data, meta = self._request("createLabel", "POST", "/labels", body=request.to_dict())
        response = CreateLabelResponse.from_dict(_as_dict(data))
        response.meta = meta
        return response

    def patch_labels(
        self,
        items: Sequence[PatchLabelItem],
        *,
        return_data: Optional[bool] = None,
    ) -> PatchLabelsResponse:
        """Update up to 10 labels in one call.

        Operation ``patchLabels``, ``PATCH /v1/api/labels``.

        Args:
            items: The updates, 1 to 10 of them.
            return_data: When ``False``, each result omits the updated label.

        Returns:
            The batch envelope, for 200, 207, 400 and 500 alike.

        Raises:
            InvalidRequestError: when a documented constraint is violated.
            APIError: on a 401, 403, 429, or a 400 that is not a batch envelope.
        """
        self._validate_patch_batch(items)
        query = _Query()
        query.add_bool("returnData", return_data)
        data, meta = self._request(
            "patchLabels", "PATCH", "/labels",
            query=query.pairs, body=[i.to_dict() for i in items], batch=True,
        )
        response = PatchLabelsResponse.from_dict(_as_dict(data))
        response.meta = meta
        return response

    # =======================================================================
    # Goals, record rules, standing orders
    # =======================================================================

    def list_goals(
        self,
        *,
        limit: Optional[int] = None,
        offset: Optional[int] = None,
        agent_hints: Optional[bool] = None,
        with_total: Optional[bool] = None,
        ids: Optional[Sequence[str]] = None,
        name: Optional[Filter] = None,
        note: Optional[Filter] = None,
        created_at: Optional[Sequence[Filter]] = None,
        updated_at: Optional[Sequence[Filter]] = None,
    ) -> GoalsPage:
        """Return one page of financial goals, newest first by default.

        Operation ``getGoals``, ``GET /v1/api/goals``.

        Args:
            limit: Page size, 1-200.
            offset: Items to skip, for pagination.
            agent_hints: Ask for structured hints alongside the data.
            with_total: Ask for a total count.
            ids: Filter by identifier, up to 30.
            name: Filter by name; requires a text prefix.
            note: Filter by note; requires a text prefix.
            created_at: Filter by creation timestamp; up to two conditions.
            updated_at: Filter by last sync timestamp; up to two conditions.

        Returns:
            The page, with ``meta`` carrying the response headers.

        Raises:
            InvalidRequestError: when a documented constraint is violated.
            APIError: on any non-2xx the server returns.
        """
        problems = self._problems()
        if problems is not None:
            check_limit(problems, limit)
            check_offset(problems, offset)
            check_id_list(problems, "id", ids, MAX_ID_FILTER_VALUES)
            validate_text_filter(problems, "name", name)
            validate_text_filter(problems, "note", note)
            validate_range_filters(problems, "createdAt", created_at)
            validate_range_filters(problems, "updatedAt", updated_at)
            self._finish(problems)

        query = _Query()
        query.add_int("limit", limit)
        query.add_int("offset", offset)
        query.add_bool("agentHints", agent_hints)
        query.add_bool("withTotal", with_total)
        query.add_csv("id", ids)
        query.add_filter("name", name)
        query.add_filter("note", note)
        query.add_filters("createdAt", created_at)
        query.add_filters("updatedAt", updated_at)

        data, meta = self._request("getGoals", "GET", "/goals", query=query.pairs)
        page = GoalsPage.from_dict(_as_dict(data))
        page.meta = meta
        return page

    def list_record_rules(
        self,
        *,
        limit: Optional[int] = None,
        offset: Optional[int] = None,
        agent_hints: Optional[bool] = None,
        with_total: Optional[bool] = None,
        ids: Optional[Sequence[str]] = None,
        name: Optional[Filter] = None,
        label_id: Optional[str] = None,
        created_at: Optional[Sequence[Filter]] = None,
        updated_at: Optional[Sequence[Filter]] = None,
    ) -> RecordRulesPage:
        """Return one page of automatic categorisation rules, newest first.

        Operation ``getRecordRules``, ``GET /v1/api/record-rules``.

        Args:
            limit: Page size, 1-200.
            offset: Items to skip, for pagination.
            agent_hints: Ask for structured hints alongside the data.
            with_total: Ask for a total count.
            ids: Filter by identifier, up to 30.
            name: Filter by name; requires a text prefix.
            label_id: Rules that apply this label.
            created_at: Filter by creation timestamp; up to two conditions.
            updated_at: Filter by last sync timestamp; up to two conditions.

        Returns:
            The page, with ``meta`` carrying the response headers.

        Raises:
            InvalidRequestError: when a documented constraint is violated.
            APIError: on any non-2xx the server returns.
        """
        problems = self._problems()
        if problems is not None:
            check_limit(problems, limit)
            check_offset(problems, offset)
            check_id_list(problems, "id", ids, MAX_ID_FILTER_VALUES)
            validate_text_filter(problems, "name", name)
            validate_range_filters(problems, "createdAt", created_at)
            validate_range_filters(problems, "updatedAt", updated_at)
            self._finish(problems)

        query = _Query()
        query.add_int("limit", limit)
        query.add_int("offset", offset)
        query.add_bool("agentHints", agent_hints)
        query.add_bool("withTotal", with_total)
        query.add_csv("id", ids)
        query.add_filter("name", name)
        query.add_str("labelId", label_id)
        query.add_filters("createdAt", created_at)
        query.add_filters("updatedAt", updated_at)

        data, meta = self._request("getRecordRules", "GET", "/record-rules", query=query.pairs)
        page = RecordRulesPage.from_dict(_as_dict(data))
        page.meta = meta
        return page

    def list_standing_orders(
        self,
        *,
        limit: Optional[int] = None,
        offset: Optional[int] = None,
        agent_hints: Optional[bool] = None,
        with_total: Optional[bool] = None,
        ids: Optional[Sequence[str]] = None,
        name: Optional[Filter] = None,
        currency_code: Optional[str] = None,
        label_id: Optional[str] = None,
        created_at: Optional[Sequence[Filter]] = None,
        updated_at: Optional[Sequence[Filter]] = None,
    ) -> StandingOrdersPage:
        """Return one page of recurring payments, newest first by default.

        Operation ``getStandingOrders``, ``GET /v1/api/standing-orders``.

        Args:
            limit: Page size, 1-200.
            offset: Items to skip, for pagination.
            agent_hints: Ask for structured hints alongside the data.
            with_total: Ask for a total count.
            ids: Filter by identifier, up to 30.
            name: Filter by name; requires a text prefix.
            currency_code: Filter by ISO 4217 code.
            label_id: Standing orders carrying this label.
            created_at: Filter by creation timestamp; up to two conditions.
            updated_at: Filter by last sync timestamp; up to two conditions.

        Returns:
            The page, with ``meta`` carrying the response headers.

        Raises:
            InvalidRequestError: when a documented constraint is violated.
            APIError: on any non-2xx the server returns.
        """
        problems = self._problems()
        if problems is not None:
            check_limit(problems, limit)
            check_offset(problems, offset)
            check_id_list(problems, "id", ids, MAX_ID_FILTER_VALUES)
            validate_text_filter(problems, "name", name)
            check_optional_currency_code(problems, "currencyCode", currency_code)
            validate_range_filters(problems, "createdAt", created_at)
            validate_range_filters(problems, "updatedAt", updated_at)
            self._finish(problems)

        query = _Query()
        query.add_int("limit", limit)
        query.add_int("offset", offset)
        query.add_bool("agentHints", agent_hints)
        query.add_bool("withTotal", with_total)
        query.add_csv("id", ids)
        query.add_filter("name", name)
        query.add_str("currencyCode", currency_code)
        query.add_str("labelId", label_id)
        query.add_filters("createdAt", created_at)
        query.add_filters("updatedAt", updated_at)

        data, meta = self._request("getStandingOrders", "GET", "/standing-orders", query=query.pairs)
        page = StandingOrdersPage.from_dict(_as_dict(data))
        page.meta = meta
        return page

    def list_standing_order_items(
        self,
        *,
        limit: Optional[int] = None,
        offset: Optional[int] = None,
        agent_hints: Optional[bool] = None,
        with_total: Optional[bool] = None,
        standing_order_id: Optional[str] = None,
        original_date: Optional[Sequence[Filter]] = None,
        dismissed: Optional[bool] = None,
        record_id: Optional[str] = None,
        paid_date: Optional[Sequence[Filter]] = None,
    ) -> StandingOrderItemsPage:
        """Return one page of standing order occurrences, newest first.

        Each item is one period of a standing order and tracks its paid,
        unpaid or dismissed status.

        Operation ``getStandingOrderItems``,
        ``GET /v1/api/standing-orders/items``.

        Args:
            limit: Page size, 1-200.
            offset: Items to skip, for pagination.
            agent_hints: Ask for structured hints alongside the data.
            with_total: Ask for a total count.
            standing_order_id: Scope to one standing order. Omit for a
                cross-order query.
            original_date: Filter by scheduled date; up to two conditions.
            dismissed: ``True`` for dismissed only, ``False`` for live only.
            record_id: The occurrence a given record settled.
            paid_date: Filter by payment date; up to two conditions.

        Returns:
            The page, with ``meta`` carrying the response headers.

        Raises:
            InvalidRequestError: when a documented constraint is violated.
            APIError: on any non-2xx the server returns.
        """
        problems = self._problems()
        if problems is not None:
            check_limit(problems, limit)
            check_offset(problems, offset)
            validate_range_filters(problems, "originalDate", original_date)
            validate_range_filters(problems, "paidDate", paid_date)
            self._finish(problems)

        query = _Query()
        query.add_int("limit", limit)
        query.add_int("offset", offset)
        query.add_bool("agentHints", agent_hints)
        query.add_bool("withTotal", with_total)
        query.add_str("standingOrderId", standing_order_id)
        query.add_filters("originalDate", original_date)
        query.add_bool("dismissed", dismissed)
        query.add_str("recordId", record_id)
        query.add_filters("paidDate", paid_date)

        data, meta = self._request(
            "getStandingOrderItems", "GET", "/standing-orders/items", query=query.pairs
        )
        page = StandingOrderItemsPage.from_dict(_as_dict(data))
        page.meta = meta
        return page

    # =======================================================================
    # Monitoring
    # =======================================================================

    def api_usage(self, period: str) -> APIUsageStats:
        """Return REST API usage statistics for the authenticated client.

        Every successful request is counted except calls to this endpoint
        itself. Bucket ranges are half-open: ``from_`` inclusive, ``to``
        exclusive, and buckets with zero usage are omitted.

        Operation ``getApiUsageStats``, ``GET /v1/api/api-usage/stats``.

        Args:
            period: ``Xdays`` (max 366, daily buckets), ``Xweeks`` (max 53, ISO
                weeks starting Monday) or ``Xmonths`` (max 13).

        Returns:
            The usage report.

        Raises:
            InvalidRequestError: when the period is malformed or over its cap.
            APIError: on any non-2xx the server returns.
        """
        problems = self._problems()
        if problems is not None:
            check_usage_period(problems, "period", period)
            self._finish(problems)

        query = _Query()
        query.add_str("period", period)
        data, meta = self._request(
            "getApiUsageStats", "GET", "/api-usage/stats", query=query.pairs
        )
        stats = APIUsageStats.from_dict(_as_dict(data))
        stats.meta = meta
        return stats

    # =======================================================================
    # Data management
    # =======================================================================

    def delete(self, entity_type: Any, ids: Sequence[str]) -> DeleteResponse:
        """Delete up to 10 entities of one type.

        Operation ``deleteByType``, ``DELETE /v1/api/{type}``.

        Two rules deserve attention. For accounts, categories, labels and
        records, call :meth:`references` first — deleting something another
        entity points at is rejected. And a paired transfer deletes as a pair:
        both record IDs must be in the same call, or the delete is refused and
        the error names the missing leg.

        Delete is idempotent per ID, so a repeated ID succeeds while
        contributing nothing to ``summary.documents_written``.

        Args:
            entity_type: The collection to delete from.
            ids: The entities to delete, 1 to 10 of them.

        Returns:
            The batch envelope, for 200, 207, 400 and 500 alike.

        Raises:
            InvalidRequestError: when the type or the ID list is unusable.
            APIError: on a 401, 403, 429, or a 400 that is not a batch envelope.
        """
        problems = self._problems()
        if problems is not None:
            check_entity_type(problems, "type", entity_type, DELETABLE_ENTITY_TYPES)
            check_batch_size(problems, "ids", len(ids), MAX_DELETE_IDS)
            check_id_list(problems, "ids", ids, MAX_DELETE_IDS)
            self._finish(problems)

        data, meta = self._request(
            "deleteByType", "DELETE", f"/{_path_segment(entity_type)}",
            body={"ids": list(ids)}, batch=True,
        )
        response = DeleteResponse.from_dict(_as_dict(data))
        response.meta = meta
        return response

    def references(self, entity_type: Any, ids: Sequence[str]) -> ReferencesResult:
        """Report which entities point at the given ones.

        Operation ``getReferences``, ``GET /v1/api/{type}/references``.

        The response is a map keyed by entity ID. An ID that does not resolve
        gets an entry carrying ``error`` (``not found`` or ``type_mismatch``)
        instead of reference counts, which is why an unknown ID does not fail
        the whole call.

        Args:
            entity_type: The collection to check. Plural forms are canonical;
                the singular forms ``category``, ``record``, ``account`` and
                ``label`` are normalised. Budgets, standing orders and record
                rules are unsupported because nothing references them.
            ids: The entities to check, 1 to 20 of them.

        Returns:
            The references map.

        Raises:
            InvalidRequestError: when the type or the ID list is unusable.
            APIError: on any non-2xx the server returns.
        """
        normalized = normalize_entity_type(entity_type)
        problems = self._problems()
        if problems is not None:
            check_entity_type(problems, "type", normalized, REFERENCEABLE_ENTITY_TYPES)
            if not ids:
                problems.add("id", "at least one ID is required")
            check_id_list(problems, "id", ids, MAX_REFERENCE_IDS)
            self._finish(problems)

        query = _Query()
        query.add_csv("id", ids)
        data, meta = self._request(
            "getReferences", "GET", f"/{_path_segment(normalized)}/references", query=query.pairs
        )
        result = ReferencesResult.from_dict(_as_dict(data))
        result.meta = meta
        return result

    def delete_unreferenced(
        self, entity_type: Any, ids: Sequence[str]
    ) -> Tuple[Optional[DeleteResponse], Optional[ReferencesResult], Dict[str, str]]:
        """Delete only the IDs nothing points at — the safe delete the docs ask for.

        Types nothing can reference — budgets, standing orders and record rules
        — skip the check and go straight to the delete.

        Args:
            entity_type: The collection to delete from.
            ids: The candidates.

        Returns:
            A ``(response, references, skipped)`` triple. ``response`` is
            ``None`` when every ID was skipped and no delete was issued;
            ``references`` is ``None`` when no check was needed; ``skipped``
            maps each skipped ID to the reason it was skipped.

        Raises:
            InvalidRequestError: when the type or the ID list is unusable.
            APIError: on any non-2xx either call returns.
        """
        normalized = normalize_entity_type(entity_type)
        if normalized not in REFERENCEABLE_ENTITY_TYPES:
            return self.delete(normalized, ids), None, {}

        refs = self.references(normalized, ids)
        skipped: Dict[str, str] = {}
        deletable: List[str] = []
        for entity_id in ids:
            entry = refs.get(entity_id)
            if entry is None:
                skipped[entity_id] = "no reference entry returned for this id"
            elif entry.error:
                skipped[entity_id] = entry.error
            elif entry.total_references > 0:
                kinds = entry.referencing_kinds()
                skipped[entity_id] = (
                    f"referenced by {', '.join(kinds)}" if kinds else "referenced by other entities"
                )
            else:
                deletable.append(entity_id)

        if not deletable:
            return None, refs, skipped
        return self.delete(normalized, deletable), refs, skipped

    # =======================================================================
    # Auto-pagination
    # =======================================================================

    def iter_accounts(self, **kwargs: Any) -> Iterator[Any]:
        """Yield every account across all pages. See :meth:`list_accounts`."""
        return self._walk(self.list_accounts, "accounts", kwargs)

    def iter_records(self, **kwargs: Any) -> Iterator[Any]:
        """Yield every record across all pages.

        Remember the implicit window: without a ``record_date`` bound the
        server scopes the walk to the last three months.

        See :meth:`list_records` for the arguments.
        """
        return self._walk(self.list_records, "records", kwargs)

    def iter_budgets(self, **kwargs: Any) -> Iterator[Any]:
        """Yield every budget across all pages. See :meth:`list_budgets`."""
        return self._walk(self.list_budgets, "budgets", kwargs)

    def iter_categories(self, **kwargs: Any) -> Iterator[Any]:
        """Yield every category across all pages. See :meth:`list_categories`."""
        return self._walk(self.list_categories, "categories", kwargs)

    def iter_labels(self, **kwargs: Any) -> Iterator[Any]:
        """Yield every label across all pages. See :meth:`list_labels`."""
        return self._walk(self.list_labels, "labels", kwargs)

    def iter_goals(self, **kwargs: Any) -> Iterator[Any]:
        """Yield every goal across all pages. See :meth:`list_goals`."""
        return self._walk(self.list_goals, "goals", kwargs)

    def iter_record_rules(self, **kwargs: Any) -> Iterator[Any]:
        """Yield every record rule across all pages. See :meth:`list_record_rules`."""
        return self._walk(self.list_record_rules, "record_rules", kwargs)

    def iter_standing_orders(self, **kwargs: Any) -> Iterator[Any]:
        """Yield every standing order across all pages. See :meth:`list_standing_orders`."""
        return self._walk(self.list_standing_orders, "standing_orders", kwargs)

    def iter_standing_order_items(self, **kwargs: Any) -> Iterator[Any]:
        """Yield every standing order occurrence across all pages.

        See :meth:`list_standing_order_items`.
        """
        return self._walk(self.list_standing_order_items, "standing_order_items", kwargs)

    def _walk(self, lister: Callable[..., Any], attribute: str, kwargs: Dict[str, Any]) -> Iterator[Any]:
        """Drive an offset-paginated endpoint until the pages run out.

        The offset advances strictly, so a server that repeats or rewinds
        ``nextOffset`` ends the walk rather than looping.
        """
        params = dict(kwargs)
        offset = params.pop("offset", None) or 0
        for _ in range(_MAX_AUTO_PAGES):
            page = lister(offset=offset, **params)
            items = getattr(page, attribute)
            for item in items:
                yield item
            next_offset = page.pagination.next_offset
            if next_offset is None or next_offset <= offset or not items:
                return
            offset = next_offset

    # -- shared validation --------------------------------------------------

    def _validate_patch_batch(self, items: Sequence[Any]) -> None:
        """Check a patch batch's size and each item, reporting every problem."""
        if not self._validate:
            return
        problems = Problems()
        check_batch_size(problems, "items", len(items), MAX_PATCH_ITEMS)
        _validate_items(problems, items)
        problems.raise_if_any()


def _validate_items(problems: Problems, items: Sequence[Any]) -> None:
    """Validate each batch item and flag IDs that repeat within the batch."""
    seen: Dict[str, int] = {}
    for index, item in enumerate(items):
        item.validate(problems, f"items[{index}]")
        entity_id = getattr(item, "id", "")
        if entity_id:
            if entity_id in seen:
                problems.add(
                    f"items[{index}].id",
                    f"duplicates items[{seen[entity_id]}] ({entity_id!r}); "
                    "the API applies both in order",
                )
            else:
                seen[entity_id] = index


# ---------------------------------------------------------------------------
# Query building
# ---------------------------------------------------------------------------


class _Query:
    """Accumulates query parameters, skipping anything unset.

    Values are kept as ordered pairs rather than a dict, because a range
    parameter sends one entry per condition.
    """

    __slots__ = ("pairs",)

    def __init__(self) -> None:
        self.pairs: List[Tuple[str, str]] = []

    def add_int(self, key: str, value: Optional[int]) -> None:
        if value is not None:
            self.pairs.append((key, str(value)))

    def add_bool(self, key: str, value: Optional[bool]) -> None:
        if value is not None:
            self.pairs.append((key, "true" if value else "false"))

    def add_str(self, key: str, value: Optional[str]) -> None:
        if value:
            self.pairs.append((key, str(value)))

    def add_enum(self, key: str, value: Any) -> None:
        if value is not None and value != "":
            self.pairs.append((key, str(value)))

    def add_csv(self, key: str, values: Optional[Sequence[str]]) -> None:
        if values:
            self.pairs.append((key, ",".join(str(v) for v in values)))

    def add_enum_csv(self, key: str, values: Optional[Sequence[Any]]) -> None:
        if values:
            self.pairs.append((key, ",".join(str(v) for v in values)))

    def add_filter(self, key: str, value: Optional[Filter]) -> None:
        if value is not None:
            self.pairs.append((key, str(value)))

    def add_filters(self, key: str, values: Optional[Sequence[Filter]]) -> None:
        for item in values or ():
            self.pairs.append((key, str(item)))

    def add_sort(self, key: str, value: Optional[Sort]) -> None:
        if value is not None and value.field:
            self.pairs.append((key, str(value)))


# ---------------------------------------------------------------------------
# Response helpers
# ---------------------------------------------------------------------------


def _path_segment(value: Any) -> str:
    """Render an entity type for the URL path."""
    from urllib.parse import quote

    return quote(str(value), safe="")


def _as_dict(value: Any) -> Dict[str, Any]:
    """Coerce a decoded body to a mapping, so a surprising shape decodes empty."""
    return value if isinstance(value, dict) else {}


def _try_json(response: HTTPResponse) -> Any:
    """Parse a body as JSON, returning ``None`` when it is not."""
    if not response.body.strip():
        return None
    try:
        return response.json()
    except ValueError:
        return None


def _looks_like_batch_envelope(decoded: Any) -> bool:
    """Distinguish a per-item batch body from a plain ``ValidationError``.

    Both can arrive as 400: the batch body carries ``results`` or ``summary``,
    the validation error carries only ``error``.
    """
    return isinstance(decoded, dict) and ("results" in decoded or "summary" in decoded)


def _meta_from_response(response: HTTPResponse, attempt_no: int) -> ResponseMeta:
    """Read the documented headers off a response."""
    headers = response.headers
    return ResponseMeta(
        status_code=response.status,
        rate_limit_limit=_header_int(headers, "x-ratelimit-limit"),
        rate_limit_remaining=_header_int(headers, "x-ratelimit-remaining"),
        retry_after=parse_retry_after(headers.get("retry-after")),
        last_data_change_at=parse_datetime(headers.get("x-last-data-change-at")),
        last_data_change_rev=headers.get("x-last-data-change-rev", ""),
        sync_in_progress=_header_bool(headers, "x-sync-in-progress"),
        headers=dict(headers),
        attempts=attempt_no,
    )


def _header_int(headers: Mapping[str, str], key: str) -> Optional[int]:
    raw = headers.get(key)
    if raw is None:
        return None
    try:
        return int(raw.strip())
    except (ValueError, AttributeError):
        return None


def _header_bool(headers: Mapping[str, str], key: str) -> Optional[bool]:
    raw = headers.get(key)
    if raw is None:
        return None
    return raw.strip().lower() == "true"


def _extract_message(content_type: str, body: bytes) -> str:
    """Pull the message out of an error body.

    A JSON ``{"error": ...}`` wins; otherwise the raw text, which is what the
    ``text/plain`` 403 and a proxy's HTML error page carry.
    """
    text = body.decode("utf-8", errors="replace").strip()
    if not text:
        return ""
    if "json" in content_type.lower() or text.startswith("{"):
        try:
            decoded = json.loads(text)
        except ValueError:
            pass
        else:
            if isinstance(decoded, dict):
                for key in ("error", "message"):
                    value = decoded.get(key)
                    if isinstance(value, str) and value:
                        return value
    return text[:512]


def _api_error(operation: str, response: HTTPResponse, meta: ResponseMeta) -> APIError:
    """Build the exception describing a non-2xx response."""
    status = response.status
    message = _extract_message(response.content_type, response.body) or default_message_for_status(status)
    cls = error_for_status(status)
    return cls(
        message,
        operation=operation,
        status_code=status,
        body=response.text(),
        retry_after=meta.retry_after,
        meta=meta,
    )
