"""Every schema the Wallet API returns, as dataclasses.

Each model exposes ``from_dict`` and keeps the payload it was built from in
``raw``, so a field this SDK does not model yet is still reachable. Decoding is
deliberately forgiving: unknown keys are ignored, unknown enum members
round-trip verbatim, and a null where an object was expected becomes ``None``.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import date, datetime
from typing import Any, Dict, Iterator, List, Optional, Sequence

from ._convert import (
    as_bool,
    as_float,
    as_int,
    as_str,
    enum_or_raw,
    opt_bool,
    opt_float,
    opt_int,
    opt_str,
    parse_date,
    parse_datetime,
    str_list,
    sub_model,
    sub_models,
)
from .enums import (
    AccountType,
    BalanceMode,
    BudgetType,
    Cardinality,
    ErrorType,
    GoalState,
    Granularity,
    HintSeverity,
    HintType,
    RecordSource,
    RecordState,
    RecordType,
    Reminder,
)

__all__ = [
    "ResponseMeta",
    "Pagination",
    "AgentHintAction",
    "AgentHint",
    "AgentHints",
    "AmountWithCurrency",
    "ConvertedAmount",
    "StatDateRange",
    "AccountBalance",
    "AccountStats",
    "Account",
    "CategoryGroup",
    "Category",
    "CategoryEmbed",
    "Label",
    "LabelEmbed",
    "BudgetChangeEntry",
    "ExcludedBreakdown",
    "BudgetPeriodSpending",
    "BudgetSpending",
    "Budget",
    "RecordPhoto",
    "RecordPlace",
    "MirrorRecordEmbed",
    "TransferOutput",
    "Record",
    "RecordRule",
    "Goal",
    "StandingOrder",
    "StandingOrderItem",
    "ReferenceResult",
    "EntityReferences",
    "ReferencesResult",
    "APIUsageStatsEntry",
    "APIUsageStats",
    "BatchSummary",
    "PatchSummary",
    "ItemResult",
    "CreateRecordResult",
    "PatchRecordResult",
    "PatchAccountResult",
    "PatchBudgetResult",
    "PatchCategoryResult",
    "PatchLabelResult",
    "DeleteResult",
    "AccountsPage",
    "BudgetsPage",
    "CategoriesPage",
    "GoalsPage",
    "LabelsPage",
    "RecordRulesPage",
    "RecordsPage",
    "StandingOrdersPage",
    "StandingOrderItemsPage",
    "CreateAccountResponse",
    "CreateBudgetResponse",
    "CreateCategoryResponse",
    "CreateLabelResponse",
    "CreateRecordsResponse",
    "PatchAccountsResponse",
    "PatchBudgetsResponse",
    "PatchCategoriesResponse",
    "PatchLabelsResponse",
    "PatchRecordsResponse",
    "DeleteResponse",
]


# ---------------------------------------------------------------------------
# Response metadata
# ---------------------------------------------------------------------------


@dataclass
class ResponseMeta:
    """The documented response headers, plus the status code.

    Every envelope carries one, so a caller can watch the rate-limit budget or
    the data-change revision without dropping to a custom transport.
    """

    #: The HTTP status that produced this envelope. It matters on batch
    #: endpoints, where 200, 207, 400 and 500 all carry a body.
    status_code: int = 0
    #: ``X-RateLimit-Limit`` — the hourly request capacity.
    rate_limit_limit: Optional[int] = None
    #: ``X-RateLimit-Remaining`` — the capacity left in the current window.
    rate_limit_remaining: Optional[int] = None
    #: The parsed ``Retry-After`` header, in seconds.
    retry_after: Optional[float] = None
    #: ``X-Last-Data-Change-At``.
    last_data_change_at: Optional[datetime] = None
    #: ``X-Last-Data-Change-Rev``, a revision counter comparable across requests.
    last_data_change_rev: str = ""
    #: ``X-Sync-In-Progress``: when true the data is valid but a background sync
    #: is running and more changes may follow.
    sync_in_progress: Optional[bool] = None
    #: The full response header set, lower-cased keys.
    headers: Dict[str, str] = field(default_factory=dict)
    #: How many HTTP round trips, retries included, produced this response.
    attempts: int = 0

    @property
    def rate_limit_exhausted(self) -> bool:
        """Whether the remaining budget has reached zero."""
        return self.rate_limit_remaining is not None and self.rate_limit_remaining <= 0


@dataclass
class Pagination:
    """The shared envelope of every list endpoint."""

    #: The page size actually applied.
    limit: int = 0
    #: The starting position actually applied.
    offset: int = 0
    #: The offset of the next page. Absent on the last page.
    next_offset: Optional[int] = None
    #: The count of all matching items. Present only when ``with_total=True``.
    total: Optional[int] = None

    @property
    def has_more(self) -> bool:
        """Whether another page exists."""
        return self.next_offset is not None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "Pagination":
        """Build pagination from a response envelope."""
        return cls(
            limit=as_int(data, "limit"),
            offset=as_int(data, "offset"),
            next_offset=opt_int(data, "nextOffset"),
            total=opt_int(data, "total"),
        )


# ---------------------------------------------------------------------------
# Agent hints
# ---------------------------------------------------------------------------


@dataclass
class AgentHintAction:
    """The follow-up an instruction hint asks for."""

    #: Fetches the next page, for a ``pagination.has_more`` hint.
    url: str = ""

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "AgentHintAction":
        """Build the action from its payload."""
        return cls(url=as_str(data, "url"))


@dataclass
class AgentHint:
    """A structured hint, returned only when ``agent_hints=True`` was requested."""

    #: The dot-notation hint category.
    type: Any = None
    #: The nature of the hint.
    severity: Any = None
    #: The human-readable description.
    text: str = ""
    #: Hint-specific context; its shape varies with ``type``.
    data: Optional[Dict[str, Any]] = None
    #: Present only when ``severity`` is ``instruction``.
    action: Optional[AgentHintAction] = None
    #: The payload this hint was decoded from.
    raw: Dict[str, Any] = field(default_factory=dict)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "AgentHint":
        """Build a hint from its payload."""
        payload = data.get("data")
        return cls(
            type=enum_or_raw(HintType, data.get("type")),
            severity=enum_or_raw(HintSeverity, data.get("severity")),
            text=as_str(data, "text"),
            data=payload if isinstance(payload, dict) else None,
            action=sub_model(AgentHintAction.from_dict, data.get("action")),
            raw=data,
        )


class AgentHints(List[AgentHint]):
    """A list of hints with convenience lookups."""

    def by_type(self, hint_type: Any) -> List[AgentHint]:
        """The hints of the given type, in the order the API sent them."""
        return [h for h in self if h.type == hint_type]

    def first(self, hint_type: Any) -> Optional[AgentHint]:
        """The first hint of the given type, or ``None``."""
        for hint in self:
            if hint.type == hint_type:
                return hint
        return None

    @classmethod
    def from_list(cls, value: Any) -> "AgentHints":
        """Build hints from a response envelope's ``agentHints`` field."""
        return cls(sub_models(AgentHint.from_dict, value))


# ---------------------------------------------------------------------------
# Shared value objects
# ---------------------------------------------------------------------------


@dataclass
class AmountWithCurrency:
    """A decimal amount plus its ISO 4217 currency.

    Values are decimals, not minor units: 1234.56 means one thousand two
    hundred thirty-four and fifty-six hundredths.
    """

    #: The decimal amount; negative is an expense.
    value: float = 0.0
    #: The ISO 4217 code.
    currency_code: str = ""

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "AmountWithCurrency":
        """Build an amount from its payload."""
        return cls(value=as_float(data, "value"), currency_code=as_str(data, "currencyCode"))


@dataclass
class ConvertedAmount:
    """A record amount rendered in the currency ``convert_to`` asked for."""

    #: The target currency.
    currency_code: str = ""
    #: The converted amount.
    value: Optional[float] = None
    #: The exchange rate applied.
    ratio: Optional[float] = None
    #: ``recordCurrency:targetCurrency``.
    conversion_pair: str = ""
    #: Set when no rate was available; the other fields are then absent.
    error: str = ""

    @property
    def ok(self) -> bool:
        """Whether a rate was available."""
        return not self.error

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "ConvertedAmount":
        """Build a converted amount from its payload."""
        return cls(
            currency_code=as_str(data, "currencyCode"),
            value=opt_float(data, "value"),
            ratio=opt_float(data, "ratio"),
            conversion_pair=as_str(data, "conversionPair"),
            error=as_str(data, "error"),
        )


@dataclass
class StatDateRange:
    """The min/max pair inside :class:`AccountStats`."""

    #: The earliest value in the range.
    min: str = ""
    #: The latest value in the range.
    max: str = ""

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "StatDateRange":
        """Build a range from its payload."""
        return cls(min=as_str(data, "min"), max=as_str(data, "max"))


# ---------------------------------------------------------------------------
# Accounts
# ---------------------------------------------------------------------------


@dataclass
class AccountBalance:
    """The computed balance of an account, credit corrections applied. Read-only."""

    #: Which formula variant produced the numbers.
    balance_mode: Any = None
    #: Both variants of the formula for this mode.
    balance_mode_formula: str = ""
    #: The app's display preference for this account.
    balance_display_option: str = ""
    #: The account currency, repeated so amounts are self-describing.
    currency_code: str = ""
    #: The corrected balance.
    current_balance: Optional[float] = None
    #: The balance before credit corrections.
    raw_current_balance: Optional[float] = None
    #: The opening balance, i.e. what was sent as ``initial_balance``.
    initial: Optional[float] = None
    #: The account's credit ceiling.
    credit_limit: Optional[float] = None
    #: The balance owed on a credit account.
    credit_balance: Optional[float] = None
    #: The credit still available.
    available_credit: Optional[float] = None
    #: The exact computation used.
    formula: str = ""
    #: Why the balance could not be computed.
    error: str = ""
    #: The payload this balance was decoded from.
    raw: Dict[str, Any] = field(default_factory=dict)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "AccountBalance":
        """Build a balance from its payload."""
        return cls(
            balance_mode=enum_or_raw(BalanceMode, data.get("balanceMode")),
            balance_mode_formula=as_str(data, "balanceModeFormula"),
            balance_display_option=as_str(data, "balanceDisplayOption"),
            currency_code=as_str(data, "currencyCode"),
            current_balance=opt_float(data, "currentBalance"),
            raw_current_balance=opt_float(data, "rawCurrentBalance"),
            initial=opt_float(data, "initial"),
            credit_limit=opt_float(data, "creditLimit"),
            credit_balance=opt_float(data, "creditBalance"),
            available_credit=opt_float(data, "availableCredit"),
            formula=as_str(data, "formula"),
            error=as_str(data, "error"),
            raw=data,
        )


@dataclass
class AccountStats:
    """A summary of the records held by an account. Read-only."""

    #: The number of records on the account.
    record_count: int = 0
    #: The sum of expenses.
    total_expenses: Optional[float] = None
    #: The sum of incomes.
    total_incomes: Optional[float] = None
    #: The min/max transaction date.
    record_date: Optional[StatDateRange] = None
    #: The min/max record creation timestamp.
    created_at: Optional[StatDateRange] = None
    #: When the stats were last recomputed.
    last_updated_at: str = ""
    #: Why the stats could not be computed.
    error: str = ""
    #: When the stats computation failed.
    error_at: str = ""
    #: The payload these stats were decoded from.
    raw: Dict[str, Any] = field(default_factory=dict)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "AccountStats":
        """Build stats from their payload."""
        return cls(
            record_count=as_int(data, "recordCount"),
            total_expenses=opt_float(data, "totalExpenses"),
            total_incomes=opt_float(data, "totalIncomes"),
            record_date=sub_model(StatDateRange.from_dict, data.get("recordDate")),
            created_at=sub_model(StatDateRange.from_dict, data.get("createdAt")),
            last_updated_at=as_str(data, "lastUpdatedAt"),
            error=as_str(data, "error"),
            error_at=as_str(data, "errorAt"),
            raw=data,
        )


@dataclass
class Account:
    """A financial account."""

    #: The unique identifier.
    id: str = ""
    #: The display name.
    name: str = ""
    #: The kind of account.
    account_type: Any = None
    #: Whether the account is archived.
    archived: bool = False
    #: The computed balance. ``None`` when unavailable.
    balance: Optional[AccountBalance] = None
    #: The account number or IBAN.
    bank_account_number: str = ""
    #: A hex colour code. Write endpoints take a :class:`UniversalColor` name.
    color: str = ""
    #: The account currency, or ``""`` when unresolvable.
    currency_code: str = ""
    #: Whether the account is kept out of aggregate statistics.
    exclude_from_stats: bool = False
    #: Whether the account is fed by an automatic bank connection. Read-only.
    is_bank_sync: bool = False
    #: Whether the account holds portfolio data. Read-only.
    is_investment_account: bool = False
    #: A summary of the account's records. ``None`` when unavailable.
    record_stats: Optional[AccountStats] = None
    #: When the account was created.
    created_at: Optional[datetime] = None
    #: When the account was last synced into the API database.
    updated_at: Optional[datetime] = None
    #: The payload this account was decoded from.
    raw: Dict[str, Any] = field(default_factory=dict)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "Account":
        """Build an account from its payload."""
        return cls(
            id=as_str(data, "id"),
            name=as_str(data, "name"),
            account_type=enum_or_raw(AccountType, data.get("accountType")),
            archived=as_bool(data, "archived"),
            balance=sub_model(AccountBalance.from_dict, data.get("balance")),
            bank_account_number=as_str(data, "bankAccountNumber"),
            color=as_str(data, "color"),
            currency_code=as_str(data, "currencyCode"),
            exclude_from_stats=as_bool(data, "excludeFromStats"),
            is_bank_sync=as_bool(data, "isBankSync"),
            is_investment_account=as_bool(data, "isInvestmentAccount"),
            record_stats=sub_model(AccountStats.from_dict, data.get("recordStats")),
            created_at=parse_datetime(data.get("createdAt")),
            updated_at=parse_datetime(data.get("updatedAt")),
            raw=data,
        )


# ---------------------------------------------------------------------------
# Categories and labels
# ---------------------------------------------------------------------------


@dataclass
class CategoryGroup:
    """The root-level envelope group a category belongs to."""

    #: The stable group slug, e.g. ``food_and_drinks``.
    id: str = ""
    #: The group display name, e.g. ``Food & Drinks``.
    name: str = ""

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "CategoryGroup":
        """Build a group from its payload."""
        return cls(id=as_str(data, "id"), name=as_str(data, "name"))


@dataclass
class Category:
    """A system category or a custom subcategory."""

    #: The category identifier. System categories use deterministic IDs.
    id: str = ""
    #: The display name.
    name: str = ""
    #: Whether the category is archived.
    archived: bool = False
    #: The spending nature, possibly inherited from the base category.
    cardinality: Any = None
    #: A hex colour code.
    color: str = ""
    #: Whether this is a user-created subcategory.
    custom_category: bool = False
    #: Whether the name was overridden by the user.
    custom_name: bool = False
    #: Whether the category is offered in the apps.
    enabled: bool = False
    #: The envelope group. ``None`` when the API omits it.
    group: Optional[CategoryGroup] = None
    #: The base category's ID. Custom categories only.
    parent_id: str = ""
    #: The base category's display name. Custom categories only.
    parent_name: str = ""
    #: The envelope slug. System categories only.
    system_id: str = ""
    #: When the category was created. May be absent.
    created_at: Optional[datetime] = None
    #: When the category was last synced. May be absent.
    updated_at: Optional[datetime] = None
    #: The payload this category was decoded from.
    raw: Dict[str, Any] = field(default_factory=dict)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "Category":
        """Build a category from its payload."""
        return cls(
            id=as_str(data, "id"),
            name=as_str(data, "name"),
            archived=as_bool(data, "archived"),
            cardinality=enum_or_raw(Cardinality, data.get("cardinality")),
            color=as_str(data, "color"),
            custom_category=as_bool(data, "customCategory"),
            custom_name=as_bool(data, "customName"),
            enabled=as_bool(data, "enabled"),
            group=sub_model(CategoryGroup.from_dict, data.get("group")),
            parent_id=as_str(data, "parentId"),
            parent_name=as_str(data, "parentName"),
            system_id=as_str(data, "systemId"),
            created_at=parse_datetime(data.get("createdAt")),
            updated_at=parse_datetime(data.get("updatedAt")),
            raw=data,
        )


@dataclass
class CategoryEmbed:
    """The trimmed category attached to a record or record rule."""

    #: The category identifier.
    id: str = ""
    #: The display name.
    name: str = ""
    #: A hex colour code.
    color: str = ""
    #: The envelope group.
    group: Optional[CategoryGroup] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "CategoryEmbed":
        """Build an embedded category from its payload."""
        return cls(
            id=as_str(data, "id"),
            name=as_str(data, "name"),
            color=as_str(data, "color"),
            group=sub_model(CategoryGroup.from_dict, data.get("group")),
        )


@dataclass
class Label:
    """A hashtag applied to records, budgets, standing orders and rules."""

    #: The label identifier.
    id: str = ""
    #: The display name.
    name: str = ""
    #: Whether the label is archived.
    archived: bool = False
    #: A hex colour code.
    color: str = ""
    #: When the label was created.
    created_at: Optional[datetime] = None
    #: When the label was last synced.
    updated_at: Optional[datetime] = None
    #: The payload this label was decoded from.
    raw: Dict[str, Any] = field(default_factory=dict)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "Label":
        """Build a label from its payload."""
        return cls(
            id=as_str(data, "id"),
            name=as_str(data, "name"),
            archived=as_bool(data, "archived"),
            color=as_str(data, "color"),
            created_at=parse_datetime(data.get("createdAt")),
            updated_at=parse_datetime(data.get("updatedAt")),
            raw=data,
        )


@dataclass
class LabelEmbed:
    """The trimmed label attached to a record, rule or standing order."""

    #: The label identifier.
    id: str = ""
    #: The display name.
    name: str = ""
    #: Whether the label is archived.
    archived: bool = False
    #: A hex colour code.
    color: str = ""

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "LabelEmbed":
        """Build an embedded label from its payload."""
        return cls(
            id=as_str(data, "id"),
            name=as_str(data, "name"),
            archived=as_bool(data, "archived"),
            color=as_str(data, "color"),
        )


# ---------------------------------------------------------------------------
# Budgets
# ---------------------------------------------------------------------------


@dataclass
class BudgetChangeEntry:
    """One limit override on a budget."""

    #: The spending limit this entry imposes.
    limit: Optional[float] = None
    #: The period identifier: ``YYYY-MM``, ``YYYY-Www`` or ``YYYY``.
    period: str = ""
    #: The first day the entry applies to.
    period_start: str = ""
    #: How many periods the entry governs, counting its own. ``None`` is
    #: open-ended: nothing after it takes over.
    period_count: Optional[int] = None
    #: When the override was written.
    created_at: str = ""

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "BudgetChangeEntry":
        """Build an override entry from its payload."""
        return cls(
            limit=opt_float(data, "limit"),
            period=as_str(data, "period"),
            period_start=as_str(data, "periodStart"),
            period_count=opt_int(data, "periodCount"),
            created_at=as_str(data, "createdAt"),
        )


@dataclass
class ExcludedBreakdown:
    """The records a budget period deliberately ignored."""

    #: The total number of excluded records.
    total: int = 0
    #: The number of transfer records excluded.
    transfers: int = 0
    #: The number of debt records excluded.
    debts: int = 0
    #: The number of income-category records excluded.
    income_categories: int = 0
    #: The number of uncategorised records excluded.
    unknown_categories: int = 0
    #: The summed amount of the excluded records.
    total_amount_sum: Optional[float] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "ExcludedBreakdown":
        """Build a breakdown from its payload."""
        return cls(
            total=as_int(data, "total"),
            transfers=as_int(data, "transfers"),
            debts=as_int(data, "debts"),
            income_categories=as_int(data, "incomeCategories"),
            unknown_categories=as_int(data, "unknownCategories"),
            total_amount_sum=opt_float(data, "totalAmountSum"),
        )


@dataclass
class BudgetPeriodSpending:
    """One period's spending against a budget."""

    #: The period identifier.
    period: str = ""
    #: The limit in force for this period, overrides applied.
    effective_limit: Optional[float] = None
    #: The amount counted against the limit.
    spent: Optional[float] = None
    #: The amount left before the limit.
    remaining: Optional[float] = None
    #: The amount spent beyond the limit.
    overspent: Optional[float] = None
    #: The fraction of the limit consumed.
    progress: Optional[float] = None
    #: The number of records counted.
    record_count: int = 0
    #: The first instant of the period.
    period_start: Optional[str] = None
    #: The last instant of the period.
    period_end: Optional[str] = None
    #: Whether the period is still in progress.
    incomplete: bool = False
    #: The sum of expenses in the period.
    total_expenses: Optional[float] = None
    #: The sum of incomes in the period.
    total_incomes: Optional[float] = None
    #: The currencies folded into the totals.
    converted_currencies: List[str] = field(default_factory=list)
    #: The records left out of the totals.
    excluded: Optional[ExcludedBreakdown] = None
    #: Why the period could not be computed.
    error: str = ""
    #: The payload this period was decoded from.
    raw: Dict[str, Any] = field(default_factory=dict)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "BudgetPeriodSpending":
        """Build a period from its payload."""
        return cls(
            period=as_str(data, "period"),
            effective_limit=opt_float(data, "effectiveLimit"),
            spent=opt_float(data, "spent"),
            remaining=opt_float(data, "remaining"),
            overspent=opt_float(data, "overspent"),
            progress=opt_float(data, "progress"),
            record_count=as_int(data, "recordCount"),
            period_start=opt_str(data, "periodStart"),
            period_end=opt_str(data, "periodEnd"),
            incomplete=as_bool(data, "incomplete"),
            total_expenses=opt_float(data, "totalExpenses"),
            total_incomes=opt_float(data, "totalIncomes"),
            converted_currencies=str_list(data, "convertedCurrencies"),
            excluded=sub_model(ExcludedBreakdown.from_dict, data.get("excluded")),
            error=as_str(data, "error"),
            raw=data,
        )


@dataclass
class BudgetSpending:
    """The spending block returned when the ``spending`` parameter asks for it."""

    #: When the figures were calculated.
    computed_at: str = ""
    #: The period in progress. ``None`` when not requested or unavailable.
    current: Optional[BudgetPeriodSpending] = None
    #: The preceding periods, most recent first.
    past: List[BudgetPeriodSpending] = field(default_factory=list)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "BudgetSpending":
        """Build a spending block from its payload."""
        return cls(
            computed_at=as_str(data, "computedAt"),
            current=sub_model(BudgetPeriodSpending.from_dict, data.get("current")),
            past=sub_models(BudgetPeriodSpending.from_dict, data.get("past")),
        )


@dataclass
class Budget:
    """A spending limit over a repeating or custom interval."""

    #: The budget identifier.
    id: str = ""
    #: The display name.
    name: str = ""
    #: The budget interval.
    type: Any = None
    #: The budget currency.
    currency_code: str = ""
    #: The uniform spending limit. ``None`` when unset.
    limit: Optional[float] = None
    #: Whether the budget is closed. ``None`` when unset.
    closed: Optional[bool] = None
    #: The day the budget was closed. Present only when closed.
    closed_date: Optional[date] = None
    #: The first day of the budget.
    start_date: Optional[date] = None
    #: The last day of the budget.
    end_date: Optional[date] = None
    #: The accounts the budget tracks.
    account_ids: List[str] = field(default_factory=list)
    #: The categories the budget tracks.
    category_ids: List[str] = field(default_factory=list)
    #: The labels the budget filters on.
    label_ids: List[str] = field(default_factory=list)
    #: The overrides in force now or later, effective-from.
    limit_overrides: List[BudgetChangeEntry] = field(default_factory=list)
    #: Overrides a later entry has already superseded.
    past_limit_overrides: List[BudgetChangeEntry] = field(default_factory=list)
    #: The spending block, when requested.
    spending: Optional[BudgetSpending] = None
    #: When the budget was created.
    created_at: Optional[datetime] = None
    #: When the budget was last synced.
    updated_at: Optional[datetime] = None
    #: The payload this budget was decoded from.
    raw: Dict[str, Any] = field(default_factory=dict)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "Budget":
        """Build a budget from its payload."""
        return cls(
            id=as_str(data, "id"),
            name=as_str(data, "name"),
            type=enum_or_raw(BudgetType, data.get("type")),
            currency_code=as_str(data, "currencyCode"),
            limit=opt_float(data, "limit"),
            closed=opt_bool(data, "closed"),
            closed_date=parse_date(data.get("closedDate")),
            start_date=parse_date(data.get("startDate")),
            end_date=parse_date(data.get("endDate")),
            account_ids=str_list(data, "accountIds"),
            category_ids=str_list(data, "categoryIds"),
            label_ids=str_list(data, "labelIds"),
            limit_overrides=sub_models(BudgetChangeEntry.from_dict, data.get("limitOverrides")),
            past_limit_overrides=sub_models(BudgetChangeEntry.from_dict, data.get("pastLimitOverrides")),
            spending=sub_model(BudgetSpending.from_dict, data.get("spending")),
            created_at=parse_datetime(data.get("createdAt")),
            updated_at=parse_datetime(data.get("updatedAt")),
            raw=data,
        )


# ---------------------------------------------------------------------------
# Records
# ---------------------------------------------------------------------------


@dataclass
class RecordPhoto:
    """An image attached to a record."""

    #: A signed URL valid for a limited time.
    temporary_url: str = ""
    #: When the photo was attached.
    created_at: Optional[datetime] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "RecordPhoto":
        """Build a photo from its payload."""
        return cls(
            temporary_url=as_str(data, "temporaryUrl"),
            created_at=parse_datetime(data.get("createdAt")),
        )


@dataclass
class RecordPlace:
    """The location attached to a record."""

    #: The street or full address.
    address: str = ""
    #: The place name.
    name: str = ""
    #: The GPS latitude.
    latitude: Optional[float] = None
    #: The GPS longitude.
    longitude: Optional[float] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "RecordPlace":
        """Build a place from its payload."""
        return cls(
            address=as_str(data, "address"),
            name=as_str(data, "name"),
            latitude=opt_float(data, "latitude"),
            longitude=opt_float(data, "longitude"),
        )


@dataclass
class MirrorRecordEmbed:
    """The far side of a paired transfer."""

    #: The linked record's identifier.
    id: str = ""
    #: The linked record's account.
    account_id: str = ""
    #: The linked record's amount.
    amount: Optional[AmountWithCurrency] = None
    #: The linked record's counter party.
    counter_party: str = ""
    #: The linked record's note.
    note: str = ""

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "MirrorRecordEmbed":
        """Build a mirror record from its payload."""
        return cls(
            id=as_str(data, "id"),
            account_id=as_str(data, "accountId"),
            amount=sub_model(AmountWithCurrency.from_dict, data.get("amount")),
            counter_party=as_str(data, "counterParty"),
            note=as_str(data, "note"),
        )


@dataclass
class TransferOutput:
    """The read-only transfer state of a record. Never accepted as input."""

    #: The transfer identity: held by one record when unpaired, shared by two
    #: when paired.
    transfer_id: str = ""
    #: ``paired`` or ``unpaired``.
    type: str = ""
    #: The other leg, present only when paired.
    mirror_record: Optional[MirrorRecordEmbed] = None

    @property
    def paired(self) -> bool:
        """Whether a mirror record exists on the other side."""
        return self.mirror_record is not None and bool(self.mirror_record.id)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "TransferOutput":
        """Build a transfer state from its payload."""
        return cls(
            transfer_id=as_str(data, "transferId"),
            type=as_str(data, "type"),
            mirror_record=sub_model(MirrorRecordEmbed.from_dict, data.get("mirrorRecord")),
        )


@dataclass
class Record:
    """A financial transaction."""

    #: The record identifier.
    id: str = ""
    #: The owning account.
    account_id: str = ""
    #: The account's display name. Read-only.
    account_name: str = ""
    #: Whether the account is bank-synced.
    account_is_bank_sync: Optional[bool] = None
    #: The transaction amount in its original currency.
    amount: AmountWithCurrency = field(default_factory=AmountWithCurrency)
    #: Present only when ``convert_to`` was requested.
    converted_amount: Optional[ConvertedAmount] = None
    #: The embedded category.
    category: Optional[CategoryEmbed] = None
    #: The payee for expenses, the payer for income.
    counter_party: str = ""
    #: The labels attached to the record.
    labels: List[LabelEmbed] = field(default_factory=list)
    #: The free-text note.
    note: str = ""
    #: The images attached to the record.
    photos: List[RecordPhoto] = field(default_factory=list)
    #: The location attached to the record.
    place: Optional[RecordPlace] = None
    #: The transaction date.
    record_date: Optional[datetime] = None
    #: The clearing state.
    record_state: Any = None
    #: The direction, derived from the amount sign.
    record_type: Any = None
    #: The client that created the record. Read-only.
    source: Any = None
    #: The transfer state. ``None`` for regular records.
    transfer: Optional[TransferOutput] = None
    #: When the record was created.
    created_at: Optional[datetime] = None
    #: When the record was last synced.
    updated_at: Optional[datetime] = None
    #: The payload this record was decoded from.
    raw: Dict[str, Any] = field(default_factory=dict)

    @property
    def is_transfer(self) -> bool:
        """Whether the record participates in a transfer."""
        return self.transfer is not None and bool(self.transfer.transfer_id)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "Record":
        """Build a record from its payload."""
        amount = sub_model(AmountWithCurrency.from_dict, data.get("amount"))
        return cls(
            id=as_str(data, "id"),
            account_id=as_str(data, "accountId"),
            account_name=as_str(data, "accountName"),
            account_is_bank_sync=opt_bool(data, "accountIsBankSync"),
            amount=amount if amount is not None else AmountWithCurrency(),
            converted_amount=sub_model(ConvertedAmount.from_dict, data.get("convertedAmount")),
            category=sub_model(CategoryEmbed.from_dict, data.get("category")),
            counter_party=as_str(data, "counterParty"),
            labels=sub_models(LabelEmbed.from_dict, data.get("labels")),
            note=as_str(data, "note"),
            photos=sub_models(RecordPhoto.from_dict, data.get("photos")),
            place=sub_model(RecordPlace.from_dict, data.get("place")),
            record_date=parse_datetime(data.get("recordDate")),
            record_state=enum_or_raw(RecordState, data.get("recordState")),
            record_type=enum_or_raw(RecordType, data.get("recordType")),
            source=enum_or_raw(RecordSource, data.get("source")),
            transfer=sub_model(TransferOutput.from_dict, data.get("transfer")),
            created_at=parse_datetime(data.get("createdAt")),
            updated_at=parse_datetime(data.get("updatedAt")),
            raw=data,
        )


@dataclass
class RecordRule:
    """An automatic categorisation rule."""

    #: The rule identifier.
    id: str = ""
    #: The display name.
    name: str = ""
    #: The category the rule assigns.
    category: Optional[CategoryEmbed] = None
    #: Scopes the rule to records leaving this account.
    from_account_id: str = ""
    #: Scopes the rule to records entering this account.
    to_account_id: str = ""
    #: The strings the rule matches on.
    keywords: List[str] = field(default_factory=list)
    #: The labels the rule applies.
    labels: List[LabelEmbed] = field(default_factory=list)
    #: When the rule was created.
    created_at: Optional[datetime] = None
    #: When the rule was last synced.
    updated_at: Optional[datetime] = None
    #: The payload this rule was decoded from.
    raw: Dict[str, Any] = field(default_factory=dict)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "RecordRule":
        """Build a rule from its payload."""
        return cls(
            id=as_str(data, "id"),
            name=as_str(data, "name"),
            category=sub_model(CategoryEmbed.from_dict, data.get("category")),
            from_account_id=as_str(data, "fromAccountId"),
            to_account_id=as_str(data, "toAccountId"),
            keywords=str_list(data, "keywords"),
            labels=sub_models(LabelEmbed.from_dict, data.get("labels")),
            created_at=parse_datetime(data.get("createdAt")),
            updated_at=parse_datetime(data.get("updatedAt")),
            raw=data,
        )


# ---------------------------------------------------------------------------
# Goals and standing orders
# ---------------------------------------------------------------------------


@dataclass
class Goal:
    """A savings target."""

    #: The goal identifier.
    id: str = ""
    #: The display name.
    name: str = ""
    #: The free-text note.
    note: str = ""
    #: A hex colour code.
    color: str = ""
    #: The target completion date.
    desired_date: Optional[datetime] = None
    #: The starting amount, in the profile's base currency.
    initial_amount: Optional[AmountWithCurrency] = None
    #: The goal amount, in the profile's base currency.
    target_amount: Optional[AmountWithCurrency] = None
    #: The lifecycle state.
    state: Any = None
    #: When the state last changed.
    state_updated_at: Optional[datetime] = None
    #: When the goal was created.
    created_at: Optional[datetime] = None
    #: When the goal was last synced.
    updated_at: Optional[datetime] = None
    #: The payload this goal was decoded from.
    raw: Dict[str, Any] = field(default_factory=dict)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "Goal":
        """Build a goal from its payload."""
        return cls(
            id=as_str(data, "id"),
            name=as_str(data, "name"),
            note=as_str(data, "note"),
            color=as_str(data, "color"),
            desired_date=parse_datetime(data.get("desiredDate")),
            initial_amount=sub_model(AmountWithCurrency.from_dict, data.get("initialAmount")),
            target_amount=sub_model(AmountWithCurrency.from_dict, data.get("targetAmount")),
            state=enum_or_raw(GoalState, data.get("state")),
            state_updated_at=parse_datetime(data.get("stateUpdatedAt")),
            created_at=parse_datetime(data.get("createdAt")),
            updated_at=parse_datetime(data.get("updatedAt")),
            raw=data,
        )


@dataclass
class StandingOrder:
    """A recurring payment."""

    #: The standing order identifier.
    id: str = ""
    #: The display name.
    name: str = ""
    #: The account the payment moves through.
    account_id: str = ""
    #: The payment amount.
    amount: Optional[float] = None
    #: The category assigned to generated records.
    category_id: str = ""
    #: The payee or payer.
    counter_party: str = ""
    #: The payment currency.
    currency_code: str = ""
    #: The next due date.
    due_date: Optional[datetime] = None
    #: Whether the due-date notification is on.
    due_date_notification_enabled: bool = False
    #: Whether the three-day notification is on.
    three_days_before_notification_enabled: bool = False
    #: The date generation starts from.
    generate_from_date: str = ""
    #: The labels applied to generated records.
    labels: List[LabelEmbed] = field(default_factory=list)
    #: Whether the user must confirm each occurrence.
    manual_payment: bool = False
    #: The free-text note.
    note: str = ""
    #: How many occurrences are tracked.
    payment_count: int = 0
    #: The schedule in RRULE format.
    recurrence_rule: str = ""
    #: The notification lead time.
    reminder: Any = None
    #: The direction of the payment.
    type: Any = None
    #: When the standing order was created.
    created_at: Optional[datetime] = None
    #: When the standing order was last synced.
    updated_at: Optional[datetime] = None
    #: The payload this standing order was decoded from.
    raw: Dict[str, Any] = field(default_factory=dict)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "StandingOrder":
        """Build a standing order from its payload."""
        return cls(
            id=as_str(data, "id"),
            name=as_str(data, "name"),
            account_id=as_str(data, "accountId"),
            amount=opt_float(data, "amount"),
            category_id=as_str(data, "categoryId"),
            counter_party=as_str(data, "counterParty"),
            currency_code=as_str(data, "currencyCode"),
            due_date=parse_datetime(data.get("dueDate")),
            due_date_notification_enabled=as_bool(data, "dueDateNotificationEnabled"),
            three_days_before_notification_enabled=as_bool(data, "threeDaysBeforeNotificationEnabled"),
            generate_from_date=as_str(data, "generateFromDate"),
            labels=sub_models(LabelEmbed.from_dict, data.get("labels")),
            manual_payment=as_bool(data, "manualPayment"),
            note=as_str(data, "note"),
            payment_count=as_int(data, "paymentCount"),
            recurrence_rule=as_str(data, "recurrenceRule"),
            reminder=enum_or_raw(Reminder, data.get("reminder")),
            type=enum_or_raw(RecordType, data.get("type")),
            created_at=parse_datetime(data.get("createdAt")),
            updated_at=parse_datetime(data.get("updatedAt")),
            raw=data,
        )


@dataclass
class StandingOrderItem:
    """One occurrence of a standing order."""

    #: The item identifier.
    id: str = ""
    #: The owning standing order.
    standing_order_id: str = ""
    #: The date the schedule produced.
    original_date: Optional[datetime] = None
    #: The date after weekend or holiday alignment.
    aligned_date: Optional[datetime] = None
    #: When the occurrence was paid.
    paid_date: Optional[datetime] = None
    #: When the occurrence was marked paid in the app.
    paid_from_app_date: Optional[datetime] = None
    #: Whether the occurrence was dismissed.
    dismissed: bool = False
    #: The records that settled this occurrence.
    record_ids: List[str] = field(default_factory=list)
    #: The payload this item was decoded from.
    raw: Dict[str, Any] = field(default_factory=dict)

    @property
    def paid(self) -> bool:
        """Whether the occurrence has been settled."""
        return self.paid_date is not None or bool(self.record_ids)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "StandingOrderItem":
        """Build an occurrence from its payload."""
        return cls(
            id=as_str(data, "id"),
            standing_order_id=as_str(data, "standingOrderId"),
            original_date=parse_datetime(data.get("originalDate")),
            aligned_date=parse_datetime(data.get("alignedDate")),
            paid_date=parse_datetime(data.get("paidDate")),
            paid_from_app_date=parse_datetime(data.get("paidFromAppDate")),
            dismissed=as_bool(data, "dismissed"),
            record_ids=str_list(data, "recordIds"),
            raw=data,
        )


# ---------------------------------------------------------------------------
# References
# ---------------------------------------------------------------------------


@dataclass
class ReferenceResult:
    """The entities of one kind that point at a target."""

    #: The field carrying the reference, e.g. ``categoryId``.
    field_name: str = ""
    #: The referencing entity IDs, up to ``limit`` of them.
    ids: List[str] = field(default_factory=list)
    #: The full count of references.
    total: int = 0
    #: The maximum number of IDs returned.
    limit: int = 0
    #: Whether ``total`` exceeds ``limit``.
    has_more: bool = False

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "ReferenceResult":
        """Build a reference result from its payload."""
        return cls(
            field_name=as_str(data, "field"),
            ids=str_list(data, "ids"),
            total=as_int(data, "total"),
            limit=as_int(data, "limit"),
            has_more=as_bool(data, "hasMore"),
        )


@dataclass
class EntityReferences:
    """One entry of the references map.

    It carries either the entities pointing at the target, or an error when the
    ID could not be resolved.
    """

    #: The records referencing the target.
    records: Optional[ReferenceResult] = None
    #: The standing orders referencing the target.
    standing_orders: Optional[ReferenceResult] = None
    #: The budgets referencing the target.
    budgets: Optional[ReferenceResult] = None
    #: The record rules referencing the target.
    record_rules: Optional[ReferenceResult] = None
    #: References the API reports under ``other``, such as Board templates.
    other: Optional[ReferenceResult] = None
    #: ``not found`` or ``type_mismatch`` when the ID did not resolve.
    error: str = ""
    #: The type the ID really belongs to, on a ``type_mismatch``.
    actual_type: str = ""
    #: A human-readable explanation accompanying a ``type_mismatch``.
    message: str = ""
    #: The payload this entry was decoded from.
    raw: Dict[str, Any] = field(default_factory=dict)

    @property
    def resolved(self) -> bool:
        """Whether the ID was found and is of the requested type."""
        return not self.error

    @property
    def total_references(self) -> int:
        """The sum of every reference count. Zero means safe to delete."""
        parts = (self.records, self.standing_orders, self.budgets, self.record_rules, self.other)
        return sum(p.total for p in parts if p is not None)

    def referencing_kinds(self) -> List[str]:
        """The names of the reference kinds with a non-zero count."""
        pairs = (
            ("records", self.records),
            ("standingOrders", self.standing_orders),
            ("budgets", self.budgets),
            ("recordRules", self.record_rules),
            ("other", self.other),
        )
        return [name for name, result in pairs if result is not None and result.total > 0]

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "EntityReferences":
        """Build a references entry from its payload."""
        return cls(
            records=sub_model(ReferenceResult.from_dict, data.get("records")),
            standing_orders=sub_model(ReferenceResult.from_dict, data.get("standingOrders")),
            budgets=sub_model(ReferenceResult.from_dict, data.get("budgets")),
            record_rules=sub_model(ReferenceResult.from_dict, data.get("recordRules")),
            other=sub_model(ReferenceResult.from_dict, data.get("other")),
            error=as_str(data, "error"),
            actual_type=as_str(data, "actualType"),
            message=as_str(data, "message"),
            raw=data,
        )


@dataclass
class ReferencesResult:
    """The ``GET /{type}/references`` response: a map from entity ID to references."""

    #: Each requested ID mapped to its references or its error.
    entries: Dict[str, EntityReferences] = field(default_factory=dict)
    #: The response metadata.
    meta: ResponseMeta = field(default_factory=ResponseMeta)

    def __getitem__(self, entity_id: str) -> EntityReferences:
        return self.entries[entity_id]

    def __contains__(self, entity_id: object) -> bool:
        return entity_id in self.entries

    def __iter__(self) -> Iterator[str]:
        return iter(self.entries)

    def __len__(self) -> int:
        return len(self.entries)

    def get(self, entity_id: str) -> Optional[EntityReferences]:
        """The entry for one ID, or ``None`` when the server returned none."""
        return self.entries.get(entity_id)

    def unresolved(self) -> Dict[str, str]:
        """The requested IDs the server could not resolve, mapped to its reason."""
        return {k: v.error for k, v in self.entries.items() if v.error}

    def safe_to_delete(self) -> List[str]:
        """The IDs that resolved and have no references, so a delete would pass."""
        return sorted(k for k, v in self.entries.items() if v.resolved and v.total_references == 0)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "ReferencesResult":
        """Build the references map from its payload."""
        entries = {
            key: EntityReferences.from_dict(value)
            for key, value in data.items()
            if isinstance(value, dict)
        }
        return cls(entries=entries)


# ---------------------------------------------------------------------------
# API usage
# ---------------------------------------------------------------------------


@dataclass
class APIUsageStatsEntry:
    """One bucket of the usage report.

    The range is half-open: ``from_`` is inclusive, ``to`` exclusive.
    """

    #: The inclusive start of the bucket.
    from_: Optional[datetime] = None
    #: The exclusive end of the bucket.
    to: Optional[datetime] = None
    #: The number of requests in the bucket.
    total: int = 0

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "APIUsageStatsEntry":
        """Build a bucket from its payload."""
        return cls(
            from_=parse_datetime(data.get("from")),
            to=parse_datetime(data.get("to")),
            total=as_int(data, "total"),
        )


@dataclass
class APIUsageStats:
    """The usage report for the authenticated client.

    Buckets with zero usage are omitted, so the entries need not be contiguous.
    """

    #: The period that was requested, e.g. ``30days``.
    period: str = ""
    #: The bucket size.
    granularity: Any = None
    #: The inclusive start of the whole range.
    from_: Optional[datetime] = None
    #: The exclusive end of the whole range.
    to: Optional[datetime] = None
    #: The number of requests over the whole range.
    total: int = 0
    #: The per-bucket totals.
    usage: List[APIUsageStatsEntry] = field(default_factory=list)
    #: The response metadata.
    meta: ResponseMeta = field(default_factory=ResponseMeta)
    #: The payload these stats were decoded from.
    raw: Dict[str, Any] = field(default_factory=dict)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "APIUsageStats":
        """Build a usage report from its payload."""
        return cls(
            period=as_str(data, "period"),
            granularity=enum_or_raw(Granularity, data.get("granularity")),
            from_=parse_datetime(data.get("from")),
            to=parse_datetime(data.get("to")),
            total=as_int(data, "total"),
            usage=sub_models(APIUsageStatsEntry.from_dict, data.get("usage")),
            raw=data,
        )


# ---------------------------------------------------------------------------
# Batch summaries and per-item results
# ---------------------------------------------------------------------------


@dataclass
class BatchSummary:
    """A summary of a batch operation.

    ``documents_written`` is not a copy of ``succeeded``. On delete a repeated
    ID is idempotent, so ``succeeded`` counts input rows while
    ``documents_written`` counts distinct documents actually removed — a
    retried delete of an already-deleted document succeeds with zero written.
    On create and patch it can exceed ``total``, because an auto-created
    transfer mirror, a rewritten pairing target, or a propagation to a mirror
    is a document without being an item.
    """

    #: The number of items in the request.
    total: int = 0
    #: The number of items processed successfully.
    succeeded: int = 0
    #: The number of items that failed on bad input.
    client_errors: int = 0
    #: The number of items that failed internally.
    server_errors: int = 0
    #: The number of distinct documents written or removed.
    documents_written: int = 0

    @property
    def failed(self) -> int:
        """The number of items that did not succeed."""
        return self.client_errors + self.server_errors

    @property
    def all_succeeded(self) -> bool:
        """Whether every item succeeded."""
        return self.total > 0 and self.failed == 0

    @property
    def all_failed(self) -> bool:
        """Whether no item succeeded."""
        return self.total > 0 and self.succeeded == 0

    @property
    def partial(self) -> bool:
        """Whether the batch mixed successes and failures."""
        return self.succeeded > 0 and self.failed > 0

    @classmethod
    def from_dict(cls, data: Optional[Dict[str, Any]]) -> "BatchSummary":
        """Build a summary from its payload, tolerating an absent one."""
        if not isinstance(data, dict):
            return cls()
        return cls(
            total=as_int(data, "total"),
            succeeded=as_int(data, "succeeded"),
            client_errors=as_int(data, "clientErrors"),
            server_errors=as_int(data, "serverErrors"),
            documents_written=as_int(data, "documentsWritten"),
        )


#: ``PATCH`` responses use the same shape as ``POST`` and ``DELETE`` ones.
PatchSummary = BatchSummary


@dataclass
class ItemResult:
    """The part every per-item batch result shares."""

    #: The 0-based position of the input item this result answers. It is the
    #: uniform correlation key across every write endpoint.
    input_index: int = 0
    #: The entity identifier the result concerns.
    id: str = ""
    #: Whether the item was processed.
    success: bool = False
    #: The failure message. Empty on success.
    error: str = ""
    #: How the failure is classified.
    error_type: Any = None
    #: The input fields the error is attributable to. Absent when the error
    #: belongs to the item as a whole.
    fields: List[str] = field(default_factory=list)
    #: The payload this result was decoded from.
    raw: Dict[str, Any] = field(default_factory=dict)

    @property
    def is_client_error(self) -> bool:
        """Whether the item failed on bad input."""
        return self.error_type == ErrorType.CLIENT

    @property
    def is_server_error(self) -> bool:
        """Whether the item failed internally."""
        return self.error_type == ErrorType.SERVER

    @classmethod
    def _base_kwargs(cls, data: Dict[str, Any]) -> Dict[str, Any]:
        return dict(
            input_index=as_int(data, "inputIndex"),
            id=as_str(data, "id"),
            success=as_bool(data, "success"),
            error=as_str(data, "error"),
            error_type=enum_or_raw(ErrorType, data.get("errorType")),
            fields=str_list(data, "fields"),
            raw=data,
        )

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "ItemResult":
        """Build a bare result from its payload."""
        return cls(**cls._base_kwargs(data))


@dataclass
class CreateRecordResult(ItemResult):
    """One entry of a ``POST /records`` response."""

    #: The created record. Absent when ``return_data=False``.
    record: Optional[Record] = None
    #: The auto-created B-side of a paired transfer, present only when this
    #: call wrote it. It survives ``return_data=False``.
    created_mirror_record_id: str = ""

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "CreateRecordResult":
        """Build a create result from its payload."""
        return cls(
            **cls._base_kwargs(data),
            record=sub_model(Record.from_dict, data.get("record")),
            created_mirror_record_id=as_str(data, "createdMirrorRecordId"),
        )


@dataclass
class PatchRecordResult(ItemResult):
    """One entry of a ``PATCH /records`` response."""

    #: The updated record. Absent when ``return_data=False``.
    record: Optional[Record] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "PatchRecordResult":
        """Build a patch result from its payload."""
        return cls(**cls._base_kwargs(data), record=sub_model(Record.from_dict, data.get("record")))


@dataclass
class PatchAccountResult(ItemResult):
    """One entry of a ``PATCH /accounts`` response."""

    #: The updated account. Absent when ``return_data=False``.
    account: Optional[Account] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "PatchAccountResult":
        """Build a patch result from its payload."""
        return cls(**cls._base_kwargs(data), account=sub_model(Account.from_dict, data.get("account")))


@dataclass
class PatchBudgetResult(ItemResult):
    """One entry of a ``PATCH /budgets`` response."""

    #: The updated budget. Absent when ``return_data=False``.
    budget: Optional[Budget] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "PatchBudgetResult":
        """Build a patch result from its payload."""
        return cls(**cls._base_kwargs(data), budget=sub_model(Budget.from_dict, data.get("budget")))


@dataclass
class PatchCategoryResult(ItemResult):
    """One entry of a ``PATCH /categories`` response."""

    #: The updated category. Absent when ``return_data=False``.
    category: Optional[Category] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "PatchCategoryResult":
        """Build a patch result from its payload."""
        return cls(**cls._base_kwargs(data), category=sub_model(Category.from_dict, data.get("category")))


@dataclass
class PatchLabelResult(ItemResult):
    """One entry of a ``PATCH /labels`` response."""

    #: The updated label. Absent when ``return_data=False``.
    label: Optional[Label] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "PatchLabelResult":
        """Build a patch result from its payload."""
        return cls(**cls._base_kwargs(data), label=sub_model(Label.from_dict, data.get("label")))


@dataclass
class DeleteResult(ItemResult):
    """One entry of a ``DELETE /{type}`` response."""


# ---------------------------------------------------------------------------
# Response envelopes
# ---------------------------------------------------------------------------


@dataclass
class _Envelope:
    """The metadata and hints every envelope carries."""

    #: The response metadata.
    meta: ResponseMeta = field(default_factory=ResponseMeta)
    #: The hints, when ``agent_hints=True`` was requested.
    agent_hints: AgentHints = field(default_factory=AgentHints)
    #: The payload this envelope was decoded from.
    raw: Dict[str, Any] = field(default_factory=dict)


@dataclass
class _Page(_Envelope):
    """The pagination every list envelope carries."""

    #: The pagination block.
    pagination: Pagination = field(default_factory=Pagination)

    @property
    def has_more(self) -> bool:
        """Whether another page exists."""
        return self.pagination.has_more

    @property
    def next_offset(self) -> Optional[int]:
        """The offset of the next page, or ``None`` on the last one."""
        return self.pagination.next_offset

    @property
    def total(self) -> Optional[int]:
        """The count of all matching items, when ``with_total=True`` was set."""
        return self.pagination.total


def _page_kwargs(data: Dict[str, Any]) -> Dict[str, Any]:
    return dict(
        pagination=Pagination.from_dict(data),
        agent_hints=AgentHints.from_list(data.get("agentHints")),
        raw=data,
    )


def _envelope_kwargs(data: Dict[str, Any]) -> Dict[str, Any]:
    return dict(agent_hints=AgentHints.from_list(data.get("agentHints")), raw=data)


@dataclass
class AccountsPage(_Page):
    """The ``GET /accounts`` response."""

    #: The accounts on this page.
    accounts: List[Account] = field(default_factory=list)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "AccountsPage":
        """Build a page from its payload."""
        return cls(accounts=sub_models(Account.from_dict, data.get("accounts")), **_page_kwargs(data))


@dataclass
class BudgetsPage(_Page):
    """The ``GET /budgets`` response."""

    #: The budgets on this page.
    budgets: List[Budget] = field(default_factory=list)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "BudgetsPage":
        """Build a page from its payload."""
        return cls(budgets=sub_models(Budget.from_dict, data.get("budgets")), **_page_kwargs(data))


@dataclass
class CategoriesPage(_Page):
    """The ``GET /categories`` response."""

    #: The categories on this page.
    categories: List[Category] = field(default_factory=list)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "CategoriesPage":
        """Build a page from its payload."""
        return cls(categories=sub_models(Category.from_dict, data.get("categories")), **_page_kwargs(data))


@dataclass
class GoalsPage(_Page):
    """The ``GET /goals`` response."""

    #: The goals on this page.
    goals: List[Goal] = field(default_factory=list)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "GoalsPage":
        """Build a page from its payload."""
        return cls(goals=sub_models(Goal.from_dict, data.get("goals")), **_page_kwargs(data))


@dataclass
class LabelsPage(_Page):
    """The ``GET /labels`` response."""

    #: The labels on this page.
    labels: List[Label] = field(default_factory=list)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "LabelsPage":
        """Build a page from its payload."""
        return cls(labels=sub_models(Label.from_dict, data.get("labels")), **_page_kwargs(data))


@dataclass
class RecordRulesPage(_Page):
    """The ``GET /record-rules`` response."""

    #: The rules on this page.
    record_rules: List[RecordRule] = field(default_factory=list)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "RecordRulesPage":
        """Build a page from its payload."""
        return cls(record_rules=sub_models(RecordRule.from_dict, data.get("recordRules")), **_page_kwargs(data))


@dataclass
class RecordsPage(_Page):
    """The ``GET /records`` response."""

    #: The records on this page.
    records: List[Record] = field(default_factory=list)
    #: The ``recordDate`` filter the server actually applied, including the
    #: implicit three-month default. ``None`` when no date filter applied at
    #: all, which happens on an ID-based lookup.
    applied_record_date_filters: Optional[List[str]] = None

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "RecordsPage":
        """Build a page from its payload."""
        applied = data.get("appliedRecordDateFilters")
        return cls(
            records=sub_models(Record.from_dict, data.get("records")),
            applied_record_date_filters=(
                [v for v in applied if isinstance(v, str)] if isinstance(applied, list) else None
            ),
            **_page_kwargs(data),
        )

    def default_window_applied(self, requested: bool) -> bool:
        """Whether the server fell back to its implicit three-month window."""
        return not requested and bool(self.applied_record_date_filters)


@dataclass
class StandingOrdersPage(_Page):
    """The ``GET /standing-orders`` response."""

    #: The standing orders on this page.
    standing_orders: List[StandingOrder] = field(default_factory=list)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "StandingOrdersPage":
        """Build a page from its payload."""
        return cls(
            standing_orders=sub_models(StandingOrder.from_dict, data.get("standingOrders")),
            **_page_kwargs(data),
        )


@dataclass
class StandingOrderItemsPage(_Page):
    """The ``GET /standing-orders/items`` response."""

    #: The occurrences on this page.
    standing_order_items: List[StandingOrderItem] = field(default_factory=list)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "StandingOrderItemsPage":
        """Build a page from its payload."""
        return cls(
            standing_order_items=sub_models(StandingOrderItem.from_dict, data.get("standingOrderItems")),
            **_page_kwargs(data),
        )


@dataclass
class CreateAccountResponse(_Envelope):
    """The ``POST /accounts`` response."""

    #: The created account.
    account: Optional[Account] = None
    #: The standard single-write summary.
    summary: BatchSummary = field(default_factory=BatchSummary)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "CreateAccountResponse":
        """Build the response from its payload."""
        return cls(
            account=sub_model(Account.from_dict, data.get("account")),
            summary=BatchSummary.from_dict(data.get("summary")),
            **_envelope_kwargs(data),
        )


@dataclass
class CreateBudgetResponse(_Envelope):
    """The ``POST /budgets`` response."""

    #: The created budget.
    budget: Optional[Budget] = None
    #: The standard single-write summary.
    summary: BatchSummary = field(default_factory=BatchSummary)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "CreateBudgetResponse":
        """Build the response from its payload."""
        return cls(
            budget=sub_model(Budget.from_dict, data.get("budget")),
            summary=BatchSummary.from_dict(data.get("summary")),
            **_envelope_kwargs(data),
        )


@dataclass
class CreateCategoryResponse(_Envelope):
    """The ``POST /categories/custom`` response."""

    #: The created subcategory.
    category: Optional[Category] = None
    #: The standard single-write summary.
    summary: BatchSummary = field(default_factory=BatchSummary)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "CreateCategoryResponse":
        """Build the response from its payload."""
        return cls(
            category=sub_model(Category.from_dict, data.get("category")),
            summary=BatchSummary.from_dict(data.get("summary")),
            **_envelope_kwargs(data),
        )


@dataclass
class CreateLabelResponse(_Envelope):
    """The ``POST /labels`` response."""

    #: The created label.
    label: Optional[Label] = None
    #: The standard single-write summary.
    summary: BatchSummary = field(default_factory=BatchSummary)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "CreateLabelResponse":
        """Build the response from its payload."""
        return cls(
            label=sub_model(Label.from_dict, data.get("label")),
            summary=BatchSummary.from_dict(data.get("summary")),
            **_envelope_kwargs(data),
        )


@dataclass
class _BatchResponse(_Envelope):
    """The summary every batch envelope carries."""

    #: The outcome counts.
    summary: BatchSummary = field(default_factory=BatchSummary)

    def _results(self) -> Sequence[ItemResult]:  # pragma: no cover - overridden
        return ()

    @property
    def succeeded(self) -> List[ItemResult]:
        """The per-item results that succeeded."""
        return [r for r in self._results() if r.success]

    @property
    def failed(self) -> List[ItemResult]:
        """The per-item results that failed."""
        return [r for r in self._results() if not r.success]

    @property
    def client_errors(self) -> List[ItemResult]:
        """The per-item results that failed on bad input."""
        return [r for r in self._results() if r.is_client_error]

    @property
    def server_errors(self) -> List[ItemResult]:
        """The per-item results that failed internally."""
        return [r for r in self._results() if r.is_server_error]

    def result_for(self, input_index: int) -> Optional[ItemResult]:
        """The result answering a given input position, or ``None``."""
        for item in self._results():
            if item.input_index == input_index:
                return item
        return None


@dataclass
class CreateRecordsResponse(_BatchResponse):
    """The ``POST /records`` response.

    It arrives on 200, 207, 400 and 500 alike — read ``meta.status_code`` and
    ``summary`` to tell them apart.
    """

    #: One entry per input record, correlated by ``input_index``.
    results: List[CreateRecordResult] = field(default_factory=list)

    def _results(self) -> Sequence[ItemResult]:
        return self.results

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "CreateRecordsResponse":
        """Build the response from its payload."""
        return cls(
            results=sub_models(CreateRecordResult.from_dict, data.get("results")),
            summary=BatchSummary.from_dict(data.get("summary")),
            **_envelope_kwargs(data),
        )


@dataclass
class PatchRecordsResponse(_BatchResponse):
    """The ``PATCH /records`` response."""

    #: One entry per input item, correlated by ``input_index``.
    results: List[PatchRecordResult] = field(default_factory=list)

    def _results(self) -> Sequence[ItemResult]:
        return self.results

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "PatchRecordsResponse":
        """Build the response from its payload."""
        return cls(
            results=sub_models(PatchRecordResult.from_dict, data.get("results")),
            summary=BatchSummary.from_dict(data.get("summary")),
            **_envelope_kwargs(data),
        )


@dataclass
class PatchAccountsResponse(_BatchResponse):
    """The ``PATCH /accounts`` response."""

    #: One entry per input item, correlated by ``input_index``.
    results: List[PatchAccountResult] = field(default_factory=list)

    def _results(self) -> Sequence[ItemResult]:
        return self.results

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "PatchAccountsResponse":
        """Build the response from its payload."""
        return cls(
            results=sub_models(PatchAccountResult.from_dict, data.get("results")),
            summary=BatchSummary.from_dict(data.get("summary")),
            **_envelope_kwargs(data),
        )


@dataclass
class PatchBudgetsResponse(_BatchResponse):
    """The ``PATCH /budgets`` response."""

    #: One entry per input item, correlated by ``input_index``.
    results: List[PatchBudgetResult] = field(default_factory=list)

    def _results(self) -> Sequence[ItemResult]:
        return self.results

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "PatchBudgetsResponse":
        """Build the response from its payload."""
        return cls(
            results=sub_models(PatchBudgetResult.from_dict, data.get("results")),
            summary=BatchSummary.from_dict(data.get("summary")),
            **_envelope_kwargs(data),
        )


@dataclass
class PatchCategoriesResponse(_BatchResponse):
    """The ``PATCH /categories`` response."""

    #: One entry per input item, correlated by ``input_index``.
    results: List[PatchCategoryResult] = field(default_factory=list)

    def _results(self) -> Sequence[ItemResult]:
        return self.results

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "PatchCategoriesResponse":
        """Build the response from its payload."""
        return cls(
            results=sub_models(PatchCategoryResult.from_dict, data.get("results")),
            summary=BatchSummary.from_dict(data.get("summary")),
            **_envelope_kwargs(data),
        )


@dataclass
class PatchLabelsResponse(_BatchResponse):
    """The ``PATCH /labels`` response."""

    #: One entry per input item, correlated by ``input_index``.
    results: List[PatchLabelResult] = field(default_factory=list)

    def _results(self) -> Sequence[ItemResult]:
        return self.results

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "PatchLabelsResponse":
        """Build the response from its payload."""
        return cls(
            results=sub_models(PatchLabelResult.from_dict, data.get("results")),
            summary=BatchSummary.from_dict(data.get("summary")),
            **_envelope_kwargs(data),
        )


@dataclass
class DeleteResponse(_BatchResponse):
    """The ``DELETE /{type}`` response."""

    #: One entry per input ID, correlated by ``input_index``.
    results: List[DeleteResult] = field(default_factory=list)

    def _results(self) -> Sequence[ItemResult]:
        return self.results

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "DeleteResponse":
        """Build the response from its payload."""
        return cls(
            results=sub_models(DeleteResult.from_dict, data.get("results")),
            summary=BatchSummary.from_dict(data.get("summary")),
            **_envelope_kwargs(data),
        )
