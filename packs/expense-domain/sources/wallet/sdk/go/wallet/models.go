package wallet

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Response metadata
// ---------------------------------------------------------------------------

// ResponseMeta carries the documented response headers plus the status code.
// Every response envelope embeds it, so a caller can watch the rate-limit
// budget or the data-change revision without dropping to a custom transport.
type ResponseMeta struct {
	// StatusCode is the HTTP status that produced this envelope. It matters
	// on batch endpoints, where 200, 207, 400 and 500 all carry a body.
	StatusCode int `json:"-"`
	// RateLimitLimit is X-RateLimit-Limit — the hourly request capacity.
	RateLimitLimit *int `json:"-"`
	// RateLimitRemaining is X-RateLimit-Remaining, the capacity left in the
	// current window.
	RateLimitRemaining *int `json:"-"`
	// RetryAfter is the parsed Retry-After header, if present.
	RetryAfter time.Duration `json:"-"`
	// LastDataChangeAt is X-Last-Data-Change-At.
	LastDataChangeAt *time.Time `json:"-"`
	// LastDataChangeRev is X-Last-Data-Change-Rev, a revision counter that can
	// be compared between requests to detect changes.
	LastDataChangeRev string `json:"-"`
	// SyncInProgress reflects X-Sync-In-Progress: when true the data is valid
	// but a background sync is running and more changes may follow.
	SyncInProgress *bool `json:"-"`
	// Header is the full response header set.
	Header http.Header `json:"-"`
	// Attempts is how many HTTP round trips the client made, including
	// retries, to produce this response.
	Attempts int `json:"-"`
}

// RateLimitExhausted reports whether the remaining budget has reached zero.
func (m ResponseMeta) RateLimitExhausted() bool {
	return m.RateLimitRemaining != nil && *m.RateLimitRemaining <= 0
}

// responseMeta is embedded by every envelope to satisfy metaSetter.
type responseMeta struct {
	// Meta carries the response headers and status code.
	Meta ResponseMeta `json:"-"`
}

func (r *responseMeta) setMeta(m ResponseMeta) { r.Meta = m }

// metaSetter is implemented by every top-level response type.
type metaSetter interface {
	setMeta(ResponseMeta)
}

// parseMeta reads the documented headers off a response.
func parseMeta(resp *http.Response, attempts int) ResponseMeta {
	m := ResponseMeta{StatusCode: resp.StatusCode, Header: resp.Header, Attempts: attempts}
	if v := resp.Header.Get("X-RateLimit-Limit"); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			m.RateLimitLimit = &n
		}
	}
	if v := resp.Header.Get("X-RateLimit-Remaining"); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			m.RateLimitRemaining = &n
		}
	}
	if v := resp.Header.Get("X-Last-Data-Change-At"); v != "" {
		var t Time
		if err := t.UnmarshalJSON([]byte(strconv.Quote(v))); err == nil && t.Valid() {
			parsed := t.Time
			m.LastDataChangeAt = &parsed
		}
	}
	m.LastDataChangeRev = resp.Header.Get("X-Last-Data-Change-Rev")
	if v := resp.Header.Get("X-Sync-In-Progress"); v != "" {
		b := strings.EqualFold(strings.TrimSpace(v), "true")
		m.SyncInProgress = &b
	}
	m.RetryAfter = parseRetryAfter(resp.Header.Get("Retry-After"))
	return m
}

// parseRetryAfter handles both forms of Retry-After: delta-seconds and an
// HTTP date. A date in the past yields zero.
func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs < 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	if when, err := http.ParseTime(v); err == nil {
		if d := time.Until(when); d > 0 {
			return d
		}
	}
	return 0
}

// ---------------------------------------------------------------------------
// Pagination and hints
// ---------------------------------------------------------------------------

// Pagination is the shared envelope of every list endpoint.
type Pagination struct {
	// Limit is the page size actually applied.
	Limit int `json:"limit"`
	// Offset is the starting position actually applied.
	Offset int `json:"offset"`
	// NextOffset is the offset of the next page. It is absent on the last page.
	NextOffset *int `json:"nextOffset,omitempty"`
	// Total is the count of all matching items. It is present only when
	// withTotal=true was requested.
	Total *int `json:"total,omitempty"`
}

// HasMore reports whether another page exists.
func (p Pagination) HasMore() bool { return p.NextOffset != nil }

// AgentHintAction carries the follow-up an instruction hint asks for.
type AgentHintAction struct {
	// URL fetches the next page, for a pagination.has_more hint.
	URL string `json:"url,omitempty"`
}

// AgentHint is a structured hint for AI agents, returned only when
// agentHints=true was requested.
type AgentHint struct {
	// Type is the dot-notation hint category.
	Type HintType `json:"type"`
	// Severity indicates the nature of the hint.
	Severity HintSeverity `json:"severity"`
	// Text is the human-readable description.
	Text string `json:"text"`
	// Data holds hint-specific context; its shape varies with Type.
	Data map[string]any `json:"data,omitempty"`
	// Action is present only when Severity is "instruction".
	Action *AgentHintAction `json:"action,omitempty"`
}

// AgentHints is a slice of hints with convenience lookups.
type AgentHints []AgentHint

// ByType returns the hints of the given type, in the order the API sent them.
func (h AgentHints) ByType(t HintType) []AgentHint {
	var out []AgentHint
	for _, hint := range h {
		if hint.Type == t {
			out = append(out, hint)
		}
	}
	return out
}

// First returns the first hint of the given type, and whether one was found.
func (h AgentHints) First(t HintType) (AgentHint, bool) {
	for _, hint := range h {
		if hint.Type == t {
			return hint, true
		}
	}
	return AgentHint{}, false
}

// ---------------------------------------------------------------------------
// Shared value objects
// ---------------------------------------------------------------------------

// AmountWithCurrency is a decimal amount plus its ISO 4217 currency. Values
// are decimals, not minor units: 1234.56 means one thousand two hundred
// thirty-four and fifty-six hundredths.
type AmountWithCurrency struct {
	// CurrencyCode is the ISO 4217 code.
	CurrencyCode string `json:"currencyCode,omitempty"`
	// Value is the decimal amount; negative is an expense.
	Value float64 `json:"value"`
}

// ConvertedAmount is a record amount rendered in the currency asked for via
// the convertTo query parameter.
type ConvertedAmount struct {
	// ConversionPair is "recordCurrency:targetCurrency".
	ConversionPair string `json:"conversionPair,omitempty"`
	// CurrencyCode is the target currency.
	CurrencyCode string `json:"currencyCode,omitempty"`
	// Error is set when no rate was available; the other fields are then absent.
	Error string `json:"error,omitempty"`
	// Ratio is the exchange rate applied.
	Ratio float64 `json:"ratio,omitempty"`
	// Value is the converted amount.
	Value float64 `json:"value,omitempty"`
}

// OK reports whether a rate was available.
func (c ConvertedAmount) OK() bool { return c.Error == "" }

// StatDateRange is the min/max pair inside AccountStats.
type StatDateRange struct {
	// Max is the latest value in the range.
	Max string `json:"max,omitempty"`
	// Min is the earliest value in the range.
	Min string `json:"min,omitempty"`
}

// ---------------------------------------------------------------------------
// Accounts
// ---------------------------------------------------------------------------

// AccountBalance is the computed balance of an account, with credit-card and
// overdraft corrections applied. Read-only.
type AccountBalance struct {
	// AvailableCredit is the credit still available on a credit account.
	AvailableCredit *float64 `json:"availableCredit,omitempty"`
	// BalanceDisplayOption is the app's display preference for this account.
	BalanceDisplayOption string `json:"balanceDisplayOption,omitempty"`
	// BalanceMode identifies which formula variant produced the numbers.
	BalanceMode BalanceMode `json:"balanceMode,omitempty"`
	// BalanceModeFormula shows both variants of the formula for this mode.
	BalanceModeFormula string `json:"balanceModeFormula,omitempty"`
	// CreditBalance is the balance owed on a credit account.
	CreditBalance *float64 `json:"creditBalance,omitempty"`
	// CreditLimit is the account's credit ceiling.
	CreditLimit *float64 `json:"creditLimit,omitempty"`
	// CurrencyCode repeats the account currency so amounts are self-describing.
	CurrencyCode string `json:"currencyCode,omitempty"`
	// CurrentBalance is the corrected balance.
	CurrentBalance *float64 `json:"currentBalance,omitempty"`
	// Error explains why the balance could not be computed.
	Error string `json:"error,omitempty"`
	// Formula is the exact computation used.
	Formula string `json:"formula,omitempty"`
	// Initial is the opening balance, i.e. what was sent as initialBalance.
	Initial *float64 `json:"initial,omitempty"`
	// RawCurrentBalance is the balance before credit corrections.
	RawCurrentBalance *float64 `json:"rawCurrentBalance,omitempty"`
}

// AccountStats summarises the records held by an account. Read-only.
type AccountStats struct {
	// CreatedAt is the min/max record creation timestamp.
	CreatedAt *StatDateRange `json:"createdAt,omitempty"`
	// Error explains why the stats could not be computed.
	Error string `json:"error,omitempty"`
	// ErrorAt is when the stats computation failed.
	ErrorAt string `json:"errorAt,omitempty"`
	// LastUpdatedAt is when the stats were last recomputed.
	LastUpdatedAt string `json:"lastUpdatedAt,omitempty"`
	// RecordCount is the number of records on the account.
	RecordCount int `json:"recordCount,omitempty"`
	// RecordDate is the min/max transaction date.
	RecordDate *StatDateRange `json:"recordDate,omitempty"`
	// TotalExpenses is the sum of expenses.
	TotalExpenses *float64 `json:"totalExpenses,omitempty"`
	// TotalIncomes is the sum of incomes.
	TotalIncomes *float64 `json:"totalIncomes,omitempty"`
}

// Account is a financial account.
type Account struct {
	// ID is the unique identifier.
	ID string `json:"id,omitempty"`
	// Name is the display name.
	Name string `json:"name,omitempty"`
	// AccountType is the kind of account.
	AccountType AccountType `json:"accountType,omitempty"`
	// Archived reports whether the account is archived.
	Archived bool `json:"archived,omitempty"`
	// Balance is the computed balance. Nil when unavailable.
	Balance *AccountBalance `json:"balance,omitempty"`
	// BankAccountNumber is the account number or IBAN.
	BankAccountNumber string `json:"bankAccountNumber,omitempty"`
	// Color is a hex colour code. Write endpoints take a UniversalColor name.
	Color string `json:"color,omitempty"`
	// CurrencyCode is the account currency, or "" when unresolvable.
	CurrencyCode string `json:"currencyCode,omitempty"`
	// ExcludeFromStats keeps the account out of aggregate statistics.
	ExcludeFromStats bool `json:"excludeFromStats,omitempty"`
	// IsBankSync reports whether the account is fed by an automatic bank
	// connection. Read-only.
	IsBankSync bool `json:"isBankSync,omitempty"`
	// IsInvestmentAccount reports whether the account holds portfolio data.
	// Read-only.
	IsInvestmentAccount bool `json:"isInvestmentAccount,omitempty"`
	// RecordStats summarises the account's records. Nil when unavailable.
	RecordStats *AccountStats `json:"recordStats,omitempty"`
	// CreatedAt is when the account was created.
	CreatedAt Time `json:"createdAt,omitempty"`
	// UpdatedAt is when the account was last synced into the API database.
	UpdatedAt Time `json:"updatedAt,omitempty"`
}

// ---------------------------------------------------------------------------
// Categories and labels
// ---------------------------------------------------------------------------

// CategoryGroup is the root-level envelope group a category belongs to.
type CategoryGroup struct {
	// ID is the stable group slug, e.g. "food_and_drinks".
	ID string `json:"id,omitempty"`
	// Name is the group display name, e.g. "Food & Drinks".
	Name string `json:"name,omitempty"`
}

// Category is a system category or a custom subcategory.
type Category struct {
	// ID is the category identifier. System categories use deterministic IDs.
	ID string `json:"id,omitempty"`
	// Name is the display name.
	Name string `json:"name,omitempty"`
	// Archived reports whether the category is archived.
	Archived bool `json:"archived,omitempty"`
	// Cardinality is the spending nature, possibly inherited from the base
	// category.
	Cardinality Cardinality `json:"cardinality,omitempty"`
	// Color is a hex colour code.
	Color string `json:"color,omitempty"`
	// CustomCategory reports whether this is a user-created subcategory.
	CustomCategory bool `json:"customCategory,omitempty"`
	// CustomName reports whether the name was overridden by the user.
	CustomName bool `json:"customName,omitempty"`
	// Enabled reports whether the category is offered in the apps.
	Enabled bool `json:"enabled,omitempty"`
	// Group is the envelope group. Nil when the API omits it.
	Group *CategoryGroup `json:"group,omitempty"`
	// ParentID is the base category's ID. Custom categories only.
	ParentID string `json:"parentId,omitempty"`
	// ParentName is the base category's display name. Custom categories only.
	ParentName string `json:"parentName,omitempty"`
	// SystemID is the envelope slug. System categories only.
	SystemID string `json:"systemId,omitempty"`
	// CreatedAt is when the category was created. May be absent.
	CreatedAt Time `json:"createdAt,omitempty"`
	// UpdatedAt is when the category was last synced. May be absent.
	UpdatedAt Time `json:"updatedAt,omitempty"`
}

// CategoryEmbed is the trimmed category attached to a record or record rule.
type CategoryEmbed struct {
	// ID is the category identifier.
	ID string `json:"id,omitempty"`
	// Name is the display name.
	Name string `json:"name,omitempty"`
	// Color is a hex colour code.
	Color string `json:"color,omitempty"`
	// Group is the envelope group.
	Group *CategoryGroup `json:"group,omitempty"`
}

// Label is a hashtag applied to records, budgets, standing orders and rules.
type Label struct {
	// ID is the label identifier.
	ID string `json:"id,omitempty"`
	// Name is the display name.
	Name string `json:"name,omitempty"`
	// Archived reports whether the label is archived.
	Archived bool `json:"archived,omitempty"`
	// Color is a hex colour code.
	Color string `json:"color,omitempty"`
	// CreatedAt is when the label was created.
	CreatedAt Time `json:"createdAt,omitempty"`
	// UpdatedAt is when the label was last synced.
	UpdatedAt Time `json:"updatedAt,omitempty"`
}

// LabelEmbed is the trimmed label attached to a record, rule or order.
type LabelEmbed struct {
	// ID is the label identifier.
	ID string `json:"id,omitempty"`
	// Name is the display name.
	Name string `json:"name,omitempty"`
	// Archived reports whether the label is archived.
	Archived bool `json:"archived,omitempty"`
	// Color is a hex colour code.
	Color string `json:"color,omitempty"`
}

// ---------------------------------------------------------------------------
// Budgets
// ---------------------------------------------------------------------------

// BudgetChangeEntry is one limit override on a budget.
type BudgetChangeEntry struct {
	// CreatedAt is when the override was written.
	CreatedAt string `json:"createdAt,omitempty"`
	// Limit is the spending limit this entry imposes.
	Limit float64 `json:"limit,omitempty"`
	// Period is the period identifier: YYYY-MM, YYYY-Www or YYYY.
	Period string `json:"period,omitempty"`
	// PeriodCount is how many periods the entry governs, counting its own.
	// Zero means open-ended.
	PeriodCount int `json:"periodCount,omitempty"`
	// PeriodStart is the first day the entry applies to (YYYY-MM-DD).
	PeriodStart string `json:"periodStart,omitempty"`
}

// ExcludedBreakdown counts the records a budget period deliberately ignored.
type ExcludedBreakdown struct {
	// Debts is the number of debt records excluded.
	Debts int `json:"debts,omitempty"`
	// IncomeCategories is the number of income-category records excluded.
	IncomeCategories int `json:"incomeCategories,omitempty"`
	// Total is the total number of excluded records.
	Total int `json:"total,omitempty"`
	// TotalAmountSum is the summed amount of the excluded records.
	TotalAmountSum *float64 `json:"totalAmountSum,omitempty"`
	// Transfers is the number of transfer records excluded.
	Transfers int `json:"transfers,omitempty"`
	// UnknownCategories is the number of uncategorised records excluded.
	UnknownCategories int `json:"unknownCategories,omitempty"`
}

// BudgetPeriodSpending is one period's spending against a budget.
type BudgetPeriodSpending struct {
	// ConvertedCurrencies lists the currencies folded into the totals.
	ConvertedCurrencies []string `json:"convertedCurrencies,omitempty"`
	// EffectiveLimit is the limit in force for this period, overrides applied.
	EffectiveLimit float64 `json:"effectiveLimit,omitempty"`
	// Error explains why the period could not be computed.
	Error string `json:"error,omitempty"`
	// Excluded breaks down the records left out of the totals.
	Excluded *ExcludedBreakdown `json:"excluded,omitempty"`
	// Incomplete reports that the period is still in progress.
	Incomplete bool `json:"incomplete,omitempty"`
	// Overspent is the amount spent beyond the limit.
	Overspent *float64 `json:"overspent,omitempty"`
	// Period is the period identifier.
	Period string `json:"period,omitempty"`
	// PeriodEnd is the last instant of the period.
	PeriodEnd *string `json:"periodEnd,omitempty"`
	// PeriodStart is the first instant of the period.
	PeriodStart *string `json:"periodStart,omitempty"`
	// Progress is the fraction of the limit consumed.
	Progress *float64 `json:"progress,omitempty"`
	// RecordCount is the number of records counted.
	RecordCount int `json:"recordCount,omitempty"`
	// Remaining is the amount left before the limit.
	Remaining *float64 `json:"remaining,omitempty"`
	// Spent is the amount counted against the limit.
	Spent *float64 `json:"spent,omitempty"`
	// TotalExpenses is the sum of expenses in the period.
	TotalExpenses *float64 `json:"totalExpenses,omitempty"`
	// TotalIncomes is the sum of incomes in the period.
	TotalIncomes *float64 `json:"totalIncomes,omitempty"`
}

// BudgetSpending is the spending block returned when the spending query
// parameter asks for it.
type BudgetSpending struct {
	// ComputedAt is when the figures were calculated.
	ComputedAt string `json:"computedAt,omitempty"`
	// Current is the period in progress. Nil when not requested or unavailable.
	Current *BudgetPeriodSpending `json:"current,omitempty"`
	// Past holds the preceding periods, most recent first.
	Past []BudgetPeriodSpending `json:"past,omitempty"`
}

// Budget is a spending limit over a repeating or custom interval.
type Budget struct {
	// ID is the budget identifier.
	ID string `json:"id,omitempty"`
	// Name is the display name.
	Name string `json:"name,omitempty"`
	// AccountIDs are the accounts the budget tracks.
	AccountIDs []string `json:"accountIds,omitempty"`
	// CategoryIDs are the categories the budget tracks.
	CategoryIDs []string `json:"categoryIds,omitempty"`
	// LabelIDs are the labels the budget filters on.
	LabelIDs []string `json:"labelIds,omitempty"`
	// Closed reports whether the budget is closed. Nil when unset.
	Closed *bool `json:"closed,omitempty"`
	// ClosedDate is the day the budget was closed. Present only when closed.
	ClosedDate Date `json:"closedDate,omitempty"`
	// CurrencyCode is the budget currency.
	CurrencyCode string `json:"currencyCode,omitempty"`
	// StartDate is the first day of the budget.
	StartDate Date `json:"startDate,omitempty"`
	// EndDate is the last day of the budget.
	EndDate Date `json:"endDate,omitempty"`
	// Limit is the uniform spending limit. Nil when unset.
	Limit *float64 `json:"limit,omitempty"`
	// LimitOverrides are the overrides in force now or later, effective-from.
	LimitOverrides []BudgetChangeEntry `json:"limitOverrides,omitempty"`
	// PastLimitOverrides are overrides a later entry has already superseded.
	PastLimitOverrides []BudgetChangeEntry `json:"pastLimitOverrides,omitempty"`
	// Spending is the spending block, when requested.
	Spending *BudgetSpending `json:"spending,omitempty"`
	// Type is the budget interval.
	Type BudgetType `json:"type,omitempty"`
	// CreatedAt is when the budget was created.
	CreatedAt Time `json:"createdAt,omitempty"`
	// UpdatedAt is when the budget was last synced.
	UpdatedAt Time `json:"updatedAt,omitempty"`
}

// ---------------------------------------------------------------------------
// Records
// ---------------------------------------------------------------------------

// RecordPhoto is an image attached to a record.
type RecordPhoto struct {
	// CreatedAt is when the photo was attached.
	CreatedAt Time `json:"createdAt,omitempty"`
	// TemporaryURL is a signed URL valid for a limited time.
	TemporaryURL string `json:"temporaryUrl,omitempty"`
}

// RecordPlace is the location attached to a record.
type RecordPlace struct {
	// Address is the street or full address.
	Address string `json:"address,omitempty"`
	// Latitude is the GPS latitude.
	Latitude float64 `json:"latitude,omitempty"`
	// Longitude is the GPS longitude.
	Longitude float64 `json:"longitude,omitempty"`
	// Name is the place name.
	Name string `json:"name,omitempty"`
}

// MirrorRecordEmbed is the far side of a paired transfer.
type MirrorRecordEmbed struct {
	// ID is the linked record's identifier.
	ID string `json:"id,omitempty"`
	// AccountID is the linked record's account.
	AccountID string `json:"accountId,omitempty"`
	// Amount is the linked record's amount.
	Amount *AmountWithCurrency `json:"amount,omitempty"`
	// CounterParty is the linked record's counter party.
	CounterParty string `json:"counterParty,omitempty"`
	// Note is the linked record's note.
	Note string `json:"note,omitempty"`
}

// TransferOutput is the read-only transfer state of a record. It is never
// accepted as input.
type TransferOutput struct {
	// MirrorRecord is the other leg, present only when paired.
	MirrorRecord *MirrorRecordEmbed `json:"mirrorRecord,omitempty"`
	// TransferID is the transfer identity: held by one record when unpaired,
	// shared by two when paired.
	TransferID string `json:"transferId,omitempty"`
	// Type is "paired" or "unpaired".
	Type string `json:"type,omitempty"`
}

// Paired reports whether a mirror record exists on the other side.
func (t TransferOutput) Paired() bool { return t.MirrorRecord != nil && t.MirrorRecord.ID != "" }

// Record is a financial transaction.
type Record struct {
	// ID is the record identifier.
	ID string `json:"id,omitempty"`
	// AccountID is the owning account.
	AccountID string `json:"accountId,omitempty"`
	// AccountName is the account's display name. Read-only.
	AccountName string `json:"accountName,omitempty"`
	// AccountIsBankSync reports whether the account is bank-synced.
	AccountIsBankSync *bool `json:"accountIsBankSync,omitempty"`
	// Amount is the transaction amount in its original currency.
	Amount AmountWithCurrency `json:"amount"`
	// ConvertedAmount is present only when convertTo was requested.
	ConvertedAmount *ConvertedAmount `json:"convertedAmount,omitempty"`
	// Category is the embedded category.
	Category *CategoryEmbed `json:"category,omitempty"`
	// CounterParty is the payee for expenses, the payer for income.
	CounterParty string `json:"counterParty,omitempty"`
	// Labels are the labels attached to the record.
	Labels []LabelEmbed `json:"labels,omitempty"`
	// Note is the free-text note.
	Note string `json:"note,omitempty"`
	// Photos are the images attached to the record.
	Photos []RecordPhoto `json:"photos,omitempty"`
	// Place is the location attached to the record.
	Place *RecordPlace `json:"place,omitempty"`
	// RecordDate is the transaction date.
	RecordDate Time `json:"recordDate,omitempty"`
	// RecordState is the clearing state.
	RecordState RecordState `json:"recordState,omitempty"`
	// RecordType is the direction, derived from the amount sign.
	RecordType RecordType `json:"recordType,omitempty"`
	// Source is the client that created the record. Read-only.
	Source RecordSource `json:"source,omitempty"`
	// Transfer is the transfer state. Nil for regular records.
	Transfer *TransferOutput `json:"transfer,omitempty"`
	// CreatedAt is when the record was created.
	CreatedAt Time `json:"createdAt,omitempty"`
	// UpdatedAt is when the record was last synced.
	UpdatedAt Time `json:"updatedAt,omitempty"`
}

// IsTransfer reports whether the record participates in a transfer.
func (r Record) IsTransfer() bool { return r.Transfer != nil && r.Transfer.TransferID != "" }

// RecordRule is an automatic categorisation rule.
type RecordRule struct {
	// ID is the rule identifier.
	ID string `json:"id,omitempty"`
	// Name is the display name.
	Name string `json:"name,omitempty"`
	// Category is the category the rule assigns.
	Category *CategoryEmbed `json:"category,omitempty"`
	// FromAccountID scopes the rule to records leaving this account.
	FromAccountID string `json:"fromAccountId,omitempty"`
	// ToAccountID scopes the rule to records entering this account.
	ToAccountID string `json:"toAccountId,omitempty"`
	// Keywords are the strings the rule matches on.
	Keywords []string `json:"keywords,omitempty"`
	// Labels are the labels the rule applies.
	Labels []LabelEmbed `json:"labels,omitempty"`
	// CreatedAt is when the rule was created.
	CreatedAt Time `json:"createdAt,omitempty"`
	// UpdatedAt is when the rule was last synced.
	UpdatedAt Time `json:"updatedAt,omitempty"`
}

// ---------------------------------------------------------------------------
// Goals and standing orders
// ---------------------------------------------------------------------------

// Goal is a savings target.
type Goal struct {
	// ID is the goal identifier.
	ID string `json:"id,omitempty"`
	// Name is the display name.
	Name string `json:"name,omitempty"`
	// Note is the free-text note.
	Note string `json:"note,omitempty"`
	// Color is a hex colour code.
	Color string `json:"color,omitempty"`
	// DesiredDate is the target completion date.
	DesiredDate Time `json:"desiredDate,omitempty"`
	// InitialAmount is the starting amount, in the profile's base currency.
	InitialAmount *AmountWithCurrency `json:"initialAmount,omitempty"`
	// TargetAmount is the goal amount, in the profile's base currency.
	TargetAmount *AmountWithCurrency `json:"targetAmount,omitempty"`
	// State is the lifecycle state.
	State GoalState `json:"state,omitempty"`
	// StateUpdatedAt is when the state last changed.
	StateUpdatedAt Time `json:"stateUpdatedAt,omitempty"`
	// CreatedAt is when the goal was created.
	CreatedAt Time `json:"createdAt,omitempty"`
	// UpdatedAt is when the goal was last synced.
	UpdatedAt Time `json:"updatedAt,omitempty"`
}

// StandingOrder is a recurring payment.
type StandingOrder struct {
	// ID is the standing order identifier.
	ID string `json:"id,omitempty"`
	// Name is the display name.
	Name string `json:"name,omitempty"`
	// AccountID is the account the payment moves through.
	AccountID string `json:"accountId,omitempty"`
	// Amount is the payment amount.
	Amount float64 `json:"amount,omitempty"`
	// CategoryID is the category assigned to generated records.
	CategoryID string `json:"categoryId,omitempty"`
	// CounterParty is the payee or payer.
	CounterParty string `json:"counterParty,omitempty"`
	// CurrencyCode is the payment currency.
	CurrencyCode string `json:"currencyCode,omitempty"`
	// DueDate is the next due date.
	DueDate Time `json:"dueDate,omitempty"`
	// DueDateNotificationEnabled turns on the due-date notification.
	DueDateNotificationEnabled bool `json:"dueDateNotificationEnabled,omitempty"`
	// GenerateFromDate is the date generation starts from.
	GenerateFromDate string `json:"generateFromDate,omitempty"`
	// Labels are the labels applied to generated records.
	Labels []LabelEmbed `json:"labels,omitempty"`
	// ManualPayment requires the user to confirm each occurrence.
	ManualPayment bool `json:"manualPayment,omitempty"`
	// Note is the free-text note.
	Note string `json:"note,omitempty"`
	// PaymentCount is how many occurrences are tracked.
	PaymentCount int `json:"paymentCount,omitempty"`
	// RecurrenceRule is the schedule in RRULE format.
	RecurrenceRule string `json:"recurrenceRule,omitempty"`
	// Reminder is the notification lead time.
	Reminder Reminder `json:"reminder,omitempty"`
	// ThreeDaysBeforeNotificationEnabled turns on the three-day notification.
	ThreeDaysBeforeNotificationEnabled bool `json:"threeDaysBeforeNotificationEnabled,omitempty"`
	// Type is the direction of the payment.
	Type RecordType `json:"type,omitempty"`
	// CreatedAt is when the standing order was created.
	CreatedAt Time `json:"createdAt,omitempty"`
	// UpdatedAt is when the standing order was last synced.
	UpdatedAt Time `json:"updatedAt,omitempty"`
}

// StandingOrderItem is one occurrence of a standing order.
type StandingOrderItem struct {
	// ID is the item identifier.
	ID string `json:"id,omitempty"`
	// StandingOrderID is the owning standing order.
	StandingOrderID string `json:"standingOrderId,omitempty"`
	// AlignedDate is the date after weekend/holiday alignment.
	AlignedDate Time `json:"alignedDate,omitempty"`
	// OriginalDate is the date the schedule produced.
	OriginalDate Time `json:"originalDate,omitempty"`
	// PaidDate is when the occurrence was paid.
	PaidDate Time `json:"paidDate,omitempty"`
	// PaidFromAppDate is when the occurrence was marked paid in the app.
	PaidFromAppDate Time `json:"paidFromAppDate,omitempty"`
	// Dismissed reports whether the occurrence was dismissed.
	Dismissed bool `json:"dismissed,omitempty"`
	// RecordIDs are the records that settled this occurrence.
	RecordIDs []string `json:"recordIds,omitempty"`
}

// Paid reports whether the occurrence has been settled.
func (s StandingOrderItem) Paid() bool { return s.PaidDate.Valid() || len(s.RecordIDs) > 0 }

// ---------------------------------------------------------------------------
// References
// ---------------------------------------------------------------------------

// ReferenceResult lists the entities of one kind that point at a target.
type ReferenceResult struct {
	// Field is the field carrying the reference, e.g. "categoryId".
	Field string `json:"field,omitempty"`
	// HasMore reports that Total exceeds Limit.
	HasMore bool `json:"hasMore,omitempty"`
	// IDs are the referencing entity IDs, up to Limit of them.
	IDs []string `json:"ids,omitempty"`
	// Limit is the maximum number of IDs returned.
	Limit int `json:"limit,omitempty"`
	// Total is the full count of references.
	Total int `json:"total,omitempty"`
}

// ReferencesResponse is one entry of the references map: either the entities
// pointing at the target, or an error when the ID could not be resolved.
type ReferencesResponse struct {
	// ActualType names the type the ID really belongs to, on a type_mismatch.
	ActualType string `json:"actualType,omitempty"`
	// Error is "not found" or "type_mismatch" when the ID did not resolve.
	Error string `json:"error,omitempty"`
	// Message explains a type_mismatch in human terms.
	Message string `json:"message,omitempty"`
	// Budgets are the budgets referencing the target.
	Budgets *ReferenceResult `json:"budgets,omitempty"`
	// RecordRules are the record rules referencing the target.
	RecordRules *ReferenceResult `json:"recordRules,omitempty"`
	// Records are the records referencing the target.
	Records *ReferenceResult `json:"records,omitempty"`
	// StandingOrders are the standing orders referencing the target.
	StandingOrders *ReferenceResult `json:"standingOrders,omitempty"`
	// Other holds references the API reports under "other", such as Board
	// template documents. It is untyped because its shape is Board-specific.
	Other *ReferenceResult `json:"other,omitempty"`
}

// Resolved reports whether the ID was found and is of the requested type.
func (r ReferencesResponse) Resolved() bool { return r.Error == "" }

// TotalReferences sums every reference count in the entry. Zero means the
// entity is safe to delete.
func (r ReferencesResponse) TotalReferences() int {
	total := 0
	for _, res := range []*ReferenceResult{r.Budgets, r.RecordRules, r.Records, r.StandingOrders, r.Other} {
		if res != nil {
			total += res.Total
		}
	}
	return total
}

// ---------------------------------------------------------------------------
// API usage
// ---------------------------------------------------------------------------

// APIUsageStatsEntry is one bucket of the usage report. The range is
// half-open: From is inclusive, To exclusive.
type APIUsageStatsEntry struct {
	// From is the inclusive start of the bucket.
	From Time `json:"from"`
	// To is the exclusive end of the bucket.
	To Time `json:"to"`
	// Total is the number of requests in the bucket.
	Total int `json:"total"`
}

// APIUsageStats is the usage report for the authenticated client. Buckets
// with zero usage are omitted.
type APIUsageStats struct {
	responseMeta
	// Period is the period that was requested, e.g. "30days".
	Period string `json:"period"`
	// Granularity is the bucket size.
	Granularity Granularity `json:"granularity"`
	// From is the inclusive start of the whole range.
	From Time `json:"from"`
	// To is the exclusive end of the whole range.
	To Time `json:"to"`
	// Total is the number of requests over the whole range.
	Total int `json:"total"`
	// Usage holds the per-bucket totals.
	Usage []APIUsageStatsEntry `json:"usage"`
}

// ---------------------------------------------------------------------------
// Batch summaries
// ---------------------------------------------------------------------------

// BatchSummary summarises a create or delete batch.
//
// DocumentsWritten is not a copy of Succeeded. On delete a repeated ID is
// idempotent, so Succeeded counts input rows while DocumentsWritten counts
// distinct documents actually removed — a retried delete of an already-deleted
// document succeeds with zero written. On create it can exceed Total, because
// an auto-created transfer mirror or a rewritten pairing target is a document
// without being an item.
type BatchSummary struct {
	// Total is the number of items in the request.
	Total int `json:"total"`
	// Succeeded is the number of items processed successfully.
	Succeeded int `json:"succeeded"`
	// ClientErrors is the number of items that failed on bad input.
	ClientErrors int `json:"clientErrors"`
	// ServerErrors is the number of items that failed internally.
	ServerErrors int `json:"serverErrors"`
	// DocumentsWritten is the number of distinct documents written or removed.
	DocumentsWritten int `json:"documentsWritten"`
}

// Failed is the number of items that did not succeed.
func (s BatchSummary) Failed() int { return s.ClientErrors + s.ServerErrors }

// AllSucceeded reports whether every item succeeded.
func (s BatchSummary) AllSucceeded() bool { return s.Failed() == 0 && s.Total > 0 }

// AllFailed reports whether no item succeeded.
func (s BatchSummary) AllFailed() bool { return s.Total > 0 && s.Succeeded == 0 }

// Partial reports whether the batch mixed successes and failures.
func (s BatchSummary) Partial() bool { return s.Succeeded > 0 && s.Failed() > 0 }

// PatchSummary summarises an update batch.
//
// DocumentsWritten is independent of item success: an item that failed after
// its document committed still counts, and a no-op item succeeds with zero. On
// records it can exceed Total, because severing, orphaning or propagating to a
// mirror each write another document.
type PatchSummary = BatchSummary

// ---------------------------------------------------------------------------
// Per-item batch results
// ---------------------------------------------------------------------------

// ItemError is the failure half of a per-item batch result.
type ItemError struct {
	// Error is the failure message. Empty on success.
	Error string `json:"error,omitempty"`
	// ErrorType classifies the failure as client- or server-side.
	ErrorType ErrorType `json:"errorType,omitempty"`
	// Fields names the input fields the error is attributable to. It is absent
	// when the error belongs to the item as a whole.
	Fields []string `json:"fields,omitempty"`
}

// IsClientError reports whether the item failed on bad input.
func (e ItemError) IsClientError() bool { return e.ErrorType == ClientError }

// IsServerError reports whether the item failed internally.
func (e ItemError) IsServerError() bool { return e.ErrorType == ServerError }

// ResultBase is the part every per-item batch result shares.
type ResultBase struct {
	ItemError
	// ID is the entity identifier the result concerns.
	ID string `json:"id,omitempty"`
	// InputIndex is the 0-based position of the input item this result answers.
	// It is the uniform correlation key across every write endpoint.
	InputIndex int `json:"inputIndex"`
	// Success reports whether the item was processed.
	Success bool `json:"success"`
}

// CreateRecordResult is one entry of a POST /records response.
type CreateRecordResult struct {
	ResultBase
	// Record is the created record. Absent when returnData=false.
	Record *Record `json:"record,omitempty"`
	// CreatedMirrorRecordID is the auto-created B-side of a paired transfer,
	// present only when this call wrote it. It survives returnData=false.
	CreatedMirrorRecordID string `json:"createdMirrorRecordId,omitempty"`
}

// PatchRecordResult is one entry of a PATCH /records response.
type PatchRecordResult struct {
	ResultBase
	// Record is the updated record. Absent when returnData=false.
	Record *Record `json:"record,omitempty"`
}

// PatchAccountResult is one entry of a PATCH /accounts response.
type PatchAccountResult struct {
	ResultBase
	// Account is the updated account. Absent when returnData=false.
	Account *Account `json:"account,omitempty"`
}

// PatchBudgetResult is one entry of a PATCH /budgets response.
type PatchBudgetResult struct {
	ResultBase
	// Budget is the updated budget. Absent when returnData=false.
	Budget *Budget `json:"budget,omitempty"`
}

// PatchCategoryResult is one entry of a PATCH /categories response.
type PatchCategoryResult struct {
	ResultBase
	// Category is the updated category. Absent when returnData=false.
	Category *Category `json:"category,omitempty"`
}

// PatchLabelResult is one entry of a PATCH /labels response.
type PatchLabelResult struct {
	ResultBase
	// Label is the updated label. Absent when returnData=false.
	Label *Label `json:"label,omitempty"`
}

// DeleteResult is one entry of a DELETE /{type} response.
type DeleteResult struct {
	ResultBase
}

// ---------------------------------------------------------------------------
// Response envelopes
// ---------------------------------------------------------------------------

// AccountsPage is the GET /accounts response.
type AccountsPage struct {
	responseMeta
	Pagination
	// Accounts are the accounts on this page.
	Accounts []Account `json:"accounts"`
	// AgentHints are the hints, when agentHints=true was requested.
	AgentHints AgentHints `json:"agentHints,omitempty"`
}

// BudgetsPage is the GET /budgets response.
type BudgetsPage struct {
	responseMeta
	Pagination
	// Budgets are the budgets on this page.
	Budgets []Budget `json:"budgets"`
	// AgentHints are the hints, when agentHints=true was requested.
	AgentHints AgentHints `json:"agentHints,omitempty"`
}

// CategoriesPage is the GET /categories response.
type CategoriesPage struct {
	responseMeta
	Pagination
	// Categories are the categories on this page.
	Categories []Category `json:"categories"`
	// AgentHints are the hints, when agentHints=true was requested.
	AgentHints AgentHints `json:"agentHints,omitempty"`
}

// GoalsPage is the GET /goals response.
type GoalsPage struct {
	responseMeta
	Pagination
	// Goals are the goals on this page.
	Goals []Goal `json:"goals"`
	// AgentHints are the hints, when agentHints=true was requested.
	AgentHints AgentHints `json:"agentHints,omitempty"`
}

// LabelsPage is the GET /labels response.
type LabelsPage struct {
	responseMeta
	Pagination
	// Labels are the labels on this page.
	Labels []Label `json:"labels"`
	// AgentHints are the hints, when agentHints=true was requested.
	AgentHints AgentHints `json:"agentHints,omitempty"`
}

// RecordRulesPage is the GET /record-rules response.
type RecordRulesPage struct {
	responseMeta
	Pagination
	// RecordRules are the rules on this page.
	RecordRules []RecordRule `json:"recordRules"`
	// AgentHints are the hints, when agentHints=true was requested.
	AgentHints AgentHints `json:"agentHints,omitempty"`
}

// RecordsPage is the GET /records response.
type RecordsPage struct {
	responseMeta
	Pagination
	// Records are the records on this page.
	Records []Record `json:"records"`
	// AppliedRecordDateFilters is the recordDate filter the server actually
	// applied, including the implicit three-month default. It is nil when no
	// date filter applied at all, which happens on ID-based lookups.
	AppliedRecordDateFilters []string `json:"appliedRecordDateFilters,omitempty"`
	// AgentHints are the hints, when agentHints=true was requested.
	AgentHints AgentHints `json:"agentHints,omitempty"`
}

// DefaultWindowApplied reports whether the server fell back to its implicit
// three-month recordDate window because the request carried no bound.
func (p RecordsPage) DefaultWindowApplied(requested bool) bool {
	return !requested && len(p.AppliedRecordDateFilters) > 0
}

// StandingOrdersPage is the GET /standing-orders response.
type StandingOrdersPage struct {
	responseMeta
	Pagination
	// StandingOrders are the standing orders on this page.
	StandingOrders []StandingOrder `json:"standingOrders"`
	// AgentHints are the hints, when agentHints=true was requested.
	AgentHints AgentHints `json:"agentHints,omitempty"`
}

// StandingOrderItemsPage is the GET /standing-orders/items response.
type StandingOrderItemsPage struct {
	responseMeta
	Pagination
	// StandingOrderItems are the occurrences on this page.
	StandingOrderItems []StandingOrderItem `json:"standingOrderItems"`
	// AgentHints are the hints, when agentHints=true was requested.
	AgentHints AgentHints `json:"agentHints,omitempty"`
}

// CreateAccountResponse is the POST /accounts response.
type CreateAccountResponse struct {
	responseMeta
	// Account is the created account.
	Account *Account `json:"account,omitempty"`
	// Summary is the standard single-write summary.
	Summary *BatchSummary `json:"summary,omitempty"`
	// AgentHints are the hints, when agentHints=true was requested.
	AgentHints AgentHints `json:"agentHints,omitempty"`
}

// CreateBudgetResponse is the POST /budgets response.
type CreateBudgetResponse struct {
	responseMeta
	// Budget is the created budget.
	Budget *Budget `json:"budget,omitempty"`
	// Summary is the standard single-write summary.
	Summary *BatchSummary `json:"summary,omitempty"`
	// AgentHints are the hints, when agentHints=true was requested.
	AgentHints AgentHints `json:"agentHints,omitempty"`
}

// CreateCategoryResponse is the POST /categories/custom response.
type CreateCategoryResponse struct {
	responseMeta
	// Category is the created subcategory.
	Category *Category `json:"category,omitempty"`
	// Summary is the standard single-write summary.
	Summary *BatchSummary `json:"summary,omitempty"`
	// AgentHints are the hints, when agentHints=true was requested.
	AgentHints AgentHints `json:"agentHints,omitempty"`
}

// CreateLabelResponse is the POST /labels response.
type CreateLabelResponse struct {
	responseMeta
	// Label is the created label.
	Label *Label `json:"label,omitempty"`
	// Summary is the standard single-write summary.
	Summary *BatchSummary `json:"summary,omitempty"`
	// AgentHints are the hints, when agentHints=true was requested.
	AgentHints AgentHints `json:"agentHints,omitempty"`
}

// CreateRecordsResponse is the POST /records response. It arrives on 200,
// 207, 400 and 500 alike — read Meta.StatusCode and Summary to tell them
// apart.
type CreateRecordsResponse struct {
	responseMeta
	// Results holds one entry per input record, correlated by InputIndex.
	Results []CreateRecordResult `json:"results"`
	// Summary counts the outcomes.
	Summary BatchSummary `json:"summary"`
	// AgentHints are the hints, when agentHints=true was requested.
	AgentHints AgentHints `json:"agentHints,omitempty"`
}

// PatchRecordsResponse is the PATCH /records response.
type PatchRecordsResponse struct {
	responseMeta
	// Results holds one entry per input item, correlated by InputIndex.
	Results []PatchRecordResult `json:"results"`
	// Summary counts the outcomes.
	Summary PatchSummary `json:"summary"`
	// AgentHints are the hints, when agentHints=true was requested.
	AgentHints AgentHints `json:"agentHints,omitempty"`
}

// PatchAccountsResponse is the PATCH /accounts response.
type PatchAccountsResponse struct {
	responseMeta
	// Results holds one entry per input item, correlated by InputIndex.
	Results []PatchAccountResult `json:"results"`
	// Summary counts the outcomes.
	Summary PatchSummary `json:"summary"`
	// AgentHints are the hints, when agentHints=true was requested.
	AgentHints AgentHints `json:"agentHints,omitempty"`
}

// PatchBudgetsResponse is the PATCH /budgets response.
type PatchBudgetsResponse struct {
	responseMeta
	// Results holds one entry per input item, correlated by InputIndex.
	Results []PatchBudgetResult `json:"results"`
	// Summary counts the outcomes.
	Summary PatchSummary `json:"summary"`
	// AgentHints are the hints, when agentHints=true was requested.
	AgentHints AgentHints `json:"agentHints,omitempty"`
}

// PatchCategoriesResponse is the PATCH /categories response.
type PatchCategoriesResponse struct {
	responseMeta
	// Results holds one entry per input item, correlated by InputIndex.
	Results []PatchCategoryResult `json:"results"`
	// Summary counts the outcomes.
	Summary PatchSummary `json:"summary"`
	// AgentHints are the hints, when agentHints=true was requested.
	AgentHints AgentHints `json:"agentHints,omitempty"`
}

// PatchLabelsResponse is the PATCH /labels response.
type PatchLabelsResponse struct {
	responseMeta
	// Results holds one entry per input item, correlated by InputIndex.
	Results []PatchLabelResult `json:"results"`
	// Summary counts the outcomes.
	Summary PatchSummary `json:"summary"`
	// AgentHints are the hints, when agentHints=true was requested.
	AgentHints AgentHints `json:"agentHints,omitempty"`
}

// DeleteResponse is the DELETE /{type} response.
type DeleteResponse struct {
	responseMeta
	// Results holds one entry per input ID, correlated by InputIndex.
	Results []DeleteResult `json:"results"`
	// Summary counts the outcomes.
	Summary BatchSummary `json:"summary"`
	// AgentHints are the hints, when agentHints=true was requested.
	AgentHints AgentHints `json:"agentHints,omitempty"`
}

// ReferencesResult is the GET /{type}/references response: a map from entity
// ID to the references pointing at it, or to a per-ID error.
type ReferencesResult struct {
	responseMeta
	// Entries maps each requested ID to its references or its error.
	Entries map[string]ReferencesResponse `json:"-"`
}

// UnmarshalJSON decodes the bare map the endpoint returns.
func (r *ReferencesResult) UnmarshalJSON(b []byte) error {
	var entries map[string]ReferencesResponse
	if err := json.Unmarshal(b, &entries); err != nil {
		return err
	}
	r.Entries = entries
	return nil
}

// MarshalJSON re-emits the bare map, so a round trip is lossless.
func (r ReferencesResult) MarshalJSON() ([]byte, error) {
	if r.Entries == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(r.Entries)
}

// Unresolved lists the requested IDs the server could not resolve, with the
// reason it gave. It is empty when every ID resolved.
func (r ReferencesResult) Unresolved() map[string]string {
	out := map[string]string{}
	for id, entry := range r.Entries {
		if entry.Error != "" {
			out[id] = entry.Error
		}
	}
	return out
}

// SafeToDelete lists the requested IDs that resolved and have no references,
// so they can go straight to Delete.
func (r ReferencesResult) SafeToDelete() []string {
	var out []string
	for id, entry := range r.Entries {
		if entry.Resolved() && entry.TotalReferences() == 0 {
			out = append(out, id)
		}
	}
	return out
}
