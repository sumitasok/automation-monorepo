package wallet

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// FilterOp is a query filter prefix. The API rejects an unprefixed value on
// any parameter that documents a prefix, so every filter goes through here.
type FilterOp string

// Filter prefixes. Eq works on both range and text parameters; the ordering
// operators are range-only and the contains operators are text-only.
const (
	// OpEq is an exact match, "eq.".
	OpEq FilterOp = "eq"
	// OpGt is strictly greater than, "gt.".
	OpGt FilterOp = "gt"
	// OpGte is greater than or equal, "gte.".
	OpGte FilterOp = "gte"
	// OpLt is strictly less than, "lt.".
	OpLt FilterOp = "lt"
	// OpLte is less than or equal, "lte.".
	OpLte FilterOp = "lte"
	// OpContains is a case-sensitive substring match, "contains.".
	OpContains FilterOp = "contains"
	// OpContainsI is a case-insensitive substring match, "contains-i.".
	OpContainsI FilterOp = "contains-i"
)

var rangeOps = []FilterOp{OpEq, OpGt, OpGte, OpLt, OpLte}
var textOps = []FilterOp{OpEq, OpContains, OpContainsI}

// IsRange reports whether the operator is valid on a range parameter.
func (o FilterOp) IsRange() bool { return containsEnum(rangeOps, o) }

// IsText reports whether the operator is valid on a text parameter.
func (o FilterOp) IsText() bool { return containsEnum(textOps, o) }

// Filter is one prefixed condition, e.g. gte.2024-01-01.
type Filter struct {
	// Op is the prefix.
	Op FilterOp
	// Value is the unprefixed comparand.
	Value string
}

// String renders the wire form, "<op>.<value>".
func (f Filter) String() string { return string(f.Op) + "." + f.Value }

// IsZero reports whether the filter was never set.
func (f Filter) IsZero() bool { return f.Op == "" && f.Value == "" }

// ---------------------------------------------------------------------------
// Constructors
// ---------------------------------------------------------------------------

// Eq matches exactly. It is valid on both range and text parameters. On a
// date-only value in a range filter the server spans the whole UTC day.
func Eq(v string) Filter { return Filter{Op: OpEq, Value: v} }

// Gt matches values strictly greater than v.
func Gt(v string) Filter { return Filter{Op: OpGt, Value: v} }

// Gte matches values greater than or equal to v.
func Gte(v string) Filter { return Filter{Op: OpGte, Value: v} }

// Lt matches values strictly less than v.
func Lt(v string) Filter { return Filter{Op: OpLt, Value: v} }

// Lte matches values less than or equal to v.
func Lte(v string) Filter { return Filter{Op: OpLte, Value: v} }

// Contains matches a case-sensitive substring.
func Contains(v string) Filter { return Filter{Op: OpContains, Value: v} }

// ContainsI matches a case-insensitive substring.
func ContainsI(v string) Filter { return Filter{Op: OpContainsI, Value: v} }

// ---------------------------------------------------------------------------
// Typed helpers
// ---------------------------------------------------------------------------

// FormatTime renders a timestamp the way the filter parameters expect: RFC
// 3339 in UTC, to the second.
func FormatTime(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05Z") }

// FormatDate renders a calendar date, YYYY-MM-DD. Beware the asymmetry the
// API documents: a date-only recordDate on a WRITE is stored at 12:00:00Z,
// while a date-only bound in a FILTER means UTC midnight. lt.<date> therefore
// excludes records stamped that same day at noon.
func FormatDate(t time.Time) string { return t.UTC().Format("2006-01-02") }

// FormatAmount renders a decimal amount for the amount filter, trimming
// trailing zeroes so 100.50 goes out as 100.5 and 100.00 as 100.
func FormatAmount(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

// TimeFilter builds a range condition over a timestamp.
func TimeFilter(op FilterOp, t time.Time) Filter { return Filter{Op: op, Value: FormatTime(t)} }

// DateFilter builds a range condition over a calendar date.
func DateFilter(op FilterOp, t time.Time) Filter { return Filter{Op: op, Value: FormatDate(t)} }

// AmountFilter builds a range condition over a decimal amount.
func AmountFilter(op FilterOp, v float64) Filter { return Filter{Op: op, Value: FormatAmount(v)} }

// Between builds the two-condition half-open range [from, to) that the range
// parameters accept: gte.from and lt.to.
func Between(from, to time.Time) []Filter {
	return []Filter{TimeFilter(OpGte, from), TimeFilter(OpLt, to)}
}

// BetweenDates builds the half-open date range [from, to): gte.from, lt.to.
func BetweenDates(from, to time.Time) []Filter {
	return []Filter{DateFilter(OpGte, from), DateFilter(OpLt, to)}
}

// OnDate builds the single condition that spans one whole UTC day.
func OnDate(t time.Time) []Filter { return []Filter{DateFilter(OpEq, t)} }

// ParseFilter reads a wire-form filter such as "gte.2024-01-01" back into a
// Filter. It returns an error when the prefix is missing or unknown, which is
// what the server would answer with a 400.
func ParseFilter(s string) (Filter, error) {
	// contains-i must be tried before the generic split, because its own name
	// contains no dot but its value may.
	for _, op := range []FilterOp{OpContainsI, OpContains, OpGte, OpLte, OpGt, OpLt, OpEq} {
		prefix := string(op) + "."
		if strings.HasPrefix(s, prefix) {
			return Filter{Op: op, Value: strings.TrimPrefix(s, prefix)}, nil
		}
	}
	return Filter{}, fmt.Errorf("wallet: %q has no recognised filter prefix (want one of eq., gt., gte., lt., lte., contains., contains-i.)", s)
}

// MustParseFilter is ParseFilter for constants; it panics on a bad prefix.
func MustParseFilter(s string) Filter {
	f, err := ParseFilter(s)
	if err != nil {
		panic(err)
	}
	return f
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

// maxRangeConditions is the ceiling the API documents for range parameters.
const maxRangeConditions = 2

// validateRangeFilters checks a range parameter: at most two conditions, each
// with a range-capable prefix and a non-empty value.
func validateRangeFilters(field string, filters []Filter) error {
	var p problems
	if len(filters) > maxRangeConditions {
		p.add(field, "at most %d conditions allowed, got %d", maxRangeConditions, len(filters))
	}
	for i, f := range filters {
		at := fmt.Sprintf("%s[%d]", field, i)
		switch {
		case f.Op == "":
			p.add(at, "requires a range prefix (eq., gt., gte., lt., lte.)")
		case !f.Op.IsRange():
			p.add(at, "%q is not a range operator; use eq., gt., gte., lt. or lte.", f.Op)
		}
		if strings.TrimSpace(f.Value) == "" {
			p.add(at, "value must not be empty")
		}
	}
	return p.err()
}

// validateTextFilter checks a text parameter: a text-capable prefix and a
// non-empty value.
func validateTextFilter(field string, f Filter) error {
	var p problems
	switch {
	case f.Op == "":
		p.add(field, "requires a text prefix (eq., contains., contains-i.)")
	case !f.Op.IsText():
		p.add(field, "%q is not a text operator; use eq., contains. or contains-i.", f.Op)
	}
	if f.Value == "" {
		p.add(field, "value must not be empty")
	}
	return p.err()
}

// ---------------------------------------------------------------------------
// Sorting
// ---------------------------------------------------------------------------

// Sort is a sortBy value: a field plus a direction rendered as a +/- prefix.
type Sort struct {
	// Field is the field to sort on.
	Field string
	// Descending selects the "-" prefix; otherwise "+".
	Descending bool
}

// String renders the wire form, e.g. "-recordDate".
func (s Sort) String() string {
	if s.Descending {
		return "-" + s.Field
	}
	return "+" + s.Field
}

// Asc sorts ascending on field.
func Asc(field string) Sort { return Sort{Field: field} }

// Desc sorts descending on field.
func Desc(field string) Sort { return Sort{Field: field, Descending: true} }

// Sortable fields. Most list endpoints allow only the two timestamps;
// records additionally allows recordDate and amount.
var (
	// CommonSortFields are the fields every sortable list endpoint accepts.
	CommonSortFields = []string{"createdAt", "updatedAt"}
	// RecordSortFields are the fields GET /records accepts.
	RecordSortFields = []string{"recordDate", "amount", "createdAt", "updatedAt"}
)

// validateSort checks a sort against the fields an endpoint allows.
func validateSort(field string, s *Sort, allowed []string) error {
	if s == nil {
		return nil
	}
	if s.Field == "" {
		return errorf(field, "sort field must not be empty")
	}
	for _, a := range allowed {
		if a == s.Field {
			return nil
		}
	}
	return errorf(field, "%q is not sortable here; allowed: %s", s.Field, strings.Join(allowed, ", "))
}
