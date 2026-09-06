package wallet

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestNewValidatesConfig(t *testing.T) {
	tests := []struct {
		name  string
		cfg   Config
		field string
	}{
		{"missing token", Config{}, "Token"},
		{"blank token", Config{Token: "   "}, "Token"},
		{"bad scheme", Config{Token: "t", BaseURL: "ftp://example.test"}, "BaseURL"},
		{"no host", Config{Token: "t", BaseURL: "https://"}, "BaseURL"},
		{"unparseable", Config{Token: "t", BaseURL: "://nope"}, "BaseURL"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New(tc.cfg)
			errs := mustInvalidRequest(t, err)
			wantField(t, errs, tc.field)
			if !errors.Is(err, ErrInvalidRequest) {
				t.Errorf("errors.Is(err, ErrInvalidRequest) = false")
			}
		})
	}
}

func TestNewDefaults(t *testing.T) {
	c, err := New(Config{Token: "t"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := c.BaseURL(); got != DefaultBaseURL {
		t.Errorf("BaseURL = %q, want %q", got, DefaultBaseURL)
	}
	if c.httpClient.Timeout != DefaultTimeout {
		t.Errorf("timeout = %v, want %v", c.httpClient.Timeout, DefaultTimeout)
	}
	if c.userAgent != DefaultUserAgent {
		t.Errorf("user agent = %q, want %q", c.userAgent, DefaultUserAgent)
	}
}

func TestBaseURLWithPathPrefixIsPreserved(t *testing.T) {
	m := newMockServer(t, mockResponse{JSONBody: map[string]any{"accounts": []any{}}})
	c := newTestClient(t, m, func(cfg *Config) {
		cfg.BaseURL = m.URL() + "/proxy/wallet/"
	})
	if _, err := c.ListAccounts(testCtx(t), nil); err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if got, want := m.Last(t).Path, "/proxy/wallet/v1/api/accounts"; got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
}

func TestRequestCarriesAuthAndDefaultHeaders(t *testing.T) {
	m := newMockServer(t, mockResponse{JSONBody: map[string]any{"accounts": []any{}}})
	c := newTestClient(t, m, func(cfg *Config) {
		cfg.UserAgent = "my-agent/2.0"
		cfg.Header = http.Header{"X-Trace-Id": []string{"abc123"}}
	})
	if _, err := c.ListAccounts(testCtx(t), nil); err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	req := m.Last(t)
	if got, want := req.Header.Get("Authorization"), "Bearer test-token"; got != want {
		t.Errorf("Authorization = %q, want %q", got, want)
	}
	if got, want := req.Header.Get("Accept"), "application/json"; got != want {
		t.Errorf("Accept = %q, want %q", got, want)
	}
	if got, want := req.Header.Get("User-Agent"), "my-agent/2.0"; got != want {
		t.Errorf("User-Agent = %q, want %q", got, want)
	}
	if got, want := req.Header.Get("X-Trace-Id"), "abc123"; got != want {
		t.Errorf("X-Trace-Id = %q, want %q", got, want)
	}
	// A GET has no body, so it must not claim a content type.
	if got := req.Header.Get("Content-Type"); got != "" {
		t.Errorf("Content-Type on GET = %q, want empty", got)
	}
}

func TestCustomHeaderCannotOverrideAuthorization(t *testing.T) {
	m := newMockServer(t, mockResponse{JSONBody: map[string]any{"accounts": []any{}}})
	c := newTestClient(t, m, func(cfg *Config) {
		cfg.Header = http.Header{"Authorization": []string{"Bearer attacker"}}
	})
	if _, err := c.ListAccounts(testCtx(t), nil); err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if got, want := m.Last(t).Header.Get("Authorization"), "Bearer test-token"; got != want {
		t.Errorf("Authorization = %q, want the client's own token %q", got, want)
	}
}

func TestPostSetsJSONContentType(t *testing.T) {
	m := newMockServer(t, mockResponse{Status: 201, JSONBody: map[string]any{"label": map[string]any{"id": "l1"}}})
	c := newTestClient(t, m)
	if _, err := c.CreateLabel(testCtx(t), &CreateLabelRequest{Name: "vacation"}); err != nil {
		t.Fatalf("CreateLabel: %v", err)
	}
	req := m.Last(t)
	if got, want := req.Header.Get("Content-Type"), "application/json"; got != want {
		t.Errorf("Content-Type = %q, want %q", got, want)
	}
	if !strings.Contains(string(req.Body), `"vacation"`) {
		t.Errorf("body does not carry the name: %s", req.Body)
	}
}

func TestResponseMetaParsesEveryDocumentedHeader(t *testing.T) {
	m := newMockServer(t, mockResponse{
		JSONBody: map[string]any{"accounts": []any{}},
		Header: http.Header{
			"X-Ratelimit-Limit":      []string{"500"},
			"X-Ratelimit-Remaining":  []string{"487"},
			"X-Last-Data-Change-At":  []string{"2024-01-28T14:23:45.123Z"},
			"X-Last-Data-Change-Rev": []string{"r1234"},
			"X-Sync-In-Progress":     []string{"true"},
		},
	})
	c := newTestClient(t, m)
	page, err := c.ListAccounts(testCtx(t), nil)
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	meta := page.Meta
	if meta.RateLimitLimit == nil || *meta.RateLimitLimit != 500 {
		t.Errorf("RateLimitLimit = %v, want 500", meta.RateLimitLimit)
	}
	if meta.RateLimitRemaining == nil || *meta.RateLimitRemaining != 487 {
		t.Errorf("RateLimitRemaining = %v, want 487", meta.RateLimitRemaining)
	}
	if meta.LastDataChangeRev != "r1234" {
		t.Errorf("LastDataChangeRev = %q, want r1234", meta.LastDataChangeRev)
	}
	if meta.SyncInProgress == nil || !*meta.SyncInProgress {
		t.Errorf("SyncInProgress = %v, want true", meta.SyncInProgress)
	}
	want := time.Date(2024, 1, 28, 14, 23, 45, 123_000_000, time.UTC)
	if meta.LastDataChangeAt == nil || !meta.LastDataChangeAt.Equal(want) {
		t.Errorf("LastDataChangeAt = %v, want %v", meta.LastDataChangeAt, want)
	}
	if meta.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", meta.StatusCode)
	}
	if meta.Attempts != 1 {
		t.Errorf("Attempts = %d, want 1", meta.Attempts)
	}
	// LastMeta mirrors the most recent response.
	if c.LastMeta().LastDataChangeRev != "r1234" {
		t.Errorf("LastMeta not recorded: %+v", c.LastMeta())
	}
}

func TestResponseMetaToleratesMalformedHeaders(t *testing.T) {
	m := newMockServer(t, mockResponse{
		JSONBody: map[string]any{"accounts": []any{}},
		Header: http.Header{
			"X-Ratelimit-Limit":     []string{"not-a-number"},
			"X-Last-Data-Change-At": []string{"yesterday"},
			"X-Sync-In-Progress":    []string{"FALSE"},
		},
	})
	c := newTestClient(t, m)
	page, err := c.ListAccounts(testCtx(t), nil)
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if page.Meta.RateLimitLimit != nil {
		t.Errorf("RateLimitLimit = %v, want nil for an unparseable header", page.Meta.RateLimitLimit)
	}
	if page.Meta.LastDataChangeAt != nil {
		t.Errorf("LastDataChangeAt = %v, want nil for an unparseable header", page.Meta.LastDataChangeAt)
	}
	if page.Meta.SyncInProgress == nil || *page.Meta.SyncInProgress {
		t.Errorf("SyncInProgress = %v, want false (case-insensitive)", page.Meta.SyncInProgress)
	}
}

func TestRateLimitExhausted(t *testing.T) {
	zero, some := 0, 3
	if !(ResponseMeta{RateLimitRemaining: &zero}).RateLimitExhausted() {
		t.Error("remaining 0 should report exhausted")
	}
	if (ResponseMeta{RateLimitRemaining: &some}).RateLimitExhausted() {
		t.Error("remaining 3 should not report exhausted")
	}
	if (ResponseMeta{}).RateLimitExhausted() {
		t.Error("absent header should not report exhausted")
	}
}

func TestErrorStatusMapping(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		body        string
		contentType string
		kind        ErrorKind
		sentinel    error
		wantMessage string
	}{
		{"400", 400, `{"error":"invalid parameter"}`, "application/json", KindValidation, ErrValidation, "invalid parameter"},
		{"401", 401, `{"error":"Invalid or expired token"}`, "application/json", KindUnauthorized, ErrUnauthorized, "Invalid or expired token"},
		{"403 text/plain", 403, "Forbidden — the token belongs to the other app.", "text/plain", KindForbidden, ErrForbidden, "Forbidden — the token belongs to the other app."},
		{"404", 404, `{"error":"not found"}`, "application/json", KindNotFound, ErrNotFound, "not found"},
		{"429", 429, `{"error":"Rate limit exceeded. Please try again later."}`, "application/json", KindRateLimited, ErrRateLimited, "Rate limit exceeded. Please try again later."},
		{"500", 500, `{"error":"Internal Error"}`, "application/json", KindServer, ErrServer, "Internal Error"},
		{"503", 503, `{"error":"unavailable"}`, "application/json", KindServer, ErrServer, "unavailable"},
		{"418 undocumented", 418, `{"error":"teapot"}`, "application/json", KindUnexpectedStatus, ErrUnexpectedStatus, "teapot"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := newMockServer(t, mockResponse{Status: tc.status, Body: tc.body, ContentType: tc.contentType})
			c := newTestClient(t, m)
			_, err := c.ListAccounts(testCtx(t), nil)
			apiErr := mustAPIError(t, err, tc.kind)
			if apiErr.StatusCode != tc.status {
				t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, tc.status)
			}
			if apiErr.Message != tc.wantMessage {
				t.Errorf("Message = %q, want %q", apiErr.Message, tc.wantMessage)
			}
			if apiErr.RawBody != tc.body {
				t.Errorf("RawBody = %q, want %q", apiErr.RawBody, tc.body)
			}
			if apiErr.Op != "getAccounts" {
				t.Errorf("Op = %q, want getAccounts", apiErr.Op)
			}
			if !errors.Is(err, tc.sentinel) {
				t.Errorf("errors.Is(err, %v) = false", tc.sentinel)
			}
			if !strings.Contains(apiErr.Error(), "getAccounts") {
				t.Errorf("Error() should name the operation: %s", apiErr.Error())
			}
		})
	}
}

func TestErrorMessageFallsBackWhenBodyIsEmptyOrOpaque(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		body        string
		contentType string
		wantPrefix  string
	}{
		{"empty 401", 401, "", "application/json", "invalid or expired token"},
		{"empty 403", 403, "", "text/plain", "token belongs to the other app"},
		{"empty 429", 429, "", "application/json", "rate limit exceeded"},
		{"empty 500", 500, "", "application/json", "internal server error"},
		{"html 502", 502, "<html><body>Bad Gateway</body></html>", "text/html", "<html>"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := newMockServer(t, mockResponse{Status: tc.status, Body: tc.body, ContentType: tc.contentType})
			c := newTestClient(t, m)
			_, err := c.ListAccounts(testCtx(t), nil)
			var apiErr *Error
			if !asError(err, &apiErr) {
				t.Fatalf("expected *Error, got %T", err)
			}
			if !strings.Contains(apiErr.Message, tc.wantPrefix) {
				t.Errorf("Message = %q, want it to contain %q", apiErr.Message, tc.wantPrefix)
			}
		})
	}
}

func TestRateLimitErrorCarriesRetryAfter(t *testing.T) {
	m := newMockServer(t, mockResponse{
		Status: 429,
		Body:   `{"error":"slow down"}`,
		Header: http.Header{"Retry-After": []string{"42"}},
	})
	c := newTestClient(t, m)
	_, err := c.ListAccounts(testCtx(t), nil)
	apiErr := mustAPIError(t, err, KindRateLimited)
	if apiErr.RetryAfter != 42*time.Second {
		t.Errorf("RetryAfter = %v, want 42s", apiErr.RetryAfter)
	}
	if !strings.Contains(apiErr.Error(), "retry after 42s") {
		t.Errorf("Error() should mention the retry delay: %s", apiErr.Error())
	}
}

func TestRetryAfterAcceptsHTTPDate(t *testing.T) {
	future := time.Now().Add(90 * time.Second).UTC().Format(http.TimeFormat)
	m := newMockServer(t, mockResponse{
		Status: 429,
		Body:   `{"error":"slow down"}`,
		Header: http.Header{"Retry-After": []string{future}},
	})
	c := newTestClient(t, m)
	_, err := c.ListAccounts(testCtx(t), nil)
	apiErr := mustAPIError(t, err, KindRateLimited)
	// Allow slack for the round trip; the point is that a date parsed at all.
	if apiErr.RetryAfter < 60*time.Second || apiErr.RetryAfter > 95*time.Second {
		t.Errorf("RetryAfter = %v, want roughly 90s", apiErr.RetryAfter)
	}
}

func TestRetryAfterInThePastIsZero(t *testing.T) {
	past := time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat)
	if got := parseRetryAfter(past); got != 0 {
		t.Errorf("parseRetryAfter(past) = %v, want 0", got)
	}
	if got := parseRetryAfter("-5"); got != 0 {
		t.Errorf("parseRetryAfter(-5) = %v, want 0", got)
	}
	if got := parseRetryAfter(""); got != 0 {
		t.Errorf("parseRetryAfter(empty) = %v, want 0", got)
	}
	if got := parseRetryAfter("garbage"); got != 0 {
		t.Errorf("parseRetryAfter(garbage) = %v, want 0", got)
	}
}

func TestMalformedJSONBodyIsADecodeError(t *testing.T) {
	m := newMockServer(t, mockResponse{Body: `{"accounts": [ this is not json`})
	c := newTestClient(t, m)
	_, err := c.ListAccounts(testCtx(t), nil)
	apiErr := mustAPIError(t, err, KindDecode)
	if !strings.Contains(apiErr.RawBody, "not json") {
		t.Errorf("RawBody should carry the offending body, got %q", apiErr.RawBody)
	}
	if apiErr.Unwrap() == nil {
		t.Error("decode error should wrap the JSON error")
	}
}

func TestEmptySuccessBodyIsNotAnError(t *testing.T) {
	m := newMockServer(t, mockResponse{Status: 200, Body: ""})
	c := newTestClient(t, m)
	page, err := c.ListAccounts(testCtx(t), nil)
	if err != nil {
		t.Fatalf("empty 200 should decode to a zero page, got %v", err)
	}
	if len(page.Accounts) != 0 {
		t.Errorf("accounts = %v, want empty", page.Accounts)
	}
	if page.Meta.StatusCode != 200 {
		t.Errorf("meta should still be attached, got %+v", page.Meta)
	}
}

func TestUnknownJSONFieldsAreIgnored(t *testing.T) {
	m := newMockServer(t, mockResponse{Body: `{
		"accounts":[{"id":"a1","name":"Checking","brandNewField":{"nested":true}}],
		"limit":30,"offset":0,"someFutureEnvelopeField":123
	}`})
	c := newTestClient(t, m)
	page, err := c.ListAccounts(testCtx(t), nil)
	if err != nil {
		t.Fatalf("unknown fields should not break decoding: %v", err)
	}
	if len(page.Accounts) != 1 || page.Accounts[0].ID != "a1" {
		t.Errorf("accounts = %+v, want one account a1", page.Accounts)
	}
}

func TestTransportErrorIsTyped(t *testing.T) {
	m := newMockServer(t)
	c := newTestClient(t, m)
	m.server.Close() // nothing is listening any more
	_, err := c.ListAccounts(testCtx(t), nil)
	apiErr := mustAPIError(t, err, KindTransport)
	if apiErr.StatusCode != 0 {
		t.Errorf("StatusCode = %d, want 0 for a transport failure", apiErr.StatusCode)
	}
	if !errors.Is(err, ErrTransport) {
		t.Error("errors.Is(err, ErrTransport) = false")
	}
}

func TestContextCancellationIsReported(t *testing.T) {
	m := newMockHandler(t, func(w http.ResponseWriter, r *http.Request, n int) {
		<-r.Context().Done()
	})
	c := newTestClient(t, m)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	_, err := c.ListAccounts(ctx, nil)
	apiErr := mustAPIError(t, err, KindTransport)
	if !strings.Contains(apiErr.Message, "cancelled") {
		t.Errorf("Message = %q, want it to mention cancellation", apiErr.Message)
	}
	if !errors.Is(err, context.Canceled) {
		t.Error("the cancellation cause should be reachable through errors.Is")
	}
}

func TestContextDeadlineIsReported(t *testing.T) {
	m := newMockHandler(t, func(w http.ResponseWriter, r *http.Request, n int) {
		<-r.Context().Done()
	})
	c := newTestClient(t, m)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err := c.ListAccounts(ctx, nil)
	apiErr := mustAPIError(t, err, KindTransport)
	if !strings.Contains(apiErr.Message, "deadline") {
		t.Errorf("Message = %q, want it to mention the deadline", apiErr.Message)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Error("the deadline cause should be reachable through errors.Is")
	}
}

func TestAlreadyCancelledContextSendsNothing(t *testing.T) {
	m := newMockServer(t, mockResponse{JSONBody: map[string]any{}})
	c := newTestClient(t, m)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.ListAccounts(ctx, nil)
	mustAPIError(t, err, KindTransport)
	if m.Count() != 0 {
		t.Errorf("a cancelled context should send nothing, but %d requests arrived", m.Count())
	}
}

func TestNilContextIsTreatedAsBackground(t *testing.T) {
	m := newMockServer(t, mockResponse{JSONBody: map[string]any{"accounts": []any{}}})
	c := newTestClient(t, m)
	//lint:ignore SA1012 deliberately exercising the nil-context guard
	if _, err := c.ListAccounts(nil, nil); err != nil { //nolint:staticcheck
		t.Fatalf("a nil context should be tolerated, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Retry policy
// ---------------------------------------------------------------------------

func TestRetriesRateLimitThenSucceeds(t *testing.T) {
	m := newMockHandler(t, func(w http.ResponseWriter, r *http.Request, n int) {
		if n < 2 {
			writeMock(w, mockResponse{Status: 429, Body: `{"error":"slow down"}`,
				Header: http.Header{"Retry-After": []string{"1"}}})
			return
		}
		writeMock(w, mockResponse{JSONBody: map[string]any{"accounts": []any{map[string]any{"id": "a1"}}}})
	})
	rec := &sleepRecorder{}
	c := newTestClient(t, m, func(cfg *Config) {
		cfg.Retry = RetryConfig{MaxRetries: 3}
		cfg.Sleep = rec.Sleep
	})
	page, err := c.ListAccounts(testCtx(t), nil)
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if len(page.Accounts) != 1 {
		t.Errorf("accounts = %v, want one", page.Accounts)
	}
	if m.Count() != 3 {
		t.Errorf("requests = %d, want 3 (two failures plus the success)", m.Count())
	}
	if page.Meta.Attempts != 3 {
		t.Errorf("Attempts = %d, want 3", page.Meta.Attempts)
	}
	// Retry-After wins over the computed backoff.
	for i, d := range rec.Delays() {
		if d != time.Second {
			t.Errorf("delay[%d] = %v, want the 1s Retry-After", i, d)
		}
	}
}

func TestRetriesExhaustAndReturnTheLastError(t *testing.T) {
	m := newMockServer(t, mockResponse{Status: 500, Body: `{"error":"boom"}`})
	c := newTestClient(t, m, func(cfg *Config) {
		cfg.Retry = RetryConfig{MaxRetries: 2, NoJitter: true, BaseDelay: time.Millisecond}
	})
	_, err := c.ListAccounts(testCtx(t), nil)
	apiErr := mustAPIError(t, err, KindServer)
	if apiErr.Message != "boom" {
		t.Errorf("Message = %q, want boom", apiErr.Message)
	}
	if m.Count() != 3 {
		t.Errorf("requests = %d, want 3 (initial plus 2 retries)", m.Count())
	}
}

func TestNonRetryableStatusIsNotRetried(t *testing.T) {
	for _, status := range []int{400, 401, 403, 404} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			m := newMockServer(t, mockResponse{Status: status, Body: `{"error":"no"}`})
			c := newTestClient(t, m, func(cfg *Config) {
				cfg.Retry = RetryConfig{MaxRetries: 5, NoJitter: true, BaseDelay: time.Millisecond}
			})
			_, _ = c.ListAccounts(testCtx(t), nil)
			if m.Count() != 1 {
				t.Errorf("requests = %d, want 1 — %d must not be retried", m.Count(), status)
			}
		})
	}
}

func TestWritesAreNotRetriedByDefault(t *testing.T) {
	m := newMockServer(t, mockResponse{Status: 500, Body: `{"error":"boom"}`})
	c := newTestClient(t, m, func(cfg *Config) {
		cfg.Retry = RetryConfig{MaxRetries: 3, NoJitter: true, BaseDelay: time.Millisecond}
	})
	_, err := c.CreateLabel(testCtx(t), &CreateLabelRequest{Name: "x"})
	mustAPIError(t, err, KindServer)
	if m.Count() != 1 {
		t.Errorf("requests = %d, want 1 — a POST must not be retried by default", m.Count())
	}
}

func TestWritesAreRetriedWhenOptedIn(t *testing.T) {
	m := newMockHandler(t, func(w http.ResponseWriter, r *http.Request, n int) {
		if n == 0 {
			writeMock(w, mockResponse{Status: 500, Body: `{"error":"boom"}`})
			return
		}
		writeMock(w, mockResponse{Status: 201, JSONBody: map[string]any{"label": map[string]any{"id": "l1"}}})
	})
	c := newTestClient(t, m, func(cfg *Config) {
		cfg.Retry = RetryConfig{MaxRetries: 3, RetryWrites: true, NoJitter: true, BaseDelay: time.Millisecond}
	})
	resp, err := c.CreateLabel(testCtx(t), &CreateLabelRequest{Name: "x"})
	if err != nil {
		t.Fatalf("CreateLabel: %v", err)
	}
	if resp.Label == nil || resp.Label.ID != "l1" {
		t.Errorf("label = %+v, want l1", resp.Label)
	}
	if m.Count() != 2 {
		t.Errorf("requests = %d, want 2", m.Count())
	}
	// The retried POST must re-send the same body, not an empty one.
	for i, req := range m.Requests() {
		if !strings.Contains(string(req.Body), `"x"`) {
			t.Errorf("request %d lost its body: %q", i, req.Body)
		}
	}
}

func TestDeleteIsRetriedBecauseItIsIdempotent(t *testing.T) {
	m := newMockServer(t, mockResponse{Status: 500, Body: `{"error":"boom"}`})
	c := newTestClient(t, m, func(cfg *Config) {
		cfg.Retry = RetryConfig{MaxRetries: 2, NoJitter: true, BaseDelay: time.Millisecond}
	})
	_, err := c.Delete(testCtx(t), EntityLabels, []string{"l1"})
	mustAPIError(t, err, KindServer)
	if m.Count() != 3 {
		t.Errorf("requests = %d, want 3 — DELETE is idempotent and should retry", m.Count())
	}
}

func TestRetryDisabledEntirely(t *testing.T) {
	m := newMockServer(t, mockResponse{Status: 503, Body: `{"error":"boom"}`})
	c := newTestClient(t, m, func(cfg *Config) {
		cfg.Retry = RetryConfig{MaxRetries: -1}
	})
	_, _ = c.ListAccounts(testCtx(t), nil)
	if m.Count() != 1 {
		t.Errorf("requests = %d, want 1 with retries disabled", m.Count())
	}
}

func TestCustomRetryPredicate(t *testing.T) {
	var seen []ErrorKind
	m := newMockServer(t, mockResponse{Status: 404, Body: `{"error":"nope"}`})
	c := newTestClient(t, m, func(cfg *Config) {
		cfg.Retry = RetryConfig{
			MaxRetries: 2,
			NoJitter:   true,
			BaseDelay:  time.Millisecond,
			RetryOn: func(attempt int, err *Error) bool {
				seen = append(seen, err.Kind)
				return err.Kind == KindNotFound && attempt < 1
			},
		}
	})
	_, _ = c.ListAccounts(testCtx(t), nil)
	if m.Count() != 2 {
		t.Errorf("requests = %d, want 2 — the predicate allowed exactly one retry", m.Count())
	}
	if len(seen) == 0 || seen[0] != KindNotFound {
		t.Errorf("predicate saw %v, want it to start with not_found", seen)
	}
}

func TestBackoffGrowsExponentiallyAndIsCapped(t *testing.T) {
	c := &Client{retry: RetryConfig{BaseDelay: 100 * time.Millisecond, MaxDelay: time.Second, NoJitter: true}}
	got := []time.Duration{c.backoff(0, nil), c.backoff(1, nil), c.backoff(2, nil), c.backoff(3, nil), c.backoff(10, nil)}
	want := []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond, 800 * time.Millisecond, time.Second}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("backoff(%d) = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestBackoffJitterStaysInTheUpperHalf(t *testing.T) {
	c := &Client{
		retry:     RetryConfig{BaseDelay: time.Second, MaxDelay: time.Minute},
		randFloat: func() float64 { return 0 },
	}
	if got, want := c.backoff(0, nil), 500*time.Millisecond; got != want {
		t.Errorf("jitter floor = %v, want %v", got, want)
	}
	c.randFloat = func() float64 { return 1 }
	if got, want := c.backoff(0, nil), time.Second; got != want {
		t.Errorf("jitter ceiling = %v, want %v", got, want)
	}
}

func TestRetryAfterIsCappedByMaxDelay(t *testing.T) {
	c := &Client{retry: RetryConfig{BaseDelay: time.Second, MaxDelay: 5 * time.Second, NoJitter: true}}
	got := c.backoff(0, &Error{RetryAfter: time.Hour})
	if got != 5*time.Second {
		t.Errorf("backoff = %v, want it capped at MaxDelay (5s)", got)
	}
}

func TestRetryAfterCanBeIgnored(t *testing.T) {
	c := &Client{retry: RetryConfig{BaseDelay: 10 * time.Millisecond, MaxDelay: time.Minute, NoJitter: true, DisableRetryAfter: true}}
	got := c.backoff(0, &Error{RetryAfter: time.Hour})
	if got != 10*time.Millisecond {
		t.Errorf("backoff = %v, want the computed 10ms when Retry-After is disabled", got)
	}
}

func TestSkipValidationBypassesClientChecks(t *testing.T) {
	m := newMockServer(t, mockResponse{Status: 400, Body: `{"error":"limit must be between 1 and 200"}`})
	c := newTestClient(t, m, func(cfg *Config) { cfg.SkipValidation = true })
	_, err := c.ListAccounts(testCtx(t), &ListAccountsParams{Limit: Int(9999)})
	// With validation off the call goes out and the server decides.
	mustAPIError(t, err, KindValidation)
	if m.Count() != 1 {
		t.Errorf("requests = %d, want 1 — validation was disabled", m.Count())
	}
	if got := m.Last(t).Query.Get("limit"); got != "9999" {
		t.Errorf("limit = %q, want it sent verbatim", got)
	}
}

func TestValidationFailureSendsNothing(t *testing.T) {
	m := newMockServer(t, mockResponse{JSONBody: map[string]any{}})
	c := newTestClient(t, m)
	_, err := c.ListAccounts(testCtx(t), &ListAccountsParams{Limit: Int(0)})
	errs := mustInvalidRequest(t, err)
	wantField(t, errs, "limit")
	if m.Count() != 0 {
		t.Errorf("a validation failure must not reach the network, but %d requests arrived", m.Count())
	}
}

func TestErrorRetryableClassification(t *testing.T) {
	tests := map[ErrorKind]bool{
		KindRateLimited:      true,
		KindServer:           true,
		KindTransport:        true,
		KindValidation:       false,
		KindUnauthorized:     false,
		KindForbidden:        false,
		KindNotFound:         false,
		KindDecode:           false,
		KindUnexpectedStatus: false,
	}
	for kind, want := range tests {
		if got := (&Error{Kind: kind}).Retryable(); got != want {
			t.Errorf("Retryable(%s) = %v, want %v", kind, got, want)
		}
	}
}

func TestMustNewPanicsOnBadConfig(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("MustNew should panic on an unusable config")
		}
	}()
	MustNew(Config{})
}
