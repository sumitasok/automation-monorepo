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
		status, err := c.do("GET", fmt.Sprintf("/accounts?limit=200&offset=%d", offset), nil, &page)
		if err != nil {
			return nil, err
		}
		if status >= 300 {
			return nil, fmt.Errorf("GET /accounts: HTTP %d", status)
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
	status, err := c.do("POST", "/records", map[string]any{"records": records}, &out)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK && status != http.StatusMultiStatus {
		return out.Results, fmt.Errorf("POST /records: HTTP %d", status)
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
