package wallet

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// ---------------------------------------------------------------------------
// Tolerant time types
// ---------------------------------------------------------------------------

// timeLayouts are tried in order when decoding a timestamp. The API documents
// RFC 3339 with millisecond precision, but real payloads also carry
// second-precision stamps, offsets other than Z, and bare dates.
var timeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05.999999999",
	"2006-01-02T15:04:05",
	"2006-01-02",
}

// Time is an RFC 3339 timestamp that decodes leniently and encodes canonically.
//
// It accepts null, "" and every layout in timeLayouts; a null or empty value
// leaves the Time zero, which Valid reports. It always encodes as RFC 3339
// with millisecond precision in UTC, the form the API emits.
type Time struct {
	time.Time
}

// NewTime wraps a time.Time.
func NewTime(t time.Time) Time { return Time{Time: t} }

// Valid reports whether a timestamp was actually present.
func (t Time) Valid() bool { return !t.Time.IsZero() }

// UnmarshalJSON accepts null, an empty string, or any supported layout.
func (t *Time) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		t.Time = time.Time{}
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("wallet: timestamp must be a JSON string, got %s", truncate(string(b), 64))
	}
	if strings.TrimSpace(s) == "" {
		t.Time = time.Time{}
		return nil
	}
	for _, layout := range timeLayouts {
		if parsed, err := time.Parse(layout, s); err == nil {
			t.Time = parsed
			return nil
		}
	}
	return fmt.Errorf("wallet: cannot parse timestamp %q", truncate(s, 64))
}

// MarshalJSON writes RFC 3339 with millisecond precision, or null when zero.
func (t Time) MarshalJSON() ([]byte, error) {
	if t.Time.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(t.Time.UTC().Format("2006-01-02T15:04:05.000Z"))
}

// String renders the API's wire format, or "" when zero.
func (t Time) String() string {
	if t.Time.IsZero() {
		return ""
	}
	return t.Time.UTC().Format("2006-01-02T15:04:05.000Z")
}

// Date is a calendar date (YYYY-MM-DD) — budget startDate, endDate and
// closedDate. It decodes a full timestamp too, keeping only the date part.
type Date struct {
	time.Time
}

// NewDate builds a Date from Y/M/D.
func NewDate(year int, month time.Month, day int) Date {
	return Date{Time: time.Date(year, month, day, 0, 0, 0, 0, time.UTC)}
}

// Valid reports whether a date was actually present.
func (d Date) Valid() bool { return !d.Time.IsZero() }

// UnmarshalJSON accepts null, "", "YYYY-MM-DD" or a full timestamp.
func (d *Date) UnmarshalJSON(b []byte) error {
	var t Time
	if err := t.UnmarshalJSON(b); err != nil {
		return err
	}
	if t.Time.IsZero() {
		d.Time = time.Time{}
		return nil
	}
	y, m, day := t.Time.UTC().Date()
	d.Time = time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
	return nil
}

// MarshalJSON writes "YYYY-MM-DD", or null when zero.
func (d Date) MarshalJSON() ([]byte, error) {
	if d.Time.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(d.String())
}

// String renders "YYYY-MM-DD", or "" when zero.
func (d Date) String() string {
	if d.Time.IsZero() {
		return ""
	}
	return d.Time.UTC().Format("2006-01-02")
}

// truncate caps a string at n bytes without splitting a rune, so an error
// message built from a server body is always valid UTF-8.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := n
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "…"
}

// ---------------------------------------------------------------------------
// Enumerations
//
// Every enum decodes any string: the API can add members, and a client that
// rejects an unknown one is a client that breaks on the next release. Use
// Valid to check a value against the members this SDK knows about; the
// request-side validators do exactly that before sending.
// ---------------------------------------------------------------------------

// AccountType is the kind of a financial account.
type AccountType string

// Account types. The first five are creatable; the rest arrive only via bank
// sync or the mobile apps and are read-only over REST.
const (
	AccountTypeGeneral        AccountType = "General"
	AccountTypeCash           AccountType = "Cash"
	AccountTypeCurrentAccount AccountType = "CurrentAccount"
	AccountTypeSavingAccount  AccountType = "SavingAccount"
	AccountTypeInsurance      AccountType = "Insurance"
	AccountTypeCreditCard     AccountType = "CreditCard"
	AccountTypeBonus          AccountType = "Bonus"
	AccountTypeInvestment     AccountType = "Investment"
	AccountTypeLoan           AccountType = "Loan"
	AccountTypeMortgage       AccountType = "Mortgage"
	AccountTypeOverdraft      AccountType = "Overdraft"
)

// AllAccountTypes lists every type the API can return.
var AllAccountTypes = []AccountType{
	AccountTypeGeneral, AccountTypeCash, AccountTypeCurrentAccount, AccountTypeCreditCard,
	AccountTypeSavingAccount, AccountTypeBonus, AccountTypeInsurance, AccountTypeInvestment,
	AccountTypeLoan, AccountTypeMortgage, AccountTypeOverdraft,
}

// CreatableAccountTypes lists the types POST /accounts accepts.
var CreatableAccountTypes = []AccountType{
	AccountTypeGeneral, AccountTypeCash, AccountTypeCurrentAccount,
	AccountTypeSavingAccount, AccountTypeInsurance,
}

// Valid reports whether the value is a type the API can return.
func (a AccountType) Valid() bool { return containsEnum(AllAccountTypes, a) }

// Creatable reports whether POST /accounts accepts this type.
func (a AccountType) Creatable() bool { return containsEnum(CreatableAccountTypes, a) }

// UniversalColor is one of the 16 colour names the write endpoints accept.
// Read endpoints return a hex string instead, so Account.Color is a string.
type UniversalColor string

// The 16 universal colours.
const (
	ColorBlue        UniversalColor = "Blue"
	ColorCharcoal    UniversalColor = "Charcoal"
	ColorCoffeeBrown UniversalColor = "CoffeeBrown"
	ColorDarkPurple  UniversalColor = "DarkPurple"
	ColorGreen       UniversalColor = "Green"
	ColorLightBlue   UniversalColor = "LightBlue"
	ColorLime        UniversalColor = "Lime"
	ColorOrange      UniversalColor = "Orange"
	ColorPineGreen   UniversalColor = "PineGreen"
	ColorPink        UniversalColor = "Pink"
	ColorPurple      UniversalColor = "Purple"
	ColorRed         UniversalColor = "Red"
	ColorStone       UniversalColor = "Stone"
	ColorTurquoise   UniversalColor = "Turquoise"
	ColorWine        UniversalColor = "Wine"
	ColorYellow      UniversalColor = "Yellow"
)

// AllUniversalColors lists every colour name the write endpoints accept.
var AllUniversalColors = []UniversalColor{
	ColorBlue, ColorCharcoal, ColorCoffeeBrown, ColorDarkPurple, ColorGreen, ColorLightBlue,
	ColorLime, ColorOrange, ColorPineGreen, ColorPink, ColorPurple, ColorRed, ColorStone,
	ColorTurquoise, ColorWine, ColorYellow,
}

// Valid reports whether the colour is one of the 16 accepted names.
func (c UniversalColor) Valid() bool { return containsEnum(AllUniversalColors, c) }

// Cardinality is a category's spending nature.
type Cardinality string

// Cardinality values.
const (
	CardinalityNone Cardinality = "none"
	CardinalityMust Cardinality = "must"
	CardinalityNeed Cardinality = "need"
	CardinalityWant Cardinality = "want"
)

// AllCardinalities lists every cardinality.
var AllCardinalities = []Cardinality{CardinalityNone, CardinalityMust, CardinalityNeed, CardinalityWant}

// Valid reports whether the cardinality is known.
func (c Cardinality) Valid() bool { return containsEnum(AllCardinalities, c) }

// BudgetType is a budget's interval.
type BudgetType string

// Budget interval types. BudgetAll is read-only legacy and cannot be created.
const (
	BudgetIntervalWeek  BudgetType = "BUDGET_INTERVAL_WEEK"
	BudgetIntervalMonth BudgetType = "BUDGET_INTERVAL_MONTH"
	BudgetIntervalYear  BudgetType = "BUDGET_INTERVAL_YEAR"
	BudgetCustom        BudgetType = "BUDGET_CUSTOM"
	BudgetAll           BudgetType = "BUDGET_ALL"
)

// AllBudgetTypes lists every type the API can return.
var AllBudgetTypes = []BudgetType{BudgetIntervalWeek, BudgetIntervalMonth, BudgetIntervalYear, BudgetCustom, BudgetAll}

// CreatableBudgetTypes lists the types POST /budgets accepts.
var CreatableBudgetTypes = []BudgetType{BudgetIntervalWeek, BudgetIntervalMonth, BudgetIntervalYear, BudgetCustom}

// Valid reports whether the value is a type the API can return.
func (b BudgetType) Valid() bool { return containsEnum(AllBudgetTypes, b) }

// Creatable reports whether POST /budgets accepts this type.
func (b BudgetType) Creatable() bool { return containsEnum(CreatableBudgetTypes, b) }

// RecordState is a record's clearing state.
type RecordState string

// Record states. Void and WaitForAssign are output-only.
const (
	RecordStateReconciled    RecordState = "reconciled"
	RecordStateCleared       RecordState = "cleared"
	RecordStateUncleared     RecordState = "uncleared"
	RecordStateVoid          RecordState = "void"
	RecordStateWaitForAssign RecordState = "waitForAssign"
)

// AllRecordStates lists every state the API can return.
var AllRecordStates = []RecordState{
	RecordStateReconciled, RecordStateCleared, RecordStateUncleared,
	RecordStateVoid, RecordStateWaitForAssign,
}

// WritableRecordStates lists the states the write endpoints and the
// recordState filter accept.
var WritableRecordStates = []RecordState{RecordStateReconciled, RecordStateCleared, RecordStateUncleared}

// Valid reports whether the value is a state the API can return.
func (r RecordState) Valid() bool { return containsEnum(AllRecordStates, r) }

// Writable reports whether the write endpoints accept this state.
func (r RecordState) Writable() bool { return containsEnum(WritableRecordStates, r) }

// RecordType is the direction of a record, derived from the amount sign.
type RecordType string

// Record directions.
const (
	RecordTypeIncome  RecordType = "income"
	RecordTypeExpense RecordType = "expense"
)

// AllRecordTypes lists both directions.
var AllRecordTypes = []RecordType{RecordTypeIncome, RecordTypeExpense}

// Valid reports whether the record type is known.
func (r RecordType) Valid() bool { return containsEnum(AllRecordTypes, r) }

// RecordSource identifies the client that created a record.
type RecordSource string

// Record creation sources.
const (
	SourceAndroid RecordSource = "android"
	SourceIOS     RecordSource = "ios"
	SourceWeb     RecordSource = "web"
	SourceREST    RecordSource = "rest"
	SourceMCP     RecordSource = "mcp"
	SourceBackend RecordSource = "backend"
	SourceMissing RecordSource = "missing"
)

// AllRecordSources lists every source the API can return or filter on.
var AllRecordSources = []RecordSource{
	SourceAndroid, SourceIOS, SourceWeb, SourceREST, SourceMCP, SourceBackend, SourceMissing,
}

// Valid reports whether the source is known.
func (s RecordSource) Valid() bool { return containsEnum(AllRecordSources, s) }

// PairingMode says how a created record binds to its transfer counterpart.
type PairingMode string

// Transfer pairing modes.
const (
	// PairingNew creates a mirror record on the target account.
	PairingNew PairingMode = "new"
	// PairingExisting binds to an existing record on another account.
	PairingExisting PairingMode = "existing"
	// PairingUnpaired records a transfer with no mirror.
	PairingUnpaired PairingMode = "unpaired"
)

// AllPairingModes lists every pairing mode.
var AllPairingModes = []PairingMode{PairingNew, PairingExisting, PairingUnpaired}

// Valid reports whether the pairing mode is known.
func (p PairingMode) Valid() bool { return containsEnum(AllPairingModes, p) }

// GoalState is the lifecycle state of a savings goal.
type GoalState string

// Goal states.
const (
	GoalActive  GoalState = "active"
	GoalPaused  GoalState = "paused"
	GoalReached GoalState = "reached"
)

// AllGoalStates lists every goal state.
var AllGoalStates = []GoalState{GoalActive, GoalPaused, GoalReached}

// Valid reports whether the goal state is known.
func (g GoalState) Valid() bool { return containsEnum(AllGoalStates, g) }

// Reminder is a standing order's notification lead time.
type Reminder string

// Reminder lead times.
const (
	ReminderDueToday Reminder = "due_today"
	ReminderNone     Reminder = "none"
	Reminder1Day     Reminder = "1_day"
	Reminder3Days    Reminder = "3_days"
	Reminder7Days    Reminder = "7_days"
)

// AllReminders lists every reminder value.
var AllReminders = []Reminder{ReminderDueToday, ReminderNone, Reminder1Day, Reminder3Days, Reminder7Days}

// Valid reports whether the reminder is known.
func (r Reminder) Valid() bool { return containsEnum(AllReminders, r) }

// ErrorType classifies a per-item batch failure.
type ErrorType string

// Batch item error classifications.
const (
	// ClientError means bad input: fixing the item and resending can work.
	ClientError ErrorType = "client_error"
	// ServerError means an internal failure: resending the same item can work.
	ServerError ErrorType = "server_error"
)

// BalanceMode names the formula variant used to compute an account balance.
type BalanceMode string

// Balance computation modes.
const (
	BalanceStandard                BalanceMode = "standard"
	BalanceCreditCardManual        BalanceMode = "creditCardManual"
	BalanceCreditCardManualInverse BalanceMode = "creditCardManualInverse"
	BalanceCreditCardBank          BalanceMode = "creditCardBank"
	BalanceCreditCardBankReversed  BalanceMode = "creditCardBankReversed"
	BalanceOverdraftManual         BalanceMode = "overdraftManual"
	BalanceOverdraftBank           BalanceMode = "overdraftBank"
)

// HintSeverity indicates how much an agent hint matters.
type HintSeverity string

// Agent hint severities.
const (
	HintInfo        HintSeverity = "info"
	HintWarning     HintSeverity = "warning"
	HintInstruction HintSeverity = "instruction"
)

// HintType is the dot-notation category of an agent hint.
type HintType string

// Agent hint categories.
const (
	HintPaginationHasMore  HintType = "pagination.has_more"
	HintResultPartialMatch HintType = "result.partial_match"
	HintResultEmpty        HintType = "result.empty"
	HintRateLimitWarning   HintType = "rate_limit.warning"
)

// Granularity is the bucket size of an API usage report.
type Granularity string

// Usage report granularities.
const (
	GranularityDaily   Granularity = "daily"
	GranularityWeekly  Granularity = "weekly"
	GranularityMonthly Granularity = "monthly"
)

// EntityType names a collection in the /{type} path of the delete and
// references endpoints.
type EntityType string

// Entity collections. EntityOrders is Board-only; Wallet answers 404.
const (
	EntityAccounts       EntityType = "accounts"
	EntityBudgets        EntityType = "budgets"
	EntityCategories     EntityType = "categories"
	EntityLabels         EntityType = "labels"
	EntityRecords        EntityType = "records"
	EntityRecordRules    EntityType = "record-rules"
	EntityStandingOrders EntityType = "standing-orders"
	EntityOrders         EntityType = "orders"
)

// DeletableEntityTypes lists every type DELETE /v1/api/{type} accepts.
var DeletableEntityTypes = []EntityType{
	EntityAccounts, EntityBudgets, EntityCategories, EntityLabels,
	EntityRecords, EntityRecordRules, EntityStandingOrders, EntityOrders,
}

// ReferenceableEntityTypes lists every type GET /v1/api/{type}/references
// supports. Budgets, standing orders and record rules are not referenced by
// anything, so they are absent by design.
var ReferenceableEntityTypes = []EntityType{
	EntityCategories, EntityRecords, EntityAccounts, EntityLabels, EntityOrders,
}

// NeedsReferenceCheck reports whether the type can be referenced by other
// entities, and so should be passed through References before a delete.
func (e EntityType) NeedsReferenceCheck() bool { return containsEnum(ReferenceableEntityTypes, e) }

// Deletable reports whether DELETE /v1/api/{type} accepts this type.
func (e EntityType) Deletable() bool { return containsEnum(DeletableEntityTypes, e) }

// singularEntityTypes maps the singular forms the references endpoint
// normalises onto their canonical plural.
var singularEntityTypes = map[string]EntityType{
	"category": EntityCategories,
	"record":   EntityRecords,
	"account":  EntityAccounts,
	"label":    EntityLabels,
	"order":    EntityOrders,
}

// NormalizeEntityType folds an accepted singular form onto its canonical
// plural, leaving anything else untouched.
func NormalizeEntityType(t EntityType) EntityType {
	if canonical, ok := singularEntityTypes[strings.ToLower(string(t))]; ok {
		return canonical
	}
	return t
}

// ValidationMode selects how PATCH /records treats invalid items.
type ValidationMode string

// Patch validation modes.
const (
	// ValidationLenient processes the valid items and skips the invalid ones.
	// This is the server's default.
	ValidationLenient ValidationMode = "lenient"
	// ValidationStrict validates everything first: one client error and the
	// whole call returns 400 with no writes.
	ValidationStrict ValidationMode = "strict"
)

// AllValidationModes lists both modes.
var AllValidationModes = []ValidationMode{ValidationLenient, ValidationStrict}

// Valid reports whether the validation mode is known.
func (v ValidationMode) Valid() bool { return containsEnum(AllValidationModes, v) }

// Category IDs the API guarantees are stable across every account.
const (
	// CategoryUnknownIncome is the fixed ID of the "Unknown income" category.
	CategoryUnknownIncome = "5c5c32c8-0082-8000-8000-000000000000"
	// CategoryUnknownExpense is the fixed ID of the "Unknown expense" category.
	CategoryUnknownExpense = "5c5c32c9-0082-8000-8000-000000000000"
	// CategoryUncategorized is the fixed ID of the "Uncategorized" category.
	CategoryUncategorized = "5c5c4e23-00c8-8000-8000-000000000000"
	// CategoryTransfer is the fixed, auto-managed ID of the "Transfer" category.
	CategoryTransfer = "5c5c4e21-00c8-8000-8000-000000000000"
	// CategoryUnknown is the magic categoryId filter value that expands to
	// Unknown income, Unknown expense and Uncategorized.
	CategoryUnknown = "unknown"
)

func containsEnum[T ~string](all []T, v T) bool {
	for _, candidate := range all {
		if candidate == v {
			return true
		}
	}
	return false
}

func enumNames[T ~string](all []T) string {
	parts := make([]string, len(all))
	for i, v := range all {
		parts[i] = string(v)
	}
	return strings.Join(parts, ", ")
}
