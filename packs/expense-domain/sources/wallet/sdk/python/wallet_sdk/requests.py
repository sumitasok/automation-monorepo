"""Request bodies, with the constraints the API documents enforced locally.

Each type carries a ``validate`` that collects every problem it finds, and a
``to_dict`` that emits exactly the JSON the endpoint expects — omitting any
field left unset, so an untouched field is never accidentally cleared.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import date, datetime, timedelta, timezone
from typing import Any, Dict, List, Optional, Sequence, Union

from ._convert import format_date_value, format_datetime
from ._problems import Problems
from .enums import (
    CREATABLE_ACCOUNT_TYPES,
    CREATABLE_BUDGET_TYPES,
    WRITABLE_RECORD_STATES,
    AccountType,
    BudgetType,
    Cardinality,
    ClearableRecordField,
    EntityType,
    PairingMode,
    RecordState,
    ResettableCategoryField,
    UniversalColor,
    names_of,
)
from .validation import (
    MAX_ADDRESS_LENGTH,
    MAX_BANK_ACCOUNT_NUMBER_LENGTH,
    MAX_COUNTER_PARTY_LENGTH,
    MAX_ID_FILTER_VALUES,
    MAX_LIMIT_OVERRIDES,
    MAX_NAME_LENGTH,
    MAX_NOTE_LENGTH,
    MAX_PERIOD_COUNT,
    MAX_PLACE_NAME_LENGTH,
    MAX_RECORD_DATE_FUTURE_SECONDS,
    MIN_PERIOD_COUNT,
    check_amount_value,
    check_cardinality,
    check_color,
    check_currency_code,
    check_id_list,
    check_max_length,
    check_period_format,
    check_required_id,
    check_required_text,
    check_optional_currency_code,
)

__all__ = [
    "IdSet",
    "replace_ids",
    "add_remove_ids",
    "clear_ids",
    "AmountInput",
    "CounterAmountInput",
    "PlaceInput",
    "TransferInput",
    "CreateAccountRequest",
    "PatchAccountItem",
    "CreateBudgetRequest",
    "LimitOverrideInput",
    "PatchBudgetItem",
    "CreateSubcategoryRequest",
    "PatchCategoryItem",
    "CreateLabelRequest",
    "PatchLabelItem",
    "CreateRecordRequest",
    "PatchRecordItem",
]


def _omit_none(data: Dict[str, Any]) -> Dict[str, Any]:
    """Drop keys whose value is ``None``, so an unset field is never sent."""
    return {k: v for k, v in data.items() if v is not None}


class IdSet:
    """The two shapes a patchable ID collection can take.

    A list replaces the whole set (an empty list clears it); an ``{add,
    remove}`` object applies a delta without touching the rest. Build one with
    :func:`replace_ids`, :func:`add_remove_ids` or :func:`clear_ids`.
    """

    __slots__ = ("_replace", "_add", "_remove", "_is_delta")

    def __init__(
        self,
        replace: Optional[Sequence[str]] = None,
        add: Optional[Sequence[str]] = None,
        remove: Optional[Sequence[str]] = None,
        is_delta: bool = False,
    ) -> None:
        self._replace = list(replace) if replace is not None else []
        self._add = list(add) if add is not None else None
        self._remove = list(remove) if remove is not None else None
        self._is_delta = is_delta

    @property
    def is_delta(self) -> bool:
        """Whether the value encodes an ``{add, remove}`` delta."""
        return self._is_delta

    @property
    def values(self) -> List[str]:
        """The replacement list, for the replace form."""
        return [] if self._is_delta else list(self._replace)

    @property
    def delta(self) -> Dict[str, List[str]]:
        """The add and remove lists, for the delta form."""
        if not self._is_delta:
            return {}
        out: Dict[str, List[str]] = {}
        if self._add is not None:
            out["add"] = list(self._add)
        if self._remove is not None:
            out["remove"] = list(self._remove)
        return out

    def to_json(self) -> Union[List[str], Dict[str, List[str]]]:
        """Emit whichever of the two documented shapes applies."""
        return self.delta if self._is_delta else list(self._replace)

    def validate(self, problems: Problems, field_name: str) -> None:
        """Check the delta form is not empty, which the API rejects."""
        if self._is_delta:
            if not self._add and not self._remove:
                problems.add(
                    field_name,
                    "an {add, remove} delta must carry at least one of add or remove",
                )
            check_id_list(problems, f"{field_name}.add", self._add, MAX_ID_FILTER_VALUES)
            check_id_list(problems, f"{field_name}.remove", self._remove, MAX_ID_FILTER_VALUES)
        else:
            check_id_list(problems, field_name, self._replace, MAX_ID_FILTER_VALUES)

    def __repr__(self) -> str:  # pragma: no cover - debugging aid
        return f"IdSet({self.to_json()!r})"

    def __eq__(self, other: object) -> bool:
        if not isinstance(other, IdSet):
            return NotImplemented
        return self.to_json() == other.to_json()


def replace_ids(ids: Optional[Sequence[str]]) -> IdSet:
    """Replace the whole set with ``ids``. An empty list clears it."""
    return IdSet(replace=ids or [])


def clear_ids() -> IdSet:
    """Empty the set."""
    return IdSet(replace=[])


def add_remove_ids(
    add: Optional[Sequence[str]] = None,
    remove: Optional[Sequence[str]] = None,
) -> IdSet:
    """Apply a delta, adding and removing without touching the rest."""
    return IdSet(add=add, remove=remove, is_delta=True)


def _id_set_json(value: Optional[IdSet]) -> Any:
    return None if value is None else value.to_json()


# ---------------------------------------------------------------------------
# Shared inputs
# ---------------------------------------------------------------------------


@dataclass
class AmountInput:
    """The amount on a create or patch.

    ``currency_code`` is optional and, when given, must match the account's
    currency.
    """

    #: The decimal amount. Negative is an expense, positive an income. It must
    #: not be zero.
    value: float
    #: Optional; defaults to the account's currency.
    currency_code: Optional[str] = None

    def validate(self, problems: Problems, prefix: str = "amount") -> None:
        """Check the amount."""
        check_amount_value(problems, f"{prefix}.value", self.value)
        check_optional_currency_code(problems, f"{prefix}.currencyCode", self.currency_code)

    def to_dict(self) -> Dict[str, Any]:
        """Emit the request payload."""
        return _omit_none({"value": self.value, "currencyCode": self.currency_code})


@dataclass
class CounterAmountInput:
    """The far-side amount of a cross-currency transfer.

    Both fields are required and the sign must oppose the record's amount.
    """

    #: The amount on the counter account, opposite in sign.
    value: float
    #: Must match the counter account's currency.
    currency_code: str

    def to_dict(self) -> Dict[str, Any]:
        """Emit the request payload."""
        return {"value": self.value, "currencyCode": self.currency_code}


@dataclass
class PlaceInput:
    """The location on a create or patch.

    On a patch it is a full replacement, never a merge.
    """

    #: The street or full address, at most 255 characters.
    address: Optional[str] = None
    #: The place name, at most 255 characters.
    name: Optional[str] = None
    #: The GPS latitude, -90 to 90.
    latitude: Optional[float] = None
    #: The GPS longitude, -180 to 180.
    longitude: Optional[float] = None

    def validate(self, problems: Problems, prefix: str = "place") -> None:
        """Check the place bounds."""
        check_max_length(problems, f"{prefix}.address", self.address, MAX_ADDRESS_LENGTH)
        check_max_length(problems, f"{prefix}.name", self.name, MAX_PLACE_NAME_LENGTH)
        if self.latitude is not None and not -90 <= self.latitude <= 90:
            problems.add(f"{prefix}.latitude", f"must be between -90 and 90, got {self.latitude}")
        if self.longitude is not None and not -180 <= self.longitude <= 180:
            problems.add(f"{prefix}.longitude", f"must be between -180 and 180, got {self.longitude}")

    def to_dict(self) -> Dict[str, Any]:
        """Emit the request payload."""
        return _omit_none(
            {
                "address": self.address,
                "name": self.name,
                "latitude": self.latitude,
                "longitude": self.longitude,
            }
        )


@dataclass
class TransferInput:
    """Turn a created record into an account movement.

    Omit it for a regular record.
    """

    #: Required. ``NEW`` creates a mirror on ``account_id``, ``EXISTING`` binds
    #: to ``record_id``, ``UNPAIRED`` records no mirror.
    pairing_mode: Any
    #: The target account, for ``NEW``.
    account_id: Optional[str] = None
    #: The record to bind to, for ``EXISTING``. The target must be on a
    #: different account, have the opposite amount sign, and not already be
    #: paired.
    record_id: Optional[str] = None
    #: The far-side amount of a cross-currency ``NEW``. Omit it to let the
    #: server convert via FX.
    counter_amount: Optional[CounterAmountInput] = None

    def validate(self, problems: Problems, record_amount: float, prefix: str = "transfer") -> None:
        """Check the mode-specific requirements the API documents."""
        mode = self.pairing_mode
        if mode is None or mode == "":
            problems.add(f"{prefix}.pairingMode", "is required when transfer is set")
        elif not PairingMode.is_valid(mode):
            problems.add(
                f"{prefix}.pairingMode",
                f"{str(mode)!r} is not a pairing mode; allowed: {names_of(tuple(PairingMode))}",
            )

        if mode == PairingMode.NEW:
            if not self.account_id or not str(self.account_id).strip():
                problems.add(f"{prefix}.accountId", f"is required when pairingMode is {PairingMode.NEW.value!r}")
            if self.record_id is not None:
                problems.add(
                    f"{prefix}.recordId",
                    f"is only valid when pairingMode is {PairingMode.EXISTING.value!r}",
                )
        elif mode == PairingMode.EXISTING:
            if not self.record_id or not str(self.record_id).strip():
                problems.add(
                    f"{prefix}.recordId",
                    f"is required when pairingMode is {PairingMode.EXISTING.value!r}",
                )
            if self.account_id is not None:
                problems.add(f"{prefix}.accountId", f"is only valid when pairingMode is {PairingMode.NEW.value!r}")
            if self.counter_amount is not None:
                problems.add(
                    f"{prefix}.counterAmount",
                    f"is only valid when pairingMode is {PairingMode.NEW.value!r}",
                )
        elif mode == PairingMode.UNPAIRED:
            if self.account_id is not None:
                problems.add(f"{prefix}.accountId", f"is only valid when pairingMode is {PairingMode.NEW.value!r}")
            if self.record_id is not None:
                problems.add(
                    f"{prefix}.recordId",
                    f"is only valid when pairingMode is {PairingMode.EXISTING.value!r}",
                )
            if self.counter_amount is not None:
                problems.add(
                    f"{prefix}.counterAmount",
                    f"is only valid when pairingMode is {PairingMode.NEW.value!r}",
                )

        if self.counter_amount is not None:
            check_amount_value(problems, f"{prefix}.counterAmount.value", self.counter_amount.value)
            check_currency_code(
                problems, f"{prefix}.counterAmount.currencyCode", self.counter_amount.currency_code
            )
            far = self.counter_amount.value
            if record_amount and far and (record_amount > 0) == (far > 0):
                problems.add(
                    f"{prefix}.counterAmount.value",
                    f"must have the opposite sign to the record amount ({record_amount}), got {far}",
                )

    def to_dict(self) -> Dict[str, Any]:
        """Emit the request payload."""
        return _omit_none(
            {
                "pairingMode": str(self.pairing_mode) if self.pairing_mode is not None else None,
                "accountId": self.account_id,
                "recordId": self.record_id,
                "counterAmount": self.counter_amount.to_dict() if self.counter_amount else None,
            }
        )


# ---------------------------------------------------------------------------
# Accounts
# ---------------------------------------------------------------------------


@dataclass
class CreateAccountRequest:
    """The body of ``POST /v1/api/accounts``."""

    #: The display name. Required, at most 80 characters.
    name: str
    #: The kind of account. Required, and one of the five creatable types.
    account_type: Any
    #: The 3-letter fiat currency. Required.
    currency_code: str
    #: The opening balance. Required — the API rejects the body without it, so
    #: send 0 for an empty account. Write-only: read it back as
    #: ``account.balance.initial``.
    initial_balance: float = 0.0
    #: The account number or IBAN, at most 80 characters.
    bank_account_number: Optional[str] = None
    #: A universal colour name. A random one is assigned if omitted.
    color: Optional[Any] = None
    #: Keep the account out of aggregate statistics.
    exclude_from_stats: Optional[bool] = None

    def validate(self) -> None:
        """Check every documented constraint. Raises on the first collection."""
        problems = Problems()
        check_required_text(problems, "name", self.name, MAX_NAME_LENGTH)
        if self.account_type is None or self.account_type == "":
            problems.add("accountType", "is required")
        elif self.account_type not in CREATABLE_ACCOUNT_TYPES:
            if AccountType.is_valid(self.account_type):
                problems.add(
                    "accountType",
                    f"{str(self.account_type)!r} cannot be created over REST; "
                    f"allowed: {names_of(CREATABLE_ACCOUNT_TYPES)}",
                )
            else:
                problems.add(
                    "accountType",
                    f"{str(self.account_type)!r} is not an account type; "
                    f"allowed: {names_of(CREATABLE_ACCOUNT_TYPES)}",
                )
        check_currency_code(problems, "currencyCode", self.currency_code)
        check_max_length(
            problems, "bankAccountNumber", self.bank_account_number, MAX_BANK_ACCOUNT_NUMBER_LENGTH
        )
        check_color(problems, "color", self.color)
        problems.raise_if_any()

    def to_dict(self) -> Dict[str, Any]:
        """Emit the request payload."""
        return _omit_none(
            {
                "name": self.name,
                "accountType": str(self.account_type) if self.account_type is not None else None,
                "currencyCode": self.currency_code,
                "initialBalance": self.initial_balance,
                "bankAccountNumber": self.bank_account_number,
                "color": str(self.color) if self.color is not None else None,
                "excludeFromStats": self.exclude_from_stats,
            }
        )


@dataclass
class PatchAccountItem:
    """One item of ``PATCH /v1/api/accounts``.

    Every field except ``id`` is optional; an omitted field is left unchanged.
    """

    #: The account to update. Required.
    id: str
    #: The new display name, at most 80 characters.
    name: Optional[str] = None
    #: Set or clear the archived flag.
    archived: Optional[bool] = None
    #: The new account number or IBAN. An empty string clears the value.
    bank_account_number: Optional[str] = None
    #: The new universal colour name.
    color: Optional[Any] = None
    #: Set or clear the statistics exclusion.
    exclude_from_stats: Optional[bool] = None
    #: The new opening balance. Write-only: read it back as
    #: ``account.balance.initial``.
    initial_balance: Optional[float] = None

    def validate(self, problems: Problems, prefix: str = "") -> None:
        """Check every documented constraint into ``problems``."""
        at = (lambda f: f"{prefix}.{f}" if prefix else f)
        check_required_id(problems, at("id"), self.id)
        if self.name is not None:
            check_required_text(problems, at("name"), self.name, MAX_NAME_LENGTH)
        # An empty bank account number is meaningful: it clears the value.
        check_max_length(
            problems, at("bankAccountNumber"), self.bank_account_number, MAX_BANK_ACCOUNT_NUMBER_LENGTH
        )
        check_color(problems, at("color"), self.color)
        if all(
            v is None
            for v in (
                self.name,
                self.archived,
                self.bank_account_number,
                self.color,
                self.exclude_from_stats,
                self.initial_balance,
            )
        ):
            problems.add(prefix, "no fields to update; set at least one field besides id")

    def to_dict(self) -> Dict[str, Any]:
        """Emit the request payload."""
        return _omit_none(
            {
                "id": self.id,
                "name": self.name,
                "archived": self.archived,
                "bankAccountNumber": self.bank_account_number,
                "color": str(self.color) if self.color is not None else None,
                "excludeFromStats": self.exclude_from_stats,
                "initialBalance": self.initial_balance,
            }
        )


# ---------------------------------------------------------------------------
# Budgets
# ---------------------------------------------------------------------------


@dataclass
class CreateBudgetRequest:
    """The body of ``POST /v1/api/budgets``."""

    #: The display name. Required.
    name: str
    #: The 3-letter currency. Required.
    currency_code: str
    #: The budget interval. Required and immutable after creation.
    #: ``BUDGET_CUSTOM`` additionally requires ``start_date`` and ``end_date``.
    type: Any
    #: The spending limit. Required, and greater than zero.
    limit: float
    #: The accounts to track.
    account_ids: Optional[Sequence[str]] = None
    #: The categories to track.
    category_ids: Optional[Sequence[str]] = None
    #: The labels to filter transactions by.
    label_ids: Optional[Sequence[str]] = None
    #: The first day. Required for ``BUDGET_CUSTOM``.
    start_date: Optional[Union[date, datetime, str]] = None
    #: The last day. Required for ``BUDGET_CUSTOM``.
    end_date: Optional[Union[date, datetime, str]] = None

    def validate(self) -> None:
        """Check every documented constraint."""
        problems = Problems()
        check_required_text(problems, "name", self.name, MAX_NAME_LENGTH)
        check_currency_code(problems, "currencyCode", self.currency_code)
        if self.type is None or self.type == "":
            problems.add("type", "is required")
        elif self.type not in CREATABLE_BUDGET_TYPES:
            problems.add(
                "type",
                f"{str(self.type)!r} cannot be created; allowed: {names_of(CREATABLE_BUDGET_TYPES)}",
            )
        if not isinstance(self.limit, (int, float)) or isinstance(self.limit, bool):
            problems.add("limit", f"must be a number, got {type(self.limit).__name__}")
        elif self.limit <= 0:
            problems.add("limit", f"must be greater than 0, got {self.limit}")

        if self.type == BudgetType.CUSTOM:
            if self.start_date is None:
                problems.add("startDate", f"is required for {BudgetType.CUSTOM.value}")
            if self.end_date is None:
                problems.add("endDate", f"is required for {BudgetType.CUSTOM.value}")
        _check_date_order(problems, self.start_date, self.end_date)

        check_id_list(problems, "accountIds", self.account_ids, MAX_ID_FILTER_VALUES)
        check_id_list(problems, "categoryIds", self.category_ids, MAX_ID_FILTER_VALUES)
        check_id_list(problems, "labelIds", self.label_ids, MAX_ID_FILTER_VALUES)
        problems.raise_if_any()

    def to_dict(self) -> Dict[str, Any]:
        """Emit the request payload."""
        return _omit_none(
            {
                "name": self.name,
                "currencyCode": self.currency_code,
                "type": str(self.type) if self.type is not None else None,
                "limit": self.limit,
                "accountIds": list(self.account_ids) if self.account_ids is not None else None,
                "categoryIds": list(self.category_ids) if self.category_ids is not None else None,
                "labelIds": list(self.label_ids) if self.label_ids is not None else None,
                "startDate": format_date_value(self.start_date),
                "endDate": format_date_value(self.end_date),
            }
        )


def _check_date_order(problems: Problems, start: Any, end: Any, prefix: str = "") -> None:
    """Reject an end date that precedes the start date."""
    if start is None or end is None:
        return
    left, right = format_date_value(start), format_date_value(end)
    if left and right and right < left:
        field_name = f"{prefix}.endDate" if prefix else "endDate"
        problems.add(field_name, f"must not be before startDate ({right} < {left})")


@dataclass
class LimitOverrideInput:
    """One entry of a budget's ``limitOverrides`` array.

    Overrides are EFFECTIVE-FROM: an entry sets the limit for its period and
    every period after it, until a later entry takes over. ``period_count``
    bounds it to N periods counting its own.
    """

    #: The starting period: ``YYYY-MM``, ``YYYY-Www`` or ``YYYY``. Required,
    #: and not earlier than the current period.
    period: str
    #: The new spending limit, greater than zero. Mutually exclusive with
    #: ``set_baseline``.
    limit: Optional[float] = None
    #: How many periods the override lasts, counting its own: 1 means this
    #: period only. Requires ``limit``; not allowed with ``set_baseline``.
    #: Omit for an open-ended override.
    period_count: Optional[int] = None
    #: Write an entry holding the budget's current limit, ending any earlier
    #: override from this period on. Mutually exclusive with ``limit`` and
    #: ``period_count``.
    set_baseline: Optional[bool] = None

    def validate(self, problems: Problems, prefix: str) -> None:
        """Check the mutual exclusions the API documents."""
        check_period_format(problems, f"{prefix}.period", self.period)
        baseline = bool(self.set_baseline)
        if self.limit is not None and baseline:
            problems.add(f"{prefix}.limit", "is mutually exclusive with setBaseline")
        elif self.limit is None and not baseline:
            problems.add(f"{prefix}.limit", "one of limit or setBaseline is required")
        if self.limit is not None and self.limit <= 0:
            problems.add(f"{prefix}.limit", f"must be greater than 0, got {self.limit}")
        if self.period_count is not None:
            if baseline:
                problems.add(f"{prefix}.periodCount", "is not allowed with setBaseline")
            if self.limit is None:
                problems.add(f"{prefix}.periodCount", "requires limit")
            if not MIN_PERIOD_COUNT <= self.period_count <= MAX_PERIOD_COUNT:
                problems.add(
                    f"{prefix}.periodCount",
                    f"must be between {MIN_PERIOD_COUNT} and {MAX_PERIOD_COUNT}, got {self.period_count}",
                )

    def to_dict(self) -> Dict[str, Any]:
        """Emit the request payload."""
        return _omit_none(
            {
                "period": self.period,
                "limit": self.limit,
                "periodCount": self.period_count,
                "setBaseline": self.set_baseline,
            }
        )


@dataclass
class PatchBudgetItem:
    """One item of ``PATCH /v1/api/budgets``."""

    #: The budget to update. Required.
    id: str
    #: The new display name, at most 80 characters.
    name: Optional[str] = None
    #: Replace or delta the tracked accounts.
    account_ids: Optional[IdSet] = None
    #: Replace or delta the tracked categories.
    category_ids: Optional[IdSet] = None
    #: Replace or delta the label filter.
    label_ids: Optional[IdSet] = None
    #: Close (``True``, setting ``closedDate`` to today UTC) or reopen
    #: (``False``, clearing it) the budget.
    closed: Optional[bool] = None
    #: The new first day. Only patchable on ``BUDGET_CUSTOM``.
    start_date: Optional[Union[date, datetime, str]] = None
    #: The new last day. Only patchable on ``BUDGET_CUSTOM``.
    end_date: Optional[Union[date, datetime, str]] = None
    #: Replaces ALL current and future overrides — one not re-sent is deleted,
    #: not merged. Past overrides are preserved. Not allowed on
    #: ``BUDGET_CUSTOM``. At most 50 entries.
    limit_overrides: Optional[Sequence[LimitOverrideInput]] = None
    #: Clears every override, past and future, and sets a uniform baseline
    #: limit. Works on all budget types and, combined with ``limit_overrides``,
    #: is applied first.
    reset_limit: Optional[float] = None

    def validate(self, problems: Problems, prefix: str = "") -> None:
        """Check every documented constraint into ``problems``.

        Two constraints cannot be checked here because they depend on server
        state: that ``start_date``, ``end_date`` and ``limit_overrides`` suit
        the budget's type, and that ``period`` is not in the past. The API
        enforces both.
        """
        at = (lambda f: f"{prefix}.{f}" if prefix else f)
        check_required_id(problems, at("id"), self.id)
        if self.name is not None:
            check_required_text(problems, at("name"), self.name, MAX_NAME_LENGTH)
        for field_name, value in (
            ("accountIds", self.account_ids),
            ("categoryIds", self.category_ids),
            ("labelIds", self.label_ids),
        ):
            if value is not None:
                value.validate(problems, at(field_name))

        if self.limit_overrides is not None:
            overrides = list(self.limit_overrides)
            if len(overrides) > MAX_LIMIT_OVERRIDES:
                problems.add(
                    at("limitOverrides"),
                    f"at most {MAX_LIMIT_OVERRIDES} entries allowed, got {len(overrides)}",
                )
            seen: Dict[str, int] = {}
            for index, override in enumerate(overrides):
                override.validate(problems, at(f"limitOverrides[{index}]"))
                if override.period:
                    if override.period in seen:
                        problems.add(
                            at(f"limitOverrides[{index}].period"),
                            f"duplicates limitOverrides[{seen[override.period]}] ({override.period!r})",
                        )
                    else:
                        seen[override.period] = index

        if self.reset_limit is not None and self.reset_limit <= 0:
            problems.add(at("resetLimit"), f"must be greater than 0, got {self.reset_limit}")
        _check_date_order(problems, self.start_date, self.end_date, prefix)

        if all(
            v is None
            for v in (
                self.name,
                self.account_ids,
                self.category_ids,
                self.label_ids,
                self.closed,
                self.start_date,
                self.end_date,
                self.limit_overrides,
                self.reset_limit,
            )
        ):
            problems.add(prefix, "no fields to update; set at least one field besides id")

    def to_dict(self) -> Dict[str, Any]:
        """Emit the request payload."""
        return _omit_none(
            {
                "id": self.id,
                "name": self.name,
                "accountIds": _id_set_json(self.account_ids),
                "categoryIds": _id_set_json(self.category_ids),
                "labelIds": _id_set_json(self.label_ids),
                "closed": self.closed,
                "startDate": format_date_value(self.start_date),
                "endDate": format_date_value(self.end_date),
                "limitOverrides": (
                    [o.to_dict() for o in self.limit_overrides]
                    if self.limit_overrides is not None
                    else None
                ),
                "resetLimit": self.reset_limit,
            }
        )


# ---------------------------------------------------------------------------
# Categories and labels
# ---------------------------------------------------------------------------


@dataclass
class CreateSubcategoryRequest:
    """The body of ``POST /v1/api/categories/custom``."""

    #: The subcategory name. Required, at most 80 characters.
    name: str
    #: The base category to nest under. Required.
    parent_id: str
    #: Override the parent's spending nature.
    cardinality: Optional[Any] = None
    #: A universal colour name. Defaults to the parent's colour.
    color: Optional[Any] = None

    def validate(self) -> None:
        """Check every documented constraint."""
        problems = Problems()
        check_required_text(problems, "name", self.name, MAX_NAME_LENGTH)
        check_required_id(problems, "parentId", self.parent_id)
        check_cardinality(problems, "cardinality", self.cardinality)
        check_color(problems, "color", self.color)
        problems.raise_if_any()

    def to_dict(self) -> Dict[str, Any]:
        """Emit the request payload."""
        return _omit_none(
            {
                "name": self.name,
                "parentId": self.parent_id,
                "cardinality": str(self.cardinality) if self.cardinality is not None else None,
                "color": str(self.color) if self.color is not None else None,
            }
        )


@dataclass
class PatchCategoryItem:
    """One item of ``PATCH /v1/api/categories``."""

    #: The category to update, system or custom. Required.
    id: str
    #: The new name; setting it also sets ``customName``. It cannot be combined
    #: with a reset of ``name``.
    name: Optional[str] = None
    #: The new spending nature; ``none`` clears it.
    cardinality: Optional[Any] = None
    #: The new universal colour name.
    color: Optional[Any] = None
    #: Restore fields to the base category's defaults. Base categories only: a
    #: custom subcategory has no defaults to restore. A field cannot be both
    #: set and reset in one call.
    reset: Optional[Sequence[Any]] = None

    def validate(self, problems: Problems, prefix: str = "") -> None:
        """Check every documented constraint into ``problems``."""
        at = (lambda f: f"{prefix}.{f}" if prefix else f)
        check_required_id(problems, at("id"), self.id)
        if self.name is not None:
            check_required_text(problems, at("name"), self.name, MAX_NAME_LENGTH)
        check_cardinality(problems, at("cardinality"), self.cardinality)
        check_color(problems, at("color"), self.color)

        conflicts = {
            ResettableCategoryField.NAME: self.name is not None,
            ResettableCategoryField.CARDINALITY: self.cardinality is not None,
            ResettableCategoryField.COLOR: self.color is not None,
        }
        seen = set()
        for index, value in enumerate(self.reset or ()):
            where = at(f"reset[{index}]")
            if not ResettableCategoryField.is_valid(value):
                problems.add(
                    where,
                    f"{str(value)!r} is not resettable; "
                    f"allowed: {names_of(tuple(ResettableCategoryField))}",
                )
                continue
            member = ResettableCategoryField(value)
            if member in seen:
                problems.add(where, f"{member.value!r} is listed twice")
            seen.add(member)
            if conflicts[member]:
                problems.add(where, f"{member.value!r} cannot be reset and set in the same item")

        if self.name is None and self.cardinality is None and self.color is None and not self.reset:
            problems.add(prefix, "no fields to update; set at least one field besides id")

    def to_dict(self) -> Dict[str, Any]:
        """Emit the request payload."""
        return _omit_none(
            {
                "id": self.id,
                "name": self.name,
                "cardinality": str(self.cardinality) if self.cardinality is not None else None,
                "color": str(self.color) if self.color is not None else None,
                "reset": [str(v) for v in self.reset] if self.reset is not None else None,
            }
        )


@dataclass
class CreateLabelRequest:
    """The body of ``POST /v1/api/labels``."""

    #: The label name. Required, at most 80 characters.
    name: str
    #: A universal colour name. A random one is assigned if omitted.
    color: Optional[Any] = None

    def validate(self) -> None:
        """Check every documented constraint."""
        problems = Problems()
        check_required_text(problems, "name", self.name, MAX_NAME_LENGTH)
        check_color(problems, "color", self.color)
        problems.raise_if_any()

    def to_dict(self) -> Dict[str, Any]:
        """Emit the request payload."""
        return _omit_none(
            {"name": self.name, "color": str(self.color) if self.color is not None else None}
        )


@dataclass
class PatchLabelItem:
    """One item of ``PATCH /v1/api/labels``."""

    #: The label to update. Required.
    id: str
    #: The new label name, at most 80 characters.
    name: Optional[str] = None
    #: Set or clear the archived flag.
    archived: Optional[bool] = None
    #: The new universal colour name.
    color: Optional[Any] = None

    def validate(self, problems: Problems, prefix: str = "") -> None:
        """Check every documented constraint into ``problems``."""
        at = (lambda f: f"{prefix}.{f}" if prefix else f)
        check_required_id(problems, at("id"), self.id)
        if self.name is not None:
            check_required_text(problems, at("name"), self.name, MAX_NAME_LENGTH)
        check_color(problems, at("color"), self.color)
        if self.name is None and self.archived is None and self.color is None:
            problems.add(prefix, "no fields to update; set at least one field besides id")

    def to_dict(self) -> Dict[str, Any]:
        """Emit the request payload."""
        return _omit_none(
            {
                "id": self.id,
                "name": self.name,
                "archived": self.archived,
                "color": str(self.color) if self.color is not None else None,
            }
        )


# ---------------------------------------------------------------------------
# Records
# ---------------------------------------------------------------------------


@dataclass
class CreateRecordRequest:
    """One item of ``POST /v1/api/records``."""

    #: The owning account. Required.
    account_id: str
    #: The transaction amount. Required, and must not be zero.
    amount: AmountInput
    #: The transaction date. Required, and not more than 24 hours in the
    #: future.
    #:
    #: Beware the write-side convention: a date-only value is STORED at
    #: 12:00:00Z, while a date-only bound in a filter means UTC midnight.
    record_date: Union[datetime, date, str] = None  # type: ignore[assignment]
    #: The category to assign.
    category_id: Optional[str] = None
    #: The payee or merchant, at most 255 characters.
    counter_party: Optional[str] = None
    #: The labels to attach.
    label_ids: Optional[Sequence[str]] = None
    #: The free-text note, at most 255 characters.
    note: Optional[str] = None
    #: The location.
    place: Optional[PlaceInput] = None
    #: The clearing state. The server defaults to ``cleared``.
    record_state: Optional[Any] = None
    #: Turn the record into an account movement.
    transfer: Optional[TransferInput] = None

    def validate(self, problems: Problems, prefix: str = "", now: Optional[datetime] = None) -> None:
        """Check every documented constraint into ``problems``.

        ``now`` bounds ``record_date``; it defaults to the current UTC time.
        """
        at = (lambda f: f"{prefix}.{f}" if prefix else f)
        moment = now or datetime.now(timezone.utc)
        if moment.tzinfo is None:
            moment = moment.replace(tzinfo=timezone.utc)

        check_required_id(problems, at("accountId"), self.account_id)
        if self.amount is None:
            problems.add(at("amount"), "is required")
        else:
            self.amount.validate(problems, at("amount"))

        _check_record_date(problems, at("recordDate"), self.record_date, moment)

        check_max_length(problems, at("counterParty"), self.counter_party, MAX_COUNTER_PARTY_LENGTH)
        check_max_length(problems, at("note"), self.note, MAX_NOTE_LENGTH)
        check_id_list(problems, at("labelIds"), self.label_ids, MAX_ID_FILTER_VALUES)
        if self.place is not None:
            self.place.validate(problems, at("place"))
        if self.record_state is not None and self.record_state not in WRITABLE_RECORD_STATES:
            problems.add(
                at("recordState"),
                f"{str(self.record_state)!r} cannot be set on a write; "
                f"allowed: {names_of(WRITABLE_RECORD_STATES)}",
            )
        if self.transfer is not None:
            amount_value = self.amount.value if self.amount is not None else 0.0
            self.transfer.validate(problems, amount_value, at("transfer"))

    def to_dict(self) -> Dict[str, Any]:
        """Emit the request payload."""
        return _omit_none(
            {
                "accountId": self.account_id,
                "amount": self.amount.to_dict() if self.amount is not None else None,
                "recordDate": _record_date_json(self.record_date),
                "categoryId": self.category_id,
                "counterParty": self.counter_party,
                "labelIds": list(self.label_ids) if self.label_ids is not None else None,
                "note": self.note,
                "place": self.place.to_dict() if self.place is not None else None,
                "recordState": str(self.record_state) if self.record_state is not None else None,
                "transfer": self.transfer.to_dict() if self.transfer is not None else None,
            }
        )


def _record_date_json(value: Any) -> Optional[str]:
    """Encode a record date, leaving a plain date string as the caller wrote it."""
    if value is None:
        return None
    if isinstance(value, str):
        return value
    if isinstance(value, datetime):
        return format_datetime(value)
    if isinstance(value, date):
        # A bare date is sent as a bare date: the API stores it at 12:00:00Z,
        # and rewriting it as midnight here would silently move it.
        return value.isoformat()
    raise TypeError(f"cannot encode {type(value).__name__} as a record date")


def _check_record_date(problems: Problems, field_name: str, value: Any, now: datetime) -> None:
    """Check a record date is present and not too far in the future."""
    if value is None or value == "":
        problems.add(field_name, "is required")
        return
    from ._convert import parse_datetime

    parsed = parse_datetime(value)
    if parsed is None:
        problems.add(field_name, f"must be a valid ISO 8601 timestamp, got {value!r}")
        return
    horizon = now + timedelta(seconds=MAX_RECORD_DATE_FUTURE_SECONDS)
    if parsed > horizon:
        problems.add(
            field_name,
            "must not be more than 24 hours in the future "
            f"(got {parsed.isoformat()}, now {now.isoformat()})",
        )


@dataclass
class PatchRecordItem:
    """One item of ``PATCH /v1/api/records``."""

    #: The record to update. Required.
    id: str
    #: Fields to reset to empty. A field cannot be both cleared and set.
    #: Clearing ``transfer`` cancels the transfer state and sets the category
    #: to Unknown.
    clear: Optional[Sequence[Any]] = None
    #: Move the record to another account. Neither account may be bank-synced
    #: and both must share a currency.
    account_id: Optional[str] = None
    #: The new amount. Setting it ALWAYS rewrites the record's currency to the
    #: account's currency; omitting it preserves the original.
    amount: Optional[AmountInput] = None
    #: The new category.
    category_id: Optional[str] = None
    #: The new payee or payer, at most 255 characters.
    counter_party: Optional[str] = None
    #: Replace or delta the label set.
    label_ids: Optional[IdSet] = None
    #: The new note, at most 255 characters.
    note: Optional[str] = None
    #: Fully replaces the location; there is no merge.
    place: Optional[PlaceInput] = None
    #: The new transaction date.
    record_date: Optional[Union[datetime, date, str]] = None
    #: The new clearing state.
    record_state: Optional[Any] = None

    def validate(self, problems: Problems, prefix: str = "") -> None:
        """Check every documented constraint into ``problems``."""
        at = (lambda f: f"{prefix}.{f}" if prefix else f)
        check_required_id(problems, at("id"), self.id)

        conflicts = {
            ClearableRecordField.CATEGORY_ID: self.category_id is not None,
            ClearableRecordField.LABEL_IDS: self.label_ids is not None,
            ClearableRecordField.NOTE: self.note is not None,
            ClearableRecordField.COUNTER_PARTY: self.counter_party is not None,
            ClearableRecordField.PLACE: self.place is not None,
            # There is no transfer setter on a patch, so it can never conflict.
            ClearableRecordField.TRANSFER: False,
        }
        seen = set()
        for index, value in enumerate(self.clear or ()):
            where = at(f"$clear[{index}]")
            if not ClearableRecordField.is_valid(value):
                problems.add(
                    where,
                    f"{str(value)!r} is not clearable; "
                    f"allowed: {names_of(tuple(ClearableRecordField))}",
                )
                continue
            member = ClearableRecordField(value)
            if member in seen:
                problems.add(where, f"{member.value!r} is listed twice")
            seen.add(member)
            if conflicts[member]:
                problems.add(where, f"{member.value!r} cannot be cleared and set in the same item")

        if self.amount is not None:
            self.amount.validate(problems, at("amount"))
        check_max_length(problems, at("counterParty"), self.counter_party, MAX_COUNTER_PARTY_LENGTH)
        check_max_length(problems, at("note"), self.note, MAX_NOTE_LENGTH)
        if self.label_ids is not None:
            self.label_ids.validate(problems, at("labelIds"))
        if self.place is not None:
            self.place.validate(problems, at("place"))
        if self.record_state is not None and self.record_state not in WRITABLE_RECORD_STATES:
            problems.add(
                at("recordState"),
                f"{str(self.record_state)!r} cannot be set on a write; "
                f"allowed: {names_of(WRITABLE_RECORD_STATES)}",
            )
        if self.record_date is not None:
            from ._convert import parse_datetime

            if parse_datetime(self.record_date) is None:
                problems.add(at("recordDate"), f"must be a valid ISO 8601 timestamp, got {self.record_date!r}")

        if not self.clear and all(
            v is None
            for v in (
                self.account_id,
                self.amount,
                self.category_id,
                self.counter_party,
                self.label_ids,
                self.note,
                self.place,
                self.record_date,
                self.record_state,
            )
        ):
            problems.add(prefix, "no fields to update; set at least one field besides id")

    def to_dict(self) -> Dict[str, Any]:
        """Emit the request payload."""
        return _omit_none(
            {
                "id": self.id,
                "$clear": [str(v) for v in self.clear] if self.clear else None,
                "accountId": self.account_id,
                "amount": self.amount.to_dict() if self.amount is not None else None,
                "categoryId": self.category_id,
                "counterParty": self.counter_party,
                "labelIds": _id_set_json(self.label_ids),
                "note": self.note,
                "place": self.place.to_dict() if self.place is not None else None,
                "recordDate": _record_date_json(self.record_date),
                "recordState": str(self.record_state) if self.record_state is not None else None,
            }
        )
