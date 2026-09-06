package wallet

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// GET /v1/api/records
// ---------------------------------------------------------------------------

// ListRecordsParams are the query parameters of GET /v1/api/records.
//
// Note the default window: with no RecordDate bound the server applies an
// implicit three-month filter and reports it back in
// RecordsPage.AppliedRecordDateFilters. Any single bound overrides it, and an
// ID lookup bypasses it entirely.
type ListRecordsParams struct {
	// IDs filters by identifier, up to 30 of them. An ID lookup bypasses the
	// default three-month window.
	IDs []string
	// AccountIDs filters by account, up to 10 of them.
	AccountIDs []string
	// RecordDate filters by transaction date; up to two range conditions.
	RecordDate []Filter
	// Limit is the page size, 1-200. The server defaults to 30.
	Limit *int
	// Offset skips items for pagination.
	Offset *int
	// AgentHints asks for structured hints alongside the data.
	AgentHints *bool
	// WithTotal asks for a total count, at the cost of an extra query.
	WithTotal *bool
	// CategoryIDs filters by category, up to 10 of them. The magic value
	// CategoryUnknown expands to the three unknown categories.
	CategoryIDs []string
	// LabelID returns only records carrying this label.
	LabelID *string
	// Note filters by note and requires a text prefix.
	Note *Filter
	// CounterParty filters by payee or payer and requires a text prefix.
	CounterParty *Filter
	// Amount filters on amount.value; up to two range conditions.
	Amount []Filter
	// CreatedAt filters by creation timestamp; up to two range conditions.
	CreatedAt []Filter
	// UpdatedAt filters by last sync timestamp; up to two range conditions.
	UpdatedAt []Filter
	// RecordType filters on the amount sign.
	RecordType *RecordType
	// IsTransfer selects transfer records (true) or non-transfers (false).
	// It is independent of RecordType.
	IsTransfer *bool
	// TransferIDs returns every record sharing a transfer identity: both legs
	// of a pair, or the single record of an unpaired transfer.
	TransferIDs []string
	// RecordStates filters by clearing state; only the three writable states
	// are accepted here.
	RecordStates []RecordState
	// Sources filters by the client that created the record.
	Sources []RecordSource
	// SortBy orders the page. recordDate, amount, createdAt and updatedAt are
	// sortable; the server defaults to -recordDate.
	SortBy *Sort
	// ConvertTo converts amounts to a currency, or to the magic values
	// ConvertToBase or ConvertToAccount.
	ConvertTo *string
}

// Magic values for the convertTo parameter.
const (
	// ConvertToBase converts to the profile's base currency.
	ConvertToBase = "base"
	// ConvertToAccount converts to each record's own account currency.
	ConvertToAccount = "account"
)

// Validate checks every documented constraint on the parameters.
func (p *ListRecordsParams) Validate() error {
	if p == nil {
		return nil
	}
	var probs problems
	probs.addErr(validateLimit(p.Limit))
	probs.addErr(validateOffset(p.Offset))
	probs.addErr(validateIDList("id", p.IDs, MaxIDFilterValues))
	probs.addErr(validateIDList("accountId", p.AccountIDs, MaxAccountIDFilterValues))
	probs.addErr(validateIDList("categoryId", p.CategoryIDs, MaxCategoryIDFilterValues))
	probs.addErr(validateIDList("transferId", p.TransferIDs, MaxIDFilterValues))
	probs.addErr(validateRangeFilters("recordDate", p.RecordDate))
	probs.addErr(validateRangeFilters("amount", p.Amount))
	probs.addErr(validateRangeFilters("createdAt", p.CreatedAt))
	probs.addErr(validateRangeFilters("updatedAt", p.UpdatedAt))
	if p.Note != nil {
		probs.addErr(validateTextFilter("note", *p.Note))
	}
	if p.CounterParty != nil {
		probs.addErr(validateTextFilter("counterParty", *p.CounterParty))
	}
	if p.RecordType != nil && !p.RecordType.Valid() {
		probs.add("recordType", "%q is not a record type; allowed: %s", *p.RecordType, enumNames(AllRecordTypes))
	}
	for i, s := range p.RecordStates {
		if !s.Writable() {
			probs.add(fmt.Sprintf("recordState[%d]", i),
				"%q cannot be filtered on; allowed: %s", s, enumNames(WritableRecordStates))
		}
	}
	for i, s := range p.Sources {
		if !s.Valid() {
			probs.add(fmt.Sprintf("source[%d]", i), "%q is not a source; allowed: %s", s, enumNames(AllRecordSources))
		}
	}
	probs.addErr(validateSort("sortBy", p.SortBy, RecordSortFields))
	if p.ConvertTo != nil {
		v := *p.ConvertTo
		if v != ConvertToBase && v != ConvertToAccount {
			probs.addErr(validateCurrencyCode("convertTo", v))
		}
	}
	return probs.err()
}

func (p *ListRecordsParams) query() url.Values {
	q := newQuery()
	if p == nil {
		return q.values
	}
	q.setCSV("id", p.IDs)
	q.setCSV("accountId", p.AccountIDs)
	q.setFilters("recordDate", p.RecordDate)
	q.setInt("limit", p.Limit)
	q.setInt("offset", p.Offset)
	q.setBool("agentHints", p.AgentHints)
	q.setBool("withTotal", p.WithTotal)
	q.setCSV("categoryId", p.CategoryIDs)
	q.setString("labelId", p.LabelID)
	q.setFilter("note", p.Note)
	q.setFilter("counterParty", p.CounterParty)
	q.setFilters("amount", p.Amount)
	q.setFilters("createdAt", p.CreatedAt)
	q.setFilters("updatedAt", p.UpdatedAt)
	setEnum(q, "recordType", p.RecordType)
	q.setBool("isTransfer", p.IsTransfer)
	q.setCSV("transferId", p.TransferIDs)
	setEnumCSV(q, "recordState", p.RecordStates)
	setEnumCSV(q, "source", p.Sources)
	q.setSort("sortBy", p.SortBy)
	q.setString("convertTo", p.ConvertTo)
	return q.values
}

// ListRecords returns one page of financial records.
//
// Operation: getRecords. GET /v1/api/records.
func (c *Client) ListRecords(ctx context.Context, params *ListRecordsParams) (*RecordsPage, error) {
	if err := c.validate(params); err != nil {
		return nil, err
	}
	out := &RecordsPage{}
	err := c.do(ctx, request{
		op:     "getRecords",
		method: http.MethodGet,
		path:   "/records",
		query:  params.query(),
	}, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// POST /v1/api/records
// ---------------------------------------------------------------------------

// AmountInput is the amount on a create or patch. CurrencyCode is optional
// and, when given, must match the account's currency.
type AmountInput struct {
	// Value is the decimal amount. Negative is an expense, positive an income.
	// It must not be zero.
	Value float64 `json:"value"`
	// CurrencyCode is optional; it defaults to the account's currency.
	CurrencyCode *string `json:"currencyCode,omitempty"`
}

// Validate checks the amount.
func (a *AmountInput) Validate() error {
	if a == nil {
		return errorf("", "amount is required")
	}
	var p problems
	p.addErr(validateAmountValue("value", a.Value))
	if a.CurrencyCode != nil {
		p.addErr(validateCurrencyCode("currencyCode", *a.CurrencyCode))
	}
	return p.err()
}

// CounterAmountInput is the far-side amount of a cross-currency transfer.
// Both fields are required and the sign must oppose the record's amount.
type CounterAmountInput struct {
	// Value is the amount on the counter account, opposite in sign.
	Value float64 `json:"value"`
	// CurrencyCode must match the counter account's currency.
	CurrencyCode string `json:"currencyCode"`
}

// PlaceInput is the location on a create or patch. On a patch it is a full
// replacement, never a merge.
type PlaceInput struct {
	// Address is the street or full address, at most 255 characters.
	Address *string `json:"address,omitempty"`
	// Latitude is the GPS latitude, -90 to 90.
	Latitude *float64 `json:"latitude,omitempty"`
	// Longitude is the GPS longitude, -180 to 180.
	Longitude *float64 `json:"longitude,omitempty"`
	// Name is the place name, at most 255 characters.
	Name *string `json:"name,omitempty"`
}

// Validate checks the place bounds.
func (p *PlaceInput) Validate() error {
	if p == nil {
		return nil
	}
	var probs problems
	if p.Address != nil {
		probs.addErr(validateMaxLen("address", *p.Address, MaxAddressLength))
	}
	if p.Name != nil {
		probs.addErr(validateMaxLen("name", *p.Name, MaxPlaceNameLength))
	}
	if p.Latitude != nil && (*p.Latitude < -90 || *p.Latitude > 90) {
		probs.add("latitude", "must be between -90 and 90, got %v", *p.Latitude)
	}
	if p.Longitude != nil && (*p.Longitude < -180 || *p.Longitude > 180) {
		probs.add("longitude", "must be between -180 and 180, got %v", *p.Longitude)
	}
	return probs.err()
}

// TransferInput turns a created record into an account movement. Omit it for
// a regular record.
type TransferInput struct {
	// PairingMode is required: PairingNew creates a mirror on AccountID,
	// PairingExisting binds to RecordID, PairingUnpaired records no mirror.
	PairingMode PairingMode `json:"pairingMode"`
	// AccountID is the target account, for PairingNew.
	AccountID *string `json:"accountId,omitempty"`
	// RecordID is the record to bind to, for PairingExisting. The target must
	// be on a different account, have the opposite amount sign, and not
	// already be paired.
	RecordID *string `json:"recordId,omitempty"`
	// CounterAmount is the far-side amount of a cross-currency PairingNew.
	// Omit it to let the server convert via FX.
	CounterAmount *CounterAmountInput `json:"counterAmount,omitempty"`
}

// Validate checks the mode-specific requirements the API documents.
func (t *TransferInput) Validate(recordAmount float64) error {
	if t == nil {
		return nil
	}
	var p problems
	switch {
	case t.PairingMode == "":
		p.add("pairingMode", "is required when transfer is set")
	case !t.PairingMode.Valid():
		p.add("pairingMode", "%q is not a pairing mode; allowed: %s", t.PairingMode, enumNames(AllPairingModes))
	}

	switch t.PairingMode {
	case PairingNew:
		if t.AccountID == nil || strings.TrimSpace(*t.AccountID) == "" {
			p.add("accountId", "is required when pairingMode is %q", PairingNew)
		}
		if t.RecordID != nil {
			p.add("recordId", "is only valid when pairingMode is %q", PairingExisting)
		}
	case PairingExisting:
		if t.RecordID == nil || strings.TrimSpace(*t.RecordID) == "" {
			p.add("recordId", "is required when pairingMode is %q", PairingExisting)
		}
		if t.AccountID != nil {
			p.add("accountId", "is only valid when pairingMode is %q", PairingNew)
		}
		if t.CounterAmount != nil {
			p.add("counterAmount", "is only valid when pairingMode is %q", PairingNew)
		}
	case PairingUnpaired:
		if t.AccountID != nil {
			p.add("accountId", "is only valid when pairingMode is %q", PairingNew)
		}
		if t.RecordID != nil {
			p.add("recordId", "is only valid when pairingMode is %q", PairingExisting)
		}
		if t.CounterAmount != nil {
			p.add("counterAmount", "is only valid when pairingMode is %q", PairingNew)
		}
	}

	if t.CounterAmount != nil {
		p.addErr(validateAmountValue("counterAmount.value", t.CounterAmount.Value))
		p.addErr(validateCurrencyCode("counterAmount.currencyCode", t.CounterAmount.CurrencyCode))
		// The API requires the far side to oppose the near side. Only check
		// when the near side is a usable sign.
		if recordAmount != 0 && t.CounterAmount.Value != 0 &&
			((recordAmount > 0) == (t.CounterAmount.Value > 0)) {
			p.add("counterAmount.value",
				"must have the opposite sign to the record amount (%v), got %v", recordAmount, t.CounterAmount.Value)
		}
	}
	return p.err()
}

// CreateRecordRequest is one item of POST /v1/api/records.
type CreateRecordRequest struct {
	// AccountID is the owning account. Required.
	AccountID string `json:"accountId"`
	// Amount is the transaction amount. Required, and must not be zero.
	Amount AmountInput `json:"amount"`
	// RecordDate is the transaction date. Required, and must not be more than
	// 24 hours in the future.
	//
	// This always goes out as a full RFC 3339 timestamp, so the value is
	// stored exactly as given. That sidesteps the API's date-only convention,
	// where a bare "2026-09-05" on a write is stored at 12:00:00Z while the
	// same string as a filter bound means UTC midnight — send the instant you
	// mean and the two never disagree.
	RecordDate Time `json:"recordDate"`
	// CategoryID is the category to assign.
	CategoryID *string `json:"categoryId,omitempty"`
	// CounterParty is the payee or merchant, at most 255 characters.
	CounterParty *string `json:"counterParty,omitempty"`
	// LabelIDs are the labels to attach.
	LabelIDs []string `json:"labelIds,omitempty"`
	// Note is the free-text note, at most 255 characters.
	Note *string `json:"note,omitempty"`
	// Place is the location.
	Place *PlaceInput `json:"place,omitempty"`
	// RecordState is the clearing state. The server defaults to cleared.
	RecordState *RecordState `json:"recordState,omitempty"`
	// Transfer turns the record into an account movement.
	Transfer *TransferInput `json:"transfer,omitempty"`
}

// Validate checks every documented constraint on one create item. It needs
// the current time to bound RecordDate, which is why it takes one.
func (r *CreateRecordRequest) validateAt(now time.Time) error {
	if r == nil {
		return errorf("", "record is required")
	}
	var p problems
	p.addErr(validateRequiredID("accountId", r.AccountID))
	p.prefixed("amount", r.Amount.Validate())
	if !r.RecordDate.Valid() {
		p.add("recordDate", "is required")
	} else if r.RecordDate.Time.After(now.Add(MaxRecordDateFuture)) {
		p.add("recordDate", "must not be more than 24 hours in the future (got %s, now %s)",
			r.RecordDate, FormatTime(now))
	}
	if r.CounterParty != nil {
		p.addErr(validateMaxLen("counterParty", *r.CounterParty, MaxCounterPartyLength))
	}
	if r.Note != nil {
		p.addErr(validateMaxLen("note", *r.Note, MaxNoteLength))
	}
	p.addErr(validateIDList("labelIds", r.LabelIDs, MaxIDFilterValues))
	p.prefixed("place", r.Place.Validate())
	if r.RecordState != nil && !r.RecordState.Writable() {
		p.add("recordState", "%q cannot be set on a write; allowed: %s",
			*r.RecordState, enumNames(WritableRecordStates))
	}
	p.prefixed("transfer", r.Transfer.Validate(r.Amount.Value))
	return p.err()
}

// Validate checks the item against the current wall clock.
func (r *CreateRecordRequest) Validate() error { return r.validateAt(time.Now()) }

// CreateRecordsParams are the query parameters of POST /v1/api/records.
type CreateRecordsParams struct {
	// ReturnData controls whether each result carries the created record. The
	// server defaults to true. CreatedMirrorRecordID survives false.
	ReturnData *bool
}

func (p *CreateRecordsParams) query() url.Values {
	q := newQuery()
	if p == nil {
		return q.values
	}
	q.setBool("returnData", p.ReturnData)
	return q.values
}

// createRecordsBody validates the batch as a whole.
type createRecordsBody struct {
	records []CreateRecordRequest
	now     time.Time
}

func (b createRecordsBody) Validate() error {
	var p problems
	p.addErr(validateBatchSize("records", len(b.records), MaxCreateRecords))
	for i := range b.records {
		p.prefixed(recordPath(i), b.records[i].validateAt(b.now))
	}
	return p.err()
}

// CreateRecords creates up to 50 transaction records in one call.
//
// Operation: createRecords. POST /v1/api/records. The response envelope is
// returned for 200, 207, 400 and 500 alike; inspect Summary and Results, and
// correlate each result to its input through InputIndex.
func (c *Client) CreateRecords(ctx context.Context, records []CreateRecordRequest, params *CreateRecordsParams) (*CreateRecordsResponse, error) {
	body := createRecordsBody{records: records, now: c.now()}
	if err := c.validate(body); err != nil {
		return nil, err
	}
	out := &CreateRecordsResponse{}
	err := c.do(ctx, request{
		op:     "createRecords",
		method: http.MethodPost,
		path:   "/records",
		query:  params.query(),
		body:   records,
		batch:  true,
	}, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// CreateRecord is the single-record convenience over CreateRecords. It
// returns the one result, and a nil error even when that result failed — the
// caller must check Success, exactly as with the batch form.
func (c *Client) CreateRecord(ctx context.Context, record CreateRecordRequest, params *CreateRecordsParams) (*CreateRecordResult, *CreateRecordsResponse, error) {
	resp, err := c.CreateRecords(ctx, []CreateRecordRequest{record}, params)
	if err != nil {
		return nil, nil, err
	}
	if len(resp.Results) == 0 {
		return nil, resp, &Error{
			Op: "createRecords", Kind: KindDecode, StatusCode: resp.Meta.StatusCode, Meta: resp.Meta,
			Message: "server returned no per-item result for a one-record batch",
		}
	}
	return &resp.Results[0], resp, nil
}

// ---------------------------------------------------------------------------
// PATCH /v1/api/records
// ---------------------------------------------------------------------------

// ClearableRecordField names a field PATCH /records can reset through $clear.
type ClearableRecordField string

// Fields the $clear array accepts.
const (
	ClearCategoryID   ClearableRecordField = "categoryId"
	ClearLabelIDs     ClearableRecordField = "labelIds"
	ClearNote         ClearableRecordField = "note"
	ClearCounterParty ClearableRecordField = "counterParty"
	ClearPlace        ClearableRecordField = "place"
	// ClearTransfer cancels the transfer state and sets the category to
	// Unknown.
	ClearTransfer ClearableRecordField = "transfer"
)

// AllClearableRecordFields lists every field $clear accepts.
var AllClearableRecordFields = []ClearableRecordField{
	ClearCategoryID, ClearLabelIDs, ClearNote, ClearCounterParty, ClearPlace, ClearTransfer,
}

// Valid reports whether the field can be cleared.
func (f ClearableRecordField) Valid() bool { return containsEnum(AllClearableRecordFields, f) }

// StringSet is the labelIds shape on a patch: either a full replacement or an
// {add, remove} delta. Build one with ReplaceIDs, AddRemoveIDs or ClearIDs.
type StringSet struct {
	// replace holds the full replacement list. An empty non-nil slice clears.
	replace []string
	// add and remove hold the delta form.
	add, remove []string
	// isDelta selects which form is encoded.
	isDelta bool
	// set records that this value was constructed, so an unset field can be
	// told from a deliberate clear.
	set bool
}

// ReplaceIDs replaces the whole set with ids. Passing an empty slice clears it.
func ReplaceIDs(ids []string) *StringSet {
	if ids == nil {
		ids = []string{}
	}
	return &StringSet{replace: ids, set: true}
}

// ClearIDs empties the set.
func ClearIDs() *StringSet { return ReplaceIDs([]string{}) }

// AddRemoveIDs applies a delta, adding and removing without touching the rest.
func AddRemoveIDs(add, remove []string) *StringSet {
	return &StringSet{add: add, remove: remove, isDelta: true, set: true}
}

// IsDelta reports whether the value encodes an {add, remove} delta.
func (s *StringSet) IsDelta() bool { return s != nil && s.isDelta }

// Values returns the replacement list, for the replace form.
func (s *StringSet) Values() []string {
	if s == nil || s.isDelta {
		return nil
	}
	return s.replace
}

// Delta returns the add and remove lists, for the delta form.
func (s *StringSet) Delta() (add, remove []string) {
	if s == nil || !s.isDelta {
		return nil, nil
	}
	return s.add, s.remove
}

// MarshalJSON emits whichever of the two documented shapes applies.
func (s *StringSet) MarshalJSON() ([]byte, error) {
	if s == nil || !s.set {
		return []byte("null"), nil
	}
	if !s.isDelta {
		return json.Marshal(s.replace)
	}
	delta := map[string][]string{}
	if s.add != nil {
		delta["add"] = s.add
	}
	if s.remove != nil {
		delta["remove"] = s.remove
	}
	return json.Marshal(delta)
}

// UnmarshalJSON accepts either shape, so a patch body round-trips.
func (s *StringSet) UnmarshalJSON(b []byte) error {
	trimmed := strings.TrimSpace(string(b))
	if trimmed == "null" || trimmed == "" {
		*s = StringSet{}
		return nil
	}
	if strings.HasPrefix(trimmed, "[") {
		var list []string
		if err := json.Unmarshal(b, &list); err != nil {
			return err
		}
		*s = StringSet{replace: list, set: true}
		return nil
	}
	var delta struct {
		Add    []string `json:"add"`
		Remove []string `json:"remove"`
	}
	if err := json.Unmarshal(b, &delta); err != nil {
		return err
	}
	*s = StringSet{add: delta.Add, remove: delta.Remove, isDelta: true, set: true}
	return nil
}

// Validate checks the delta form is not empty, which the API rejects with
// minProperties: 1.
func (s *StringSet) Validate(field string) error {
	if s == nil || !s.set {
		return nil
	}
	var p problems
	if s.isDelta {
		if len(s.add) == 0 && len(s.remove) == 0 {
			p.add(field, "an {add, remove} delta must carry at least one of add or remove")
		}
		p.addErr(validateIDList(field+".add", s.add, MaxIDFilterValues))
		p.addErr(validateIDList(field+".remove", s.remove, MaxIDFilterValues))
	} else {
		p.addErr(validateIDList(field, s.replace, MaxIDFilterValues))
	}
	return p.err()
}

// PatchRecordItem is one item of PATCH /v1/api/records.
type PatchRecordItem struct {
	// ID identifies the record to update. Required.
	ID string `json:"id"`
	// Clear resets fields to empty. A field cannot be both cleared and set.
	// Clearing "transfer" cancels the transfer state and sets the category to
	// Unknown.
	Clear []ClearableRecordField `json:"$clear,omitempty"`
	// AccountID moves the record to another account. Neither account may be
	// bank-synced and both must share a currency.
	AccountID *string `json:"accountId,omitempty"`
	// Amount is the new amount. Setting it ALWAYS rewrites the record's
	// currency to the account's currency; omitting it preserves the original.
	Amount *AmountInput `json:"amount,omitempty"`
	// CategoryID is the new category.
	CategoryID *string `json:"categoryId,omitempty"`
	// CounterParty is the new payee or payer, at most 255 characters.
	CounterParty *string `json:"counterParty,omitempty"`
	// LabelIDs replaces or deltas the label set.
	LabelIDs *StringSet `json:"labelIds,omitempty"`
	// Note is the new note, at most 255 characters.
	Note *string `json:"note,omitempty"`
	// Place fully replaces the location; there is no merge.
	Place *PlaceInput `json:"place,omitempty"`
	// RecordDate is the new transaction date.
	RecordDate *Time `json:"recordDate,omitempty"`
	// RecordState is the new clearing state.
	RecordState *RecordState `json:"recordState,omitempty"`
}

// clearConflicts maps each clearable field to the setter that contradicts it.
func (i *PatchRecordItem) clearConflicts() map[ClearableRecordField]bool {
	return map[ClearableRecordField]bool{
		ClearCategoryID:   i.CategoryID != nil,
		ClearLabelIDs:     i.LabelIDs != nil,
		ClearNote:         i.Note != nil,
		ClearCounterParty: i.CounterParty != nil,
		ClearPlace:        i.Place != nil,
		// There is no transfer setter on a patch, so it can never conflict.
		ClearTransfer: false,
	}
}

// Validate checks every documented constraint on one patch item.
func (i *PatchRecordItem) Validate() error {
	if i == nil {
		return errorf("", "patch item is required")
	}
	var p problems
	p.addErr(validateRequiredID("id", i.ID))

	conflicts := i.clearConflicts()
	seenClear := map[ClearableRecordField]bool{}
	for n, f := range i.Clear {
		at := fmt.Sprintf("$clear[%d]", n)
		if !f.Valid() {
			p.add(at, "%q is not clearable; allowed: %s", f, enumNames(AllClearableRecordFields))
			continue
		}
		if seenClear[f] {
			p.add(at, "%q is listed twice", f)
		}
		seenClear[f] = true
		if conflicts[f] {
			p.add(at, "%q cannot be cleared and set in the same item", f)
		}
	}

	if i.Amount != nil {
		p.prefixed("amount", i.Amount.Validate())
	}
	if i.CounterParty != nil {
		p.addErr(validateMaxLen("counterParty", *i.CounterParty, MaxCounterPartyLength))
	}
	if i.Note != nil {
		p.addErr(validateMaxLen("note", *i.Note, MaxNoteLength))
	}
	p.addErr(i.LabelIDs.Validate("labelIds"))
	p.prefixed("place", i.Place.Validate())
	if i.RecordState != nil && !i.RecordState.Writable() {
		p.add("recordState", "%q cannot be set on a write; allowed: %s",
			*i.RecordState, enumNames(WritableRecordStates))
	}
	if i.RecordDate != nil && !i.RecordDate.Valid() {
		p.add("recordDate", "must be a valid timestamp when set")
	}
	if len(i.Clear) == 0 && i.AccountID == nil && i.Amount == nil && i.CategoryID == nil &&
		i.CounterParty == nil && i.LabelIDs == nil && i.Note == nil && i.Place == nil &&
		i.RecordDate == nil && i.RecordState == nil {
		p.add("", "no fields to update; set at least one field besides id")
	}
	return p.err()
}

// PatchRecordsParams are the query parameters of PATCH /v1/api/records.
type PatchRecordsParams struct {
	// Validation selects strict or lenient handling of invalid items. The
	// server defaults to lenient.
	Validation *ValidationMode
	// ReturnData controls whether each result carries the updated record. The
	// server defaults to true.
	ReturnData *bool
}

// Validate checks the parameters.
func (p *PatchRecordsParams) Validate() error {
	if p == nil {
		return nil
	}
	if p.Validation != nil && !p.Validation.Valid() {
		return errorf("validation", "%q is not a validation mode; allowed: %s",
			*p.Validation, enumNames(AllValidationModes))
	}
	return nil
}

func (p *PatchRecordsParams) query() url.Values {
	q := newQuery()
	if p == nil {
		return q.values
	}
	setEnum(q, "validation", p.Validation)
	q.setBool("returnData", p.ReturnData)
	return q.values
}

type patchRecordsBody []PatchRecordItem

func (b patchRecordsBody) Validate() error {
	var p problems
	p.addErr(validateBatchSize("items", len(b), MaxPatchItems))
	seen := map[string]int{}
	for i := range b {
		p.prefixed(itemPath(i), b[i].Validate())
		if id := b[i].ID; id != "" {
			if first, dup := seen[id]; dup {
				p.add(itemPath(i)+".id", "duplicates items[%d] (%q); the API applies both in order", first, id)
			} else {
				seen[id] = i
			}
		}
	}
	return p.err()
}

// PatchRecords updates up to 10 records in one call.
//
// Operation: patchRecords. PATCH /v1/api/records. The response envelope is
// returned for 200, 207, 400 and 500 alike; inspect Summary and Results.
func (c *Client) PatchRecords(ctx context.Context, items []PatchRecordItem, params *PatchRecordsParams) (*PatchRecordsResponse, error) {
	if err := c.validate(params); err != nil {
		return nil, err
	}
	if err := c.validate(patchRecordsBody(items)); err != nil {
		return nil, err
	}
	out := &PatchRecordsResponse{}
	err := c.do(ctx, request{
		op:     "patchRecords",
		method: http.MethodPatch,
		path:   "/records",
		query:  params.query(),
		body:   items,
		batch:  true,
	}, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}
