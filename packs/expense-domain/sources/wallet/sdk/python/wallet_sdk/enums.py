"""Enumerations of every documented Wallet API value set.

Each enum subclasses :class:`str`, so a member compares equal to its wire form
and serialises without conversion. Decoding never rejects an unknown value —
the API can add members, and a client that raises on the next release is a
client that breaks. Use :func:`is_valid` (or the ``ALL_*`` tuples) to check a
value against what this SDK knows about; the request validators do exactly
that before sending.
"""

from __future__ import annotations

from enum import Enum
from typing import Iterable, Tuple

__all__ = [
    "StrEnum",
    "AccountType",
    "UniversalColor",
    "Cardinality",
    "BudgetType",
    "RecordState",
    "RecordType",
    "RecordSource",
    "PairingMode",
    "GoalState",
    "Reminder",
    "ErrorType",
    "BalanceMode",
    "HintSeverity",
    "HintType",
    "Granularity",
    "EntityType",
    "ValidationMode",
    "ClearableRecordField",
    "ResettableCategoryField",
    "FilterOp",
    "CREATABLE_ACCOUNT_TYPES",
    "CREATABLE_BUDGET_TYPES",
    "WRITABLE_RECORD_STATES",
    "DELETABLE_ENTITY_TYPES",
    "REFERENCEABLE_ENTITY_TYPES",
    "CATEGORY_UNKNOWN_INCOME",
    "CATEGORY_UNKNOWN_EXPENSE",
    "CATEGORY_UNCATEGORIZED",
    "CATEGORY_TRANSFER",
    "CATEGORY_UNKNOWN",
    "normalize_entity_type",
    "values_of",
    "names_of",
]


class StrEnum(str, Enum):
    """A string enum whose members render as their bare wire value."""

    def __str__(self) -> str:  # pragma: no cover - trivial
        return str(self.value)

    @classmethod
    def is_valid(cls, value: object) -> bool:
        """Report whether ``value`` is a member this SDK knows about."""
        try:
            cls(value)  # type: ignore[call-overload]
        except ValueError:
            return False
        return True


class AccountType(StrEnum):
    """The kind of a financial account.

    The first five are creatable over REST; the rest arrive only through bank
    sync or the mobile apps and are read-only here.
    """

    GENERAL = "General"
    CASH = "Cash"
    CURRENT_ACCOUNT = "CurrentAccount"
    SAVING_ACCOUNT = "SavingAccount"
    INSURANCE = "Insurance"
    CREDIT_CARD = "CreditCard"
    BONUS = "Bonus"
    INVESTMENT = "Investment"
    LOAN = "Loan"
    MORTGAGE = "Mortgage"
    OVERDRAFT = "Overdraft"


#: The account types ``POST /accounts`` accepts.
CREATABLE_ACCOUNT_TYPES: Tuple[AccountType, ...] = (
    AccountType.GENERAL,
    AccountType.CASH,
    AccountType.CURRENT_ACCOUNT,
    AccountType.SAVING_ACCOUNT,
    AccountType.INSURANCE,
)


class UniversalColor(StrEnum):
    """One of the 16 colour names the write endpoints accept.

    Read endpoints answer with a hex string instead, which is why the model
    fields are plain ``str``.
    """

    BLUE = "Blue"
    CHARCOAL = "Charcoal"
    COFFEE_BROWN = "CoffeeBrown"
    DARK_PURPLE = "DarkPurple"
    GREEN = "Green"
    LIGHT_BLUE = "LightBlue"
    LIME = "Lime"
    ORANGE = "Orange"
    PINE_GREEN = "PineGreen"
    PINK = "Pink"
    PURPLE = "Purple"
    RED = "Red"
    STONE = "Stone"
    TURQUOISE = "Turquoise"
    WINE = "Wine"
    YELLOW = "Yellow"


class Cardinality(StrEnum):
    """A category's spending nature."""

    NONE = "none"
    MUST = "must"
    NEED = "need"
    WANT = "want"


class BudgetType(StrEnum):
    """A budget's interval. ``BUDGET_ALL`` is read-only legacy."""

    WEEK = "BUDGET_INTERVAL_WEEK"
    MONTH = "BUDGET_INTERVAL_MONTH"
    YEAR = "BUDGET_INTERVAL_YEAR"
    CUSTOM = "BUDGET_CUSTOM"
    ALL = "BUDGET_ALL"


#: The budget types ``POST /budgets`` accepts.
CREATABLE_BUDGET_TYPES: Tuple[BudgetType, ...] = (
    BudgetType.WEEK,
    BudgetType.MONTH,
    BudgetType.YEAR,
    BudgetType.CUSTOM,
)


class RecordState(StrEnum):
    """A record's clearing state. ``VOID`` and ``WAIT_FOR_ASSIGN`` are read-only."""

    RECONCILED = "reconciled"
    CLEARED = "cleared"
    UNCLEARED = "uncleared"
    VOID = "void"
    WAIT_FOR_ASSIGN = "waitForAssign"


#: The states the write endpoints and the ``recordState`` filter accept.
WRITABLE_RECORD_STATES: Tuple[RecordState, ...] = (
    RecordState.RECONCILED,
    RecordState.CLEARED,
    RecordState.UNCLEARED,
)


class RecordType(StrEnum):
    """The direction of a record, derived from the amount sign."""

    INCOME = "income"
    EXPENSE = "expense"


class RecordSource(StrEnum):
    """The client that created a record."""

    ANDROID = "android"
    IOS = "ios"
    WEB = "web"
    REST = "rest"
    MCP = "mcp"
    BACKEND = "backend"
    MISSING = "missing"


class PairingMode(StrEnum):
    """How a created record binds to its transfer counterpart."""

    #: Create a mirror record on the target account.
    NEW = "new"
    #: Bind to an existing record on another account.
    EXISTING = "existing"
    #: Record a transfer with no mirror.
    UNPAIRED = "unpaired"


class GoalState(StrEnum):
    """The lifecycle state of a savings goal."""

    ACTIVE = "active"
    PAUSED = "paused"
    REACHED = "reached"


class Reminder(StrEnum):
    """A standing order's notification lead time."""

    DUE_TODAY = "due_today"
    NONE = "none"
    ONE_DAY = "1_day"
    THREE_DAYS = "3_days"
    SEVEN_DAYS = "7_days"


class ErrorType(StrEnum):
    """How a per-item batch failure is classified."""

    #: Bad input: fixing the item and resending can work.
    CLIENT = "client_error"
    #: An internal failure: resending the same item can work.
    SERVER = "server_error"


class BalanceMode(StrEnum):
    """The formula variant used to compute an account balance."""

    STANDARD = "standard"
    CREDIT_CARD_MANUAL = "creditCardManual"
    CREDIT_CARD_MANUAL_INVERSE = "creditCardManualInverse"
    CREDIT_CARD_BANK = "creditCardBank"
    CREDIT_CARD_BANK_REVERSED = "creditCardBankReversed"
    OVERDRAFT_MANUAL = "overdraftManual"
    OVERDRAFT_BANK = "overdraftBank"


class HintSeverity(StrEnum):
    """How much an agent hint matters."""

    INFO = "info"
    WARNING = "warning"
    INSTRUCTION = "instruction"


class HintType(StrEnum):
    """The dot-notation category of an agent hint."""

    PAGINATION_HAS_MORE = "pagination.has_more"
    RESULT_PARTIAL_MATCH = "result.partial_match"
    RESULT_EMPTY = "result.empty"
    RATE_LIMIT_WARNING = "rate_limit.warning"


class Granularity(StrEnum):
    """The bucket size of an API usage report."""

    DAILY = "daily"
    WEEKLY = "weekly"
    MONTHLY = "monthly"


class EntityType(StrEnum):
    """A collection in the ``/{type}`` path of the delete and references endpoints.

    ``ORDERS`` is Board-only; the Wallet API answers 404 for it.
    """

    ACCOUNTS = "accounts"
    BUDGETS = "budgets"
    CATEGORIES = "categories"
    LABELS = "labels"
    RECORDS = "records"
    RECORD_RULES = "record-rules"
    STANDING_ORDERS = "standing-orders"
    ORDERS = "orders"


#: Every type ``DELETE /v1/api/{type}`` accepts.
DELETABLE_ENTITY_TYPES: Tuple[EntityType, ...] = tuple(EntityType)

#: Every type ``GET /v1/api/{type}/references`` supports. Budgets, standing
#: orders and record rules are absent by design: nothing references them.
REFERENCEABLE_ENTITY_TYPES: Tuple[EntityType, ...] = (
    EntityType.CATEGORIES,
    EntityType.RECORDS,
    EntityType.ACCOUNTS,
    EntityType.LABELS,
    EntityType.ORDERS,
)

#: The singular forms the references endpoint normalises.
_SINGULAR_ENTITY_TYPES = {
    "category": EntityType.CATEGORIES,
    "record": EntityType.RECORDS,
    "account": EntityType.ACCOUNTS,
    "label": EntityType.LABELS,
    "order": EntityType.ORDERS,
}


def normalize_entity_type(value: object) -> object:
    """Fold an accepted singular form onto its canonical plural.

    Anything else is returned untouched, so an unknown value still reaches the
    validator that will report it.
    """
    if isinstance(value, EntityType):
        return value
    if isinstance(value, str):
        canonical = _SINGULAR_ENTITY_TYPES.get(value.lower())
        if canonical is not None:
            return canonical
        try:
            return EntityType(value)
        except ValueError:
            return value
    return value


class ValidationMode(StrEnum):
    """How ``PATCH /records`` treats invalid items."""

    #: Process the valid items and skip the invalid ones. The server default.
    LENIENT = "lenient"
    #: Validate everything first: one client error and the whole call returns
    #: 400 with no writes.
    STRICT = "strict"


class ClearableRecordField(StrEnum):
    """A field ``PATCH /records`` can reset through ``$clear``."""

    CATEGORY_ID = "categoryId"
    LABEL_IDS = "labelIds"
    NOTE = "note"
    COUNTER_PARTY = "counterParty"
    PLACE = "place"
    #: Cancels the transfer state and sets the category to Unknown.
    TRANSFER = "transfer"


class ResettableCategoryField(StrEnum):
    """A field ``PATCH /categories`` can restore to the base category's default.

    ``COLOR`` restores the colour of the category's ENVELOPE GROUP, shared by
    every category in that group — not the colour this category had before. A
    colour set from a mobile or web app outside the 16 universal names is
    therefore unrecoverable: read the current colour first if you may want it
    back.
    """

    NAME = "name"
    CARDINALITY = "cardinality"
    COLOR = "color"


class FilterOp(StrEnum):
    """A query filter prefix.

    The API rejects an unprefixed value on any parameter that documents a
    prefix, so every filter goes through one of these.
    """

    EQ = "eq"
    GT = "gt"
    GTE = "gte"
    LT = "lt"
    LTE = "lte"
    CONTAINS = "contains"
    CONTAINS_I = "contains-i"

    @property
    def is_range(self) -> bool:
        """Report whether the operator is valid on a range parameter."""
        return self in (FilterOp.EQ, FilterOp.GT, FilterOp.GTE, FilterOp.LT, FilterOp.LTE)

    @property
    def is_text(self) -> bool:
        """Report whether the operator is valid on a text parameter."""
        return self in (FilterOp.EQ, FilterOp.CONTAINS, FilterOp.CONTAINS_I)


# Category IDs the API guarantees are stable across every account.
CATEGORY_UNKNOWN_INCOME = "5c5c32c8-0082-8000-8000-000000000000"
CATEGORY_UNKNOWN_EXPENSE = "5c5c32c9-0082-8000-8000-000000000000"
CATEGORY_UNCATEGORIZED = "5c5c4e23-00c8-8000-8000-000000000000"
CATEGORY_TRANSFER = "5c5c4e21-00c8-8000-8000-000000000000"

#: The magic ``categoryId`` filter value that expands to Unknown income,
#: Unknown expense and Uncategorized.
CATEGORY_UNKNOWN = "unknown"


def values_of(members: Iterable[StrEnum]) -> Tuple[str, ...]:
    """Return the wire values of an enum collection."""
    return tuple(str(m.value) for m in members)


def names_of(members: Iterable[StrEnum]) -> str:
    """Render an enum collection for an error message."""
    return ", ".join(values_of(members))
