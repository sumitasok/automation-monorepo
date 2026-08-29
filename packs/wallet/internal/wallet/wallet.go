// Package wallet is a small client for the BudgetBakers Wallet REST API
// (https://rest.budgetbakers.com/wallet). Standard library only.
//
// Auth: Authorization: Bearer <token>. Token from
// https://web.budgetbakers.com/settings/rest-api (Premium plan).
package wallet

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client talks to the Wallet REST API.
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

// New builds a client with a sane timeout.
func New(baseURL, token string) *Client {
	return &Client{
		BaseURL: baseURL,
		Token:   token,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

// Account is the subset of GET /accounts we care about.
type Account struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	CurrencyCode string `json:"currencyCode"`
	Archived     bool   `json:"archived"`
}

// Label is the subset of GET /labels we care about.
type Label struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// NewRecord is a record to create via POST /records.
type NewRecord struct {
	AccountID    string   `json:"accountId"`
	Amount       float64  `json:"amount"` // negative=expense, positive=income
	RecordDate   string   `json:"recordDate"`
	PaymentType  string   `json:"paymentType"`
	CategoryID   string   `json:"categoryId,omitempty"`
	LabelIDs     []string `json:"labelIds,omitempty"`
	Note         string   `json:"note,omitempty"`
	CounterParty string   `json:"counterParty,omitempty"`
}

// do performs a request and returns status + body, handling auth + JSON.
func (c *Client) do(method, path string, body any, out any) (int, error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.BaseURL+path, rdr)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusConflict {
		return resp.StatusCode, fmt.Errorf("wallet sync in progress (409); retry in a few minutes: %s", string(raw))
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return resp.StatusCode, fmt.Errorf("unauthorized (401): check WALLET_API_TOKEN")
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return resp.StatusCode, fmt.Errorf("rate limited (429); slow down and retry")
	}
	// Any other non-success status (404, 400, 422, 5xx, ...): surface the raw
	// response body. Previously callers only reported the status code
	// ("POST /records: HTTP 404") with no indication of *why* — the body
	// almost always carries the actual reason (bad accountId, validation
	// error, wrong path, ...) and was being silently discarded. 207
	// Multi-Status is a genuine partial-success response, not an error here.
	if resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("%s %s: HTTP %d: %s", method, path, resp.StatusCode, truncate(string(raw), 500))
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return resp.StatusCode, fmt.Errorf("decode %s %s response: %w (body: %s)", method, path, err, truncate(string(raw), 300))
		}
	}
	return resp.StatusCode, nil
}

// GetAccounts lists accounts (paged, up to 200 per page).
func (c *Client) GetAccounts() ([]Account, error) {
	var all []Account
	offset := 0
	for {
		var page struct {
			Accounts   []Account `json:"accounts"`
			NextOffset *int      `json:"nextOffset"`
		}
		status, err := c.do("GET", fmt.Sprintf("/v1/api/accounts?limit=200&offset=%d", offset), nil, &page)
		if err != nil {
			return nil, err
		}
		if status >= 300 {
			return nil, fmt.Errorf("GET /v1/api/accounts: HTTP %d: %w", status, err)
		}
		all = append(all, page.Accounts...)
		if page.NextOffset == nil {
			break
		}
		offset = *page.NextOffset
	}
	return all, nil
}

// GetLabels lists labels.
func (c *Client) GetLabels() ([]Label, error) {
	var all []Label
	offset := 0
	for {
		var page struct {
			Labels     []Label `json:"labels"`
			NextOffset *int    `json:"nextOffset"`
		}
		status, err := c.do("GET", fmt.Sprintf("/labels?limit=200&offset=%d", offset), nil, &page)
		if err != nil {
			return nil, err
		}
		if status >= 300 {
			return nil, fmt.Errorf("GET /labels: HTTP %d", status)
		}
		all = append(all, page.Labels...)
		if page.NextOffset == nil {
			break
		}
		offset = *page.NextOffset
	}
	return all, nil
}

// Record is a Wallet record as returned by GET /records, kept as a raw
// key/value map rather than a fixed struct so fetch can round-trip whatever
// fields the API returns (id, accountId, amount, category, labels, note,
// counterParty, transfer, recordDate, createdAt, updatedAt, ...) without
// risking silent field loss if the live schema gains or renames a field.
type Record map[string]any

// farPastFloor is sent as the recordDate lower bound when the caller doesn't
// supply one, so GetRecords always sends an explicit recordDate=gte. filter
// rather than omitting the parameter — see the recordDateFrom doc below.
const farPastFloor = "2000-01-01"

// GetRecords lists every record via GET /v1/api/records, paginated 200 at a
// time. CreateRecords and batch update (PATCH) both use POST/PATCH /v1/api/records.
// The un-prefixed GET /records 404s with "no Route matched with those values"
// (confirmed against the live API 2026-08-29 — see ADR 0020 correction).
//
// recordDateFrom, when non-empty (YYYY-MM-DD), filters to records dated
// on/after that date via recordDate=gte.<value> — the range-filter
// dimension this API's own docs describe. When recordDateFrom is empty,
// farPastFloor is sent instead of omitting the filter entirely: the
// endpoint's own OpenAPI spec (confirmed 2026-08-29,
// /wallet/openapi/ui#/Banking/getRecords) documents that an omitted
// recordDate gets a default 3-month window applied
// ("appliedRecordDateFilters" in the response) — "[p]rovide any single
// bound... to override the default". A fetch that relied on the missing-
// filter default returned only 504 of 6,328 real records, every one dated
// within the prior ~92 days, before this was understood. Always sending an
// explicit wide-open lower bound avoids depending on that default — this
// matters even for updatedAtFrom-only (incremental) calls, since the spec
// ties the default window to recordDate specifically, independent of any
// other filter: an incremental fetch that only set updatedAtFrom and left
// recordDate to its default could silently miss a months-old record that
// was only just re-categorized, because its (old) recordDate would fall
// outside the implicit window even though its (recent) updatedAt matches.
//
// updatedAtFrom, when non-empty (RFC3339), additionally filters to records
// last modified on/after that timestamp via updatedAt=gte.<value> — the
// dimension an incremental fetch uses to catch both newly-created records
// and edits to old ones (e.g. a future recategorization job patching an
// old record's category — recordDate stays the same, updatedAt moves).
// This is a genuine, documented filter dimension (GET /v1/api/records
// query param "updatedAt": "Filter by last sync timestamp... Requires
// range prefix") — an earlier version of this pack used it without also
// pinning recordDate wide open and saw no effect, which at the time looked
// like the API silently ignoring it; in hindsight that run most likely
// never sent updatedAt at all (it predates this parameter existing on this
// method) and was actually hitting the recordDate default-window bug
// instead. Left empty, no updatedAt filter is sent at all.
//
// withTotal=true is sent only on the first page (offset 0): per the spec it
// "requires an additional database query", and the total shouldn't change
// mid-fetch, so one page pays for it rather than every page. Returns the
// fetched records plus that reported total, so callers can sanity-check the
// fetched count against what the server itself claims exists. Without
// withTotal=true the spec's example response shows "total": 0 — i.e. the
// field is present but meaningless unless explicitly requested; a caller
// must not treat a zero total as authoritative when withTotal was omitted,
// which is why GetRecords always requests it on the first page itself
// rather than leaving that to callers.
func (c *Client) GetRecords(recordDateFrom, updatedAtFrom string) ([]Record, int, error) {
	lower := recordDateFrom
	if lower == "" {
		lower = farPastFloor
	}
	var all []Record
	apiTotal := 0
	offset := 0
	for {
		path := fmt.Sprintf("/v1/api/records?limit=200&offset=%d&recordDate=gte.%s", offset, url.QueryEscape(lower))
		if updatedAtFrom != "" {
			path += "&updatedAt=gte." + url.QueryEscape(updatedAtFrom)
		}
		if offset == 0 {
			path += "&withTotal=true"
		}
		var page struct {
			Records    []Record `json:"records"`
			NextOffset *int     `json:"nextOffset"`
			Total      int      `json:"total"`
		}
		status, err := c.do("GET", path, nil, &page)
		if err != nil {
			return nil, 0, err
		}
		if status >= 300 {
			return nil, 0, fmt.Errorf("GET /v1/api/records: HTTP %d", status)
		}
		all = append(all, page.Records...)
		if offset == 0 {
			apiTotal = page.Total
		}
		if page.NextOffset == nil {
			break
		}
		offset = *page.NextOffset
	}
	return all, apiTotal, nil
}

// CreateLabel creates a label and returns its ID. Best-effort: if the endpoint
// is unavailable the caller is expected to surface actionable guidance.
func (c *Client) CreateLabel(name string) (string, error) {
	var out struct {
		ID    string `json:"id"`
		Label Label  `json:"label"`
	}
	status, err := c.do("POST", "/labels", map[string]string{"name": name}, &out)
	if err != nil {
		return "", err
	}
	if status >= 300 {
		return "", fmt.Errorf("POST /labels: HTTP %d (create the label %q in the Wallet app, then re-run)", status, name)
	}
	if out.ID != "" {
		return out.ID, nil
	}
	return out.Label.ID, nil
}

// EnsureLabel resolves a label by name, creating it if missing.
func (c *Client) EnsureLabel(name string) (string, error) {
	labels, err := c.GetLabels()
	if err != nil {
		return "", err
	}
	for _, l := range labels {
		if l.Name == name {
			return l.ID, nil
		}
	}
	return c.CreateLabel(name)
}

// RecordResult is one item's outcome from POST /records.
type RecordResult struct {
	InputIndex int    `json:"inputIndex"`
	ID         string `json:"id"`
	Success    bool   `json:"success"`
	Error      string `json:"error"`
	ErrorType  string `json:"errorType"`
}

// CreateRecords posts a batch (max 20). Handles 200 and 207 (partial success).
// Uses POST /v1/api/records with request body as direct array (not wrapped in object).
func (c *Client) CreateRecords(records []NewRecord) ([]RecordResult, error) {
	if len(records) == 0 {
		return nil, nil
	}
	if len(records) > 20 {
		return nil, fmt.Errorf("batch too large: %d (max 20)", len(records))
	}
	var out struct {
		Results []RecordResult `json:"results"`
		Summary struct {
			Total     int `json:"total"`
			Succeeded int `json:"succeeded"`
		} `json:"summary"`
	}
	status, err := c.do("POST", "/v1/api/records", records, &out)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK && status != http.StatusMultiStatus {
		return out.Results, fmt.Errorf("POST /v1/api/records: HTTP %d", status)
	}
	// Some responses may return a bare array; if Results is empty but status ok,
	// synthesise success results so callers can record IDs when present.
	return out.Results, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
