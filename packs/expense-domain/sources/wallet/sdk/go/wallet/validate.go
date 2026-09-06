package wallet

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Documented request limits. Every one of these is enforced client-side, so a
// caller learns about the violation without spending a request on it.
const (
	// MinPageLimit is the smallest accepted limit query value.
	MinPageLimit = 1
	// MaxPageLimit is the largest accepted limit query value.
	MaxPageLimit = 200
	// DefaultPageLimit is what the server applies when limit is omitted.
	DefaultPageLimit = 30

	// MaxIDFilterValues is the ceiling on the comma-separated id filter.
	MaxIDFilterValues = 30
	// MaxAccountIDFilterValues is the ceiling on the accountId filter.
	MaxAccountIDFilterValues = 10
	// MaxCategoryIDFilterValues is the ceiling on the categoryId filter.
	MaxCategoryIDFilterValues = 10

	// MaxCreateRecords is the ceiling on POST /records.
	MaxCreateRecords = 50
	// MaxPatchItems is the ceiling on every PATCH endpoint.
	MaxPatchItems = 10
	// MaxDeleteIDs is the ceiling on DELETE /{type}.
	MaxDeleteIDs = 10
	// MaxReferenceIDs is the ceiling on GET /{type}/references.
	MaxReferenceIDs = 20
	// MaxLimitOverrides is the ceiling on a budget's limitOverrides array.
	MaxLimitOverrides = 50

	// MaxNameLength bounds every name field.
	MaxNameLength = 80
	// MaxNoteLength bounds a record's note.
	MaxNoteLength = 255
	// MaxCounterPartyLength bounds a record's counter party.
	MaxCounterPartyLength = 255
	// MaxAddressLength bounds a place's address.
	MaxAddressLength = 255
	// MaxPlaceNameLength bounds a place's name.
	MaxPlaceNameLength = 255
	// MaxBankAccountNumberLength bounds an account number or IBAN.
	MaxBankAccountNumberLength = 80

	// MaxPeriodCount is the ceiling on a limit override's periodCount.
	MaxPeriodCount = 120
	// MinPeriodCount is the floor on a limit override's periodCount.
	MinPeriodCount = 1

	// MaxRecordDateFuture is how far ahead a created record's date may sit.
	MaxRecordDateFuture = 24 * time.Hour
)

// Validatable is implemented by every parameter and request body type, so a
// caller can pre-flight a payload without issuing the call.
type Validatable interface {
	// Validate returns an InvalidRequestErrors describing every problem, or
	// nil when the value is acceptable.
	Validate() error
}

// ---------------------------------------------------------------------------
// Pointer helpers, so optional fields can distinguish unset from zero
// ---------------------------------------------------------------------------

// Int returns a pointer to v.
func Int(v int) *int { return &v }

// Bool returns a pointer to v.
func Bool(v bool) *bool { return &v }

// Float returns a pointer to v.
func Float(v float64) *float64 { return &v }

// String returns a pointer to v.
func String(v string) *string { return &v }

// ---------------------------------------------------------------------------
// Shared field validators
// ---------------------------------------------------------------------------

func validateLimit(v *int) error {
	if v == nil {
		return nil
	}
	if *v < MinPageLimit || *v > MaxPageLimit {
		return errorf("limit", "must be between %d and %d, got %d", MinPageLimit, MaxPageLimit, *v)
	}
	return nil
}

func validateOffset(v *int) error {
	if v == nil {
		return nil
	}
	if *v < 0 {
		return errorf("offset", "must not be negative, got %d", *v)
	}
	return nil
}

// validateIDList checks a comma-separated ID filter: no blanks, no duplicates
// that would waste a slot, and no more values than the endpoint accepts.
func validateIDList(field string, ids []string, max int) error {
	var p problems
	if len(ids) > max {
		p.add(field, "at most %d values allowed, got %d", max, len(ids))
	}
	seen := make(map[string]int, len(ids))
	for i, id := range ids {
		at := fmt.Sprintf("%s[%d]", field, i)
		if strings.TrimSpace(id) == "" {
			p.add(at, "must not be empty")
			continue
		}
		if strings.Contains(id, ",") {
			p.add(at, "must not contain a comma; pass separate values instead")
		}
		if first, dup := seen[id]; dup {
			p.add(at, "duplicates %s[%d] (%q)", field, first, id)
			continue
		}
		seen[id] = i
	}
	return p.err()
}

// validateRequiredID checks a single mandatory identifier.
func validateRequiredID(field, id string) error {
	if strings.TrimSpace(id) == "" {
		return errorf(field, "is required")
	}
	return nil
}

// validateMaxLen bounds a string field, counting runes rather than bytes: the
// API's limits are character limits and a note of 255 accented characters is
// well over 255 bytes.
func validateMaxLen(field, value string, max int) error {
	if n := len([]rune(value)); n > max {
		return errorf(field, "must be at most %d characters, got %d", max, n)
	}
	return nil
}

// validateRequiredText checks a mandatory string with a length bound.
func validateRequiredText(field, value string, max int) error {
	var p problems
	if strings.TrimSpace(value) == "" {
		p.add(field, "is required")
	}
	p.addErr(validateMaxLen(field, value, max))
	return p.err()
}

// validateCurrencyCode checks the shape of an ISO 4217 code. The API resolves
// the code itself, so this only rejects values that cannot possibly be one.
func validateCurrencyCode(field, code string) error {
	if code == "" {
		return errorf(field, "is required")
	}
	if len([]rune(code)) != 3 {
		return errorf(field, "must be a 3-letter ISO 4217 code, got %q", code)
	}
	for _, r := range code {
		if r < 'A' || r > 'Z' {
			if r < 'a' || r > 'z' {
				return errorf(field, "must be a 3-letter ISO 4217 code, got %q", code)
			}
		}
	}
	return nil
}

func validateColor(field string, c *UniversalColor) error {
	if c == nil {
		return nil
	}
	if !c.Valid() {
		return errorf(field, "%q is not a universal colour name; allowed: %s", *c, enumNames(AllUniversalColors))
	}
	return nil
}

func validateCardinality(field string, c *Cardinality) error {
	if c == nil {
		return nil
	}
	if !c.Valid() {
		return errorf(field, "%q is not a cardinality; allowed: %s", *c, enumNames(AllCardinalities))
	}
	return nil
}

// validateAmountValue rejects the one value the API refuses outright.
func validateAmountValue(field string, v float64) error {
	if v == 0 {
		return errorf(field, "must not be zero; negative is an expense, positive an income")
	}
	return nil
}

// validateBatchSize bounds a batch and rejects an empty one, which the API
// answers with a 400 for minItems.
func validateBatchSize(field string, n, max int) error {
	switch {
	case n == 0:
		return errorf(field, "must contain at least 1 item")
	case n > max:
		return errorf(field, "must contain at most %d items, got %d", max, n)
	}
	return nil
}

// validatePeriodFormat checks a budget override period: YYYY, YYYY-MM or
// YYYY-Www.
func validatePeriodFormat(field, period string) error {
	if period == "" {
		return errorf(field, "is required")
	}
	switch {
	case len(period) == 4:
		if _, err := strconv.Atoi(period); err != nil {
			return errorf(field, "yearly period must be YYYY, got %q", period)
		}
		return nil
	case len(period) == 7 && period[4] == '-':
		if strings.HasPrefix(period[5:], "W") {
			return errorf(field, "weekly period must be YYYY-Www with a two-digit week, got %q", period)
		}
		if _, err := time.Parse("2006-01", period); err != nil {
			return errorf(field, "monthly period must be YYYY-MM, got %q", period)
		}
		return nil
	case len(period) == 8 && period[4] == '-' && (period[5] == 'W' || period[5] == 'w'):
		week, err := strconv.Atoi(period[6:])
		if err != nil || week < 1 || week > 53 {
			return errorf(field, "weekly period must be YYYY-Www with week 01-53, got %q", period)
		}
		if _, err := strconv.Atoi(period[:4]); err != nil {
			return errorf(field, "weekly period must be YYYY-Www, got %q", period)
		}
		return nil
	default:
		return errorf(field, "must be YYYY-MM (monthly), YYYY-Www (weekly) or YYYY (yearly), got %q", period)
	}
}

// usagePeriodSpec bounds one form of the api-usage period parameter.
type usagePeriodSpec struct {
	suffix string
	max    int
}

var usagePeriodSpecs = []usagePeriodSpec{
	{"days", 366},
	{"weeks", 53},
	{"months", 13},
}

// validateUsagePeriod checks the Xdays/Xweeks/Xmonths period parameter and
// its per-unit ceiling.
func validateUsagePeriod(field, period string) error {
	if period == "" {
		return errorf(field, "is required")
	}
	for _, spec := range usagePeriodSpecs {
		if !strings.HasSuffix(period, spec.suffix) {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSuffix(period, spec.suffix))
		if err != nil {
			return errorf(field, "must be a number followed by days, weeks or months, got %q", period)
		}
		if n < 1 {
			return errorf(field, "must be at least 1%s, got %q", spec.suffix, period)
		}
		if n > spec.max {
			return errorf(field, "at most %d%s allowed, got %q", spec.max, spec.suffix, period)
		}
		return nil
	}
	return errorf(field, "must be Xdays, Xweeks or Xmonths, got %q", period)
}

// spendingDepthField validates the budgets `spending` parameter, which is
// "none", "current" or "current+N".
func validateSpendingDepth(field, v string) error {
	switch {
	case v == "" || v == "none" || v == "current":
		return nil
	case strings.HasPrefix(v, "current+"):
		n, err := strconv.Atoi(strings.TrimPrefix(v, "current+"))
		if err != nil || n < 1 {
			return errorf(field, "must be none, current or current+N with N >= 1, got %q", v)
		}
		return nil
	default:
		return errorf(field, "must be none, current or current+N, got %q", v)
	}
}

// validateEntityType checks a /{type} path segment against an allow-list, and
// flags the Board-only collection that Wallet answers with a 404.
func validateEntityType(field string, t EntityType, allowed []EntityType) error {
	if t == "" {
		return errorf(field, "is required")
	}
	if !containsEnum(allowed, t) {
		return errorf(field, "%q is not supported here; allowed: %s", t, enumNames(allowed))
	}
	if t == EntityOrders {
		return errorf(field, "%q is Board-only; the Wallet API answers 404 for it", t)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Query serialisation helpers
// ---------------------------------------------------------------------------

// queryBuilder accumulates query parameters, skipping anything unset so an
// omitted option never becomes an empty string on the wire.
type queryBuilder struct {
	values url.Values
}

func newQuery() *queryBuilder { return &queryBuilder{values: url.Values{}} }

func (q *queryBuilder) setInt(key string, v *int) {
	if v != nil {
		q.values.Set(key, strconv.Itoa(*v))
	}
}

func (q *queryBuilder) setBool(key string, v *bool) {
	if v != nil {
		q.values.Set(key, strconv.FormatBool(*v))
	}
}

func (q *queryBuilder) setString(key string, v *string) {
	if v != nil && *v != "" {
		q.values.Set(key, *v)
	}
}

func (q *queryBuilder) setRaw(key, v string) {
	if v != "" {
		q.values.Set(key, v)
	}
}

// setCSV joins a list into the comma-separated form the ID filters take.
func (q *queryBuilder) setCSV(key string, vs []string) {
	if len(vs) > 0 {
		q.values.Set(key, strings.Join(vs, ","))
	}
}

// setEnumCSV joins a list of enum values into comma-separated form.
func setEnumCSV[T ~string](q *queryBuilder, key string, vs []T) {
	if len(vs) == 0 {
		return
	}
	parts := make([]string, len(vs))
	for i, v := range vs {
		parts[i] = string(v)
	}
	q.values.Set(key, strings.Join(parts, ","))
}

// setFilters adds one query entry per condition, which is how the API reads
// the two bounds of a range.
func (q *queryBuilder) setFilters(key string, fs []Filter) {
	for _, f := range fs {
		if f.IsZero() {
			continue
		}
		q.values.Add(key, f.String())
	}
}

// setFilter adds a single text condition.
func (q *queryBuilder) setFilter(key string, f *Filter) {
	if f != nil && !f.IsZero() {
		q.values.Set(key, f.String())
	}
}

func (q *queryBuilder) setSort(key string, s *Sort) {
	if s != nil && s.Field != "" {
		q.values.Set(key, s.String())
	}
}

func setEnum[T ~string](q *queryBuilder, key string, v *T) {
	if v != nil && *v != "" {
		q.values.Set(key, string(*v))
	}
}
