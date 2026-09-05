package event

import (
	"os"
	"path/filepath"
	"testing"
)

func writeRules(t *testing.T, yamlBody string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "expense-rules.yaml")
	if err := os.WriteFile(p, []byte(yamlBody), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadExpenseRules_MissingFileDegradesGracefully(t *testing.T) {
	rs, err := LoadExpenseRules(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if err != nil {
		t.Fatalf("missing file should not error, got %v", err)
	}
	if len(rs.Rules) != 0 {
		t.Fatalf("missing file should yield an empty rule set, got %d rules", len(rs.Rules))
	}
}

func TestLoadExpenseRules_RejectsEmptyMatch(t *testing.T) {
	p := writeRules(t, `
rules:
  - name: bad-rule
    applies_to: [event]
    match: {}
    outcome:
      event_relevance: routine
`)
	if _, err := LoadExpenseRules(p); err == nil {
		t.Fatal("expected an error for a rule with no match conditions")
	}
}

func TestLoadExpenseRules_RejectsEmptyOutcome(t *testing.T) {
	p := writeRules(t, `
rules:
  - name: bad-rule
    applies_to: [event]
    match:
      merchant_contains: ["x"]
    outcome: {}
`)
	if _, err := LoadExpenseRules(p); err == nil {
		t.Fatal("expected an error for a rule with no outcome fields")
	}
}

func TestLoadExpenseRules_RejectsDuplicateName(t *testing.T) {
	p := writeRules(t, `
rules:
  - name: dup
    applies_to: [event]
    match: {merchant_contains: ["a"]}
    outcome: {event_relevance: routine}
  - name: dup
    applies_to: [event]
    match: {merchant_contains: ["b"]}
    outcome: {event_relevance: routine}
`)
	if _, err := LoadExpenseRules(p); err == nil {
		t.Fatal("expected an error for duplicate rule names")
	}
}

func TestLoadExpenseRules_RejectsBadScope(t *testing.T) {
	p := writeRules(t, `
rules:
  - name: bad-scope
    applies_to: [somethingelse]
    match: {merchant_contains: ["a"]}
    outcome: {event_relevance: routine}
`)
	if _, err := LoadExpenseRules(p); err == nil {
		t.Fatal("expected an error for an invalid applies_to value")
	}
}

const rulesFixture = `
rules:
  - name: hungerbox-workplace-food
    applies_to: [categorize, event]
    match:
      merchant_contains: ["hungerbox"]
    outcome:
      category: "Food & Drinks"
      subcategory: "Groceries"
      labels: ["Work"]
      event_relevance: routine

  - name: disabled-rule
    enabled: false
    applies_to: [event]
    match:
      merchant_contains: ["disabledmerchant"]
    outcome:
      event_relevance: routine

  - name: categorize-only-rule
    applies_to: [categorize]
    match:
      merchant_contains: ["categorizeonly"]
    outcome:
      category: "Food & Drinks"
      subcategory: "Groceries"

  - name: uber-weekday-afternoon-commute
    applies_to: [categorize, event]
    match:
      merchant_contains: ["uber"]
      day_of_week: [mon, tue, wed, thu, fri]
      time_between: ["13:00", "20:00"]
    outcome:
      category: "Transportation"
      subcategory: "Business trips"
      labels: ["Work"]
      event_relevance: routine

  - name: big-purchase
    applies_to: [event]
    match:
      amount_between: [10000, 50000]
    outcome:
      event_relevance: routine
`

func loadFixtureRules(t *testing.T) ExpenseRules {
	t.Helper()
	rs, err := LoadExpenseRules(writeRules(t, rulesFixture))
	if err != nil {
		t.Fatal(err)
	}
	return rs
}

func TestExpenseRules_MerchantContainsMatch(t *testing.T) {
	rs := loadFixtureRules(t)
	rule, ok := rs.Match(ScopeEvent, MatchInput{Merchant: "HungerBox Bangalore"})
	if !ok || rule.Name != "hungerbox-workplace-food" {
		t.Fatalf("expected hungerbox rule to match, got %+v ok=%v", rule, ok)
	}
	if rule.Outcome.EventRelevance != EventRelevanceRoutine {
		t.Fatalf("expected event_relevance %q, got %q", EventRelevanceRoutine, rule.Outcome.EventRelevance)
	}
}

func TestExpenseRules_DisabledRuleSkipped(t *testing.T) {
	rs := loadFixtureRules(t)
	if _, ok := rs.Match(ScopeEvent, MatchInput{Merchant: "DisabledMerchant Ltd"}); ok {
		t.Fatal("a disabled rule must never match")
	}
}

func TestExpenseRules_ScopingExcludesWrongDecisionType(t *testing.T) {
	rs := loadFixtureRules(t)
	// categorize-only-rule is scoped to "categorize" only; must not match "event".
	if _, ok := rs.Match(ScopeEvent, MatchInput{Merchant: "CategorizeOnly Corp"}); ok {
		t.Fatal("categorize-scoped rule must not match an event-scope lookup")
	}
	rule, ok := rs.Match(ScopeCategorize, MatchInput{Merchant: "CategorizeOnly Corp"})
	if !ok || rule.Name != "categorize-only-rule" {
		t.Fatalf("expected categorize-only-rule to match under ScopeCategorize, got %+v ok=%v", rule, ok)
	}
}

func TestExpenseRules_KeywordContainsMatch(t *testing.T) {
	rs, err := LoadExpenseRules(writeRules(t, `
rules:
  - name: refund-keyword
    applies_to: [event]
    match:
      keyword_contains: ["refund", "reversal"]
    outcome:
      event_relevance: routine
`))
	if err != nil {
		t.Fatal(err)
	}
	rule, ok := rs.Match(ScopeEvent, MatchInput{Info: "Refund processed for order 123"})
	if !ok || rule.Name != "refund-keyword" {
		t.Fatalf("expected keyword match, got %+v ok=%v", rule, ok)
	}
	if _, ok := rs.Match(ScopeEvent, MatchInput{Info: "unrelated"}); ok {
		t.Fatal("must not match when no keyword present")
	}
}

func TestExpenseRules_DayOfWeekAndTimeBetween(t *testing.T) {
	rs := loadFixtureRules(t)

	// Weekday afternoon: 2026-07-20 is a Monday.
	rule, ok := rs.Match(ScopeEvent, MatchInput{Merchant: "Uber Trip", TxnDate: "2026-07-20 14:30:00"})
	if !ok || rule.Name != "uber-weekday-afternoon-commute" {
		t.Fatalf("expected weekday-afternoon Uber rule to match, got %+v ok=%v", rule, ok)
	}

	// Same time, but a Saturday (2026-07-25) — day_of_week excludes it.
	if _, ok := rs.Match(ScopeEvent, MatchInput{Merchant: "Uber Trip", TxnDate: "2026-07-25 14:30:00"}); ok {
		t.Fatal("weekend Uber ride must not match a weekday-only rule")
	}

	// Weekday morning, outside the time window.
	if _, ok := rs.Match(ScopeEvent, MatchInput{Merchant: "Uber Trip", TxnDate: "2026-07-20 09:00:00"}); ok {
		t.Fatal("morning Uber ride must not match an afternoon-only time window")
	}
}

func TestExpenseRules_TimeBetweenFailsClosedWithoutTimeComponent(t *testing.T) {
	rs := loadFixtureRules(t)
	if _, ok := rs.Match(ScopeEvent, MatchInput{Merchant: "Uber Trip", TxnDate: "2026-07-20"}); ok {
		t.Fatal("time_between must fail closed when TxnDate has no time component")
	}
	if _, ok := rs.Match(ScopeEvent, MatchInput{Merchant: "Uber Trip", TxnDate: "not-a-date"}); ok {
		t.Fatal("time_between must fail closed on an unparseable TxnDate")
	}
}

func TestExpenseRules_AmountBetween(t *testing.T) {
	rs := loadFixtureRules(t)
	rule, ok := rs.Match(ScopeEvent, MatchInput{Amount: "25000"})
	if !ok || rule.Name != "big-purchase" {
		t.Fatalf("expected big-purchase rule to match amount in range, got %+v ok=%v", rule, ok)
	}
	if _, ok := rs.Match(ScopeEvent, MatchInput{Amount: "500"}); ok {
		t.Fatal("amount below range must not match")
	}
	if _, ok := rs.Match(ScopeEvent, MatchInput{Amount: "not-a-number"}); ok {
		t.Fatal("unparseable amount must not match, not error out the run")
	}
}

func TestExpenseRules_FirstMatchWins(t *testing.T) {
	rs, err := LoadExpenseRules(writeRules(t, `
rules:
  - name: first
    applies_to: [event]
    match: {merchant_contains: ["shop"]}
    outcome: {event_relevance: routine}
  - name: second
    applies_to: [event]
    match: {merchant_contains: ["shop"]}
    outcome: {event_relevance: routine}
`))
	if err != nil {
		t.Fatal(err)
	}
	rule, ok := rs.Match(ScopeEvent, MatchInput{Merchant: "Big Shop"})
	if !ok || rule.Name != "first" {
		t.Fatalf("expected the first declared matching rule to win, got %+v ok=%v", rule, ok)
	}
}

func TestExpenseRules_NoMatchReturnsFalse(t *testing.T) {
	rs := loadFixtureRules(t)
	if _, ok := rs.Match(ScopeEvent, MatchInput{Merchant: "SomeRandomShop"}); ok {
		t.Fatal("expected no match for an unrelated merchant")
	}
}
