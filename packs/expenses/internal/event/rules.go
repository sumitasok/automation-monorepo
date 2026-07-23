// Rule evaluation for the shared expense-rules.yaml file (spec
// specs/002-expense-rules-engine, in the parent workspace repo). A confirmed
// rule match with event_relevance "routine" marks a transaction as
// intentionally not event-worthy directly, without calling the AI matcher —
// see Run() in updateevent.go for where this is consulted.
//
// This is an independent copy of packs/gmail/categorize/rules.go's exact
// shape (ExpenseRule/ExpenseRules/MatchCondition/Outcome, ordered,
// first-match-wins, graceful missing-file degrade), duplicated rather than
// imported because packs/gmail and packs/expenses are independently-versioned
// repos (specs/002-expense-rules-engine/research.md Decision 3) — the same
// tradeoff ADR 0011 already made for the DeepSeek-provider Strategy shape.
// Only the outcome fields this pack actually consumes (EventRelevance) are
// used here; Category/SubCategory/Labels are parsed (so both packs agree on
// the file's shape) but ignored on this side.
package event

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Scopes a rule's applies_to value may take.
const (
	ScopeCategorize = "categorize"
	ScopeEvent      = "event"
)

// EventRelevanceRoutine is the only defined value for Outcome.EventRelevance
// today: it tells expenses-update-event a transaction is not event-worthy.
const EventRelevanceRoutine = "routine"

// MatchCondition is the set of conditions a rule may state; all present
// fields AND together (data-model.md). An absent field is not evaluated.
type MatchCondition struct {
	MerchantContains []string  `yaml:"merchant_contains,omitempty"`
	KeywordContains  []string  `yaml:"keyword_contains,omitempty"`
	DayOfWeek        []string  `yaml:"day_of_week,omitempty"`
	TimeBetween      []string  `yaml:"time_between,omitempty"`
	AmountBetween    []float64 `yaml:"amount_between,omitempty"`
}

func (c MatchCondition) isEmpty() bool {
	return len(c.MerchantContains) == 0 && len(c.KeywordContains) == 0 &&
		len(c.DayOfWeek) == 0 && len(c.TimeBetween) == 0 && len(c.AmountBetween) == 0
}

// Outcome is what a matched rule assigns. At least one field must be set.
type Outcome struct {
	Category       string   `yaml:"category,omitempty"`
	SubCategory    string   `yaml:"subcategory,omitempty"`
	Labels         []string `yaml:"labels,omitempty"`
	EventRelevance string   `yaml:"event_relevance,omitempty"`
}

func (o Outcome) isEmpty() bool {
	return o.Category == "" && o.SubCategory == "" && len(o.Labels) == 0 && o.EventRelevance == ""
}

// ExpenseRule is one user-authored rule (data-model.md).
type ExpenseRule struct {
	Name        string         `yaml:"name"`
	Description string         `yaml:"description,omitempty"`
	Enabled     *bool          `yaml:"enabled,omitempty"` // nil => default true
	AppliesTo   []string       `yaml:"applies_to"`
	Match       MatchCondition `yaml:"match"`
	Outcome     Outcome        `yaml:"outcome"`
}

// IsEnabled reports whether the rule should be evaluated (FR-013): true
// unless explicitly disabled.
func (r ExpenseRule) IsEnabled() bool {
	return r.Enabled == nil || *r.Enabled
}

func (r ExpenseRule) appliesToScope(scope string) bool {
	for _, s := range r.AppliesTo {
		if s == scope {
			return true
		}
	}
	return false
}

// ExpenseRules is the top-level structure of expense-rules.yaml.
type ExpenseRules struct {
	Rules []ExpenseRule `yaml:"rules"`
}

// LoadExpenseRules reads and validates path. A missing file degrades to an
// empty rule set (no error), so a workspace with no rules file behaves
// exactly as it did before this feature (spec SC-005).
func LoadExpenseRules(path string) (ExpenseRules, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ExpenseRules{}, nil
		}
		return ExpenseRules{}, fmt.Errorf("reading rules file %q: %w", path, err)
	}
	var rs ExpenseRules
	if err := yaml.Unmarshal(data, &rs); err != nil {
		return ExpenseRules{}, fmt.Errorf("parsing rules file %q: %w", path, err)
	}
	if err := rs.validate(); err != nil {
		return ExpenseRules{}, fmt.Errorf("rules file %q: %w", path, err)
	}
	return rs, nil
}

// validate rejects config errors at load time: a missing/duplicate name, an
// unscoped or mis-scoped rule, an unconditional (empty match) rule, or an
// empty outcome.
func (rs ExpenseRules) validate() error {
	seen := make(map[string]bool, len(rs.Rules))
	for _, r := range rs.Rules {
		if strings.TrimSpace(r.Name) == "" {
			return fmt.Errorf("a rule is missing its required name")
		}
		if seen[r.Name] {
			return fmt.Errorf("rule %q: duplicate name", r.Name)
		}
		seen[r.Name] = true
		if len(r.AppliesTo) == 0 {
			return fmt.Errorf("rule %q: applies_to must list at least one of %q, %q", r.Name, ScopeCategorize, ScopeEvent)
		}
		for _, s := range r.AppliesTo {
			if s != ScopeCategorize && s != ScopeEvent {
				return fmt.Errorf("rule %q: applies_to value %q must be %q or %q", r.Name, s, ScopeCategorize, ScopeEvent)
			}
		}
		if r.Match.isEmpty() {
			return fmt.Errorf("rule %q: match must set at least one condition — an unconditional rule would match every transaction", r.Name)
		}
		if r.Outcome.isEmpty() {
			return fmt.Errorf("rule %q: outcome must set at least one field", r.Name)
		}
		if (r.Outcome.Category != "") != (r.Outcome.SubCategory != "") {
			return fmt.Errorf("rule %q: outcome.category and outcome.subcategory must be set together", r.Name)
		}
		if len(r.Match.TimeBetween) != 0 && len(r.Match.TimeBetween) != 2 {
			return fmt.Errorf("rule %q: match.time_between must have exactly 2 values [start, end]", r.Name)
		}
		if len(r.Match.AmountBetween) != 0 && len(r.Match.AmountBetween) != 2 {
			return fmt.Errorf("rule %q: match.amount_between must have exactly 2 values [min, max]", r.Name)
		}
	}
	return nil
}

// MatchInput is the transaction data a rule's conditions are evaluated
// against. Deliberately separate from the AI-facing Item type (matcher.go).
type MatchInput struct {
	Merchant string
	Info     string
	Subject  string
	Amount   string
	TxnDate  string // "2006-01-02" or "2006-01-02 15:04:05" (gmail's parser.NormaliseDate)
}

// Match returns the first enabled rule (in file order) scoped to scope
// (ScopeCategorize or ScopeEvent) whose match conditions all hold against in,
// or ok=false if none matches (FR-006: first-match-wins precedence).
func (rs ExpenseRules) Match(scope string, in MatchInput) (rule ExpenseRule, ok bool) {
	for _, r := range rs.Rules {
		if !r.IsEnabled() || !r.appliesToScope(scope) {
			continue
		}
		if r.Match.matches(in) {
			return r, true
		}
	}
	return ExpenseRule{}, false
}

// matches reports whether every condition set on c holds against in. Absent
// conditions are vacuously true.
func (c MatchCondition) matches(in MatchInput) bool {
	if len(c.MerchantContains) > 0 && !containsAnyFold(in.Merchant, c.MerchantContains) {
		return false
	}
	if len(c.KeywordContains) > 0 {
		haystack := in.Info + " " + in.Subject
		if !containsAnyFold(haystack, c.KeywordContains) {
			return false
		}
	}
	if len(c.DayOfWeek) > 0 {
		d, ok := parseTxnDate(in.TxnDate)
		if !ok || !dayOfWeekMatches(d, c.DayOfWeek) {
			return false
		}
	}
	if len(c.TimeBetween) == 2 {
		t, hasTime := parseTxnTimeOfDay(in.TxnDate)
		if !hasTime || !timeOfDayBetween(t, c.TimeBetween[0], c.TimeBetween[1]) {
			return false
		}
	}
	if len(c.AmountBetween) == 2 {
		amt, err := strconv.ParseFloat(strings.TrimSpace(in.Amount), 64)
		if err != nil || amt < c.AmountBetween[0] || amt > c.AmountBetween[1] {
			return false
		}
	}
	return true
}

// containsAnyFold reports whether haystack contains any of needles as a
// case-insensitive substring.
func containsAnyFold(haystack string, needles []string) bool {
	h := strings.ToLower(haystack)
	for _, n := range needles {
		if n == "" {
			continue
		}
		if strings.Contains(h, strings.ToLower(n)) {
			return true
		}
	}
	return false
}

// txnDateLayouts are the two shapes the gmail pack's TxnDate normalisation
// produces: date+time when the source bank alert included a time, date-only
// otherwise (specs/002-expense-rules-engine/research.md Decision 7).
var txnDateLayouts = []string{"2006-01-02 15:04:05", "2006-01-02"}

// parseTxnDate parses TxnDate against either known layout, ignoring time of
// day, for day-of-week matching (which only needs the date).
func parseTxnDate(txnDate string) (time.Time, bool) {
	txnDate = strings.TrimSpace(txnDate)
	for _, layout := range txnDateLayouts {
		if t, err := time.Parse(layout, txnDate); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// parseTxnTimeOfDay parses TxnDate and reports its time-of-day, but only
// succeeds when TxnDate actually includes a time component — a date-only
// value fails closed (ok=false) so a time_between condition never silently
// matches transactions with no time data.
func parseTxnTimeOfDay(txnDate string) (t time.Time, ok bool) {
	txnDate = strings.TrimSpace(txnDate)
	t, err := time.Parse("2006-01-02 15:04:05", txnDate)
	return t, err == nil
}

var weekdayAbbrev = map[time.Weekday]string{
	time.Monday:    "mon",
	time.Tuesday:   "tue",
	time.Wednesday: "wed",
	time.Thursday:  "thu",
	time.Friday:    "fri",
	time.Saturday:  "sat",
	time.Sunday:    "sun",
}

// dayOfWeekMatches reports whether d's weekday appears in days (case-insensitive
// mon/tue/wed/thu/fri/sat/sun).
func dayOfWeekMatches(d time.Time, days []string) bool {
	abbrev := weekdayAbbrev[d.Weekday()]
	for _, want := range days {
		if strings.EqualFold(abbrev, strings.TrimSpace(want)) {
			return true
		}
	}
	return false
}

// timeOfDayBetween reports whether t's wall-clock time falls within
// [start, end] inclusive ("HH:MM" strings). Does not support wrap-around
// windows (e.g. "22:00"-"02:00") — no rule needs that today.
func timeOfDayBetween(t time.Time, start, end string) bool {
	s, errS := time.Parse("15:04", start)
	e, errE := time.Parse("15:04", end)
	if errS != nil || errE != nil {
		return false
	}
	minutes := t.Hour()*60 + t.Minute()
	minS := s.Hour()*60 + s.Minute()
	minE := e.Hour()*60 + e.Minute()
	return minutes >= minS && minutes <= minE
}
