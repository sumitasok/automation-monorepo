package wallet

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// DefaultBaseURL is the production Wallet API server.
const DefaultBaseURL = "https://rest.budgetbakers.com/wallet"

// apiPrefix is the version segment every operation sits under.
const apiPrefix = "/v1/api"

// DefaultUserAgent identifies this SDK in request logs.
const DefaultUserAgent = "wallet-go-sdk/1.0 (+https://rest.budgetbakers.com/wallet)"

// Default transport tuning.
const (
	// DefaultTimeout bounds a single request including its body.
	DefaultTimeout = 30 * time.Second
	// DefaultMaxRetries is how many times a retryable failure is re-sent.
	DefaultMaxRetries = 3
	// DefaultRetryBaseDelay is the first backoff step.
	DefaultRetryBaseDelay = 500 * time.Millisecond
	// DefaultRetryMaxDelay caps the backoff.
	DefaultRetryMaxDelay = 30 * time.Second
	// maxErrorBodyBytes bounds how much of an error body is kept.
	maxErrorBodyBytes = 8 << 10
	// maxResponseBytes bounds a decoded response, so a runaway body cannot
	// exhaust memory. 32 MiB is far above a 200-item page.
	maxResponseBytes = 32 << 20
)

// Config configures a Client. Only Token is required.
type Config struct {
	// Token is the bearer JWT minted for the Wallet flavour of the API.
	Token string
	// BaseURL overrides the API server. Defaults to DefaultBaseURL. A path
	// component is preserved, so a proxy mounted at /api/wallet works.
	BaseURL string
	// HTTPClient overrides the underlying client. Defaults to a client with
	// Timeout set to Timeout.
	HTTPClient *http.Client
	// Timeout bounds each attempt when HTTPClient is not supplied. Defaults
	// to DefaultTimeout. A negative value disables the timeout.
	Timeout time.Duration
	// UserAgent overrides the User-Agent header.
	UserAgent string
	// Header carries extra headers added to every request. Authorization,
	// Accept and Content-Type are always set by the client and cannot be
	// overridden here.
	Header http.Header
	// Retry configures the retry policy. The zero value uses the defaults.
	Retry RetryConfig
	// Now overrides the clock, for tests.
	Now func() time.Time
	// Sleep overrides the backoff sleep, for tests. It must honour ctx.
	Sleep func(ctx context.Context, d time.Duration) error
	// Rand overrides the jitter source, for tests.
	Rand func() float64
	// SkipValidation turns off the SDK's client-side checks. The server still
	// enforces everything; this only trades early, precise errors for a round
	// trip. Useful when the API adds a value this SDK does not yet know.
	SkipValidation bool
}

// RetryConfig is the retry policy.
//
// By default only idempotent requests are retried: GET, and DELETE (whose
// per-ID semantics the API documents as idempotent). Retrying a POST or PATCH
// can duplicate writes, so those need RetryWrites set explicitly.
type RetryConfig struct {
	// MaxRetries is how many extra attempts a retryable failure gets. Zero
	// uses DefaultMaxRetries; a negative value disables retries entirely.
	MaxRetries int
	// BaseDelay is the first backoff step. Zero uses DefaultRetryBaseDelay.
	BaseDelay time.Duration
	// MaxDelay caps the backoff. Zero uses DefaultRetryMaxDelay.
	MaxDelay time.Duration
	// RetryWrites opts POST and PATCH into retries. Off by default because a
	// retried create can duplicate records.
	RetryWrites bool
	// NoJitter makes the backoff deterministic. Jitter is on by default so a
	// fleet of clients does not resynchronise after a rate limit.
	NoJitter bool
	// RespectRetryAfter honours the Retry-After header in preference to the
	// computed backoff. On by default; set DisableRetryAfter to turn it off.
	DisableRetryAfter bool
	// RetryOn overrides which failures are retried. It sees the attempt's
	// error (always a *Error) and returns whether to try again. When nil the
	// default policy applies.
	RetryOn func(attempt int, err *Error) bool
}

func (r RetryConfig) maxRetries() int {
	switch {
	case r.MaxRetries < 0:
		return 0
	case r.MaxRetries == 0:
		return DefaultMaxRetries
	default:
		return r.MaxRetries
	}
}

func (r RetryConfig) baseDelay() time.Duration {
	if r.BaseDelay <= 0 {
		return DefaultRetryBaseDelay
	}
	return r.BaseDelay
}

func (r RetryConfig) maxDelay() time.Duration {
	if r.MaxDelay <= 0 {
		return DefaultRetryMaxDelay
	}
	return r.MaxDelay
}

// Client is a Wallet API client. It is safe for concurrent use.
type Client struct {
	baseURL        *url.URL
	token          string
	httpClient     *http.Client
	userAgent      string
	header         http.Header
	retry          RetryConfig
	now            func() time.Time
	sleep          func(ctx context.Context, d time.Duration) error
	randFloat      func() float64
	skipValidation bool

	// mu guards lastMeta, which records the metadata of the most recent
	// response so a caller can watch the rate-limit budget globally.
	mu       sync.RWMutex
	lastMeta ResponseMeta
}

// New builds a Client. It fails only on an unusable configuration: a missing
// token or an unparseable base URL.
func New(cfg Config) (*Client, error) {
	var p problems
	if strings.TrimSpace(cfg.Token) == "" {
		p.add("Token", "is required")
	}
	raw := cfg.BaseURL
	if raw == "" {
		raw = DefaultBaseURL
	}
	base, err := url.Parse(raw)
	if err != nil {
		p.add("BaseURL", "is not a valid URL: %v", err)
	} else if base.Scheme != "http" && base.Scheme != "https" {
		p.add("BaseURL", "must be http or https, got %q", base.Scheme)
	} else if base.Host == "" {
		p.add("BaseURL", "must include a host")
	}
	if err := p.err(); err != nil {
		return nil, err
	}
	base.Path = strings.TrimSuffix(base.Path, "/")

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		timeout := cfg.Timeout
		if timeout == 0 {
			timeout = DefaultTimeout
		}
		if timeout < 0 {
			timeout = 0
		}
		httpClient = &http.Client{Timeout: timeout}
	}

	ua := cfg.UserAgent
	if ua == "" {
		ua = DefaultUserAgent
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	sleep := cfg.Sleep
	if sleep == nil {
		sleep = sleepCtx
	}
	randFloat := cfg.Rand
	if randFloat == nil {
		randFloat = rand.Float64
	}

	return &Client{
		baseURL:        base,
		token:          cfg.Token,
		httpClient:     httpClient,
		userAgent:      ua,
		header:         cfg.Header.Clone(),
		retry:          cfg.Retry,
		now:            now,
		sleep:          sleep,
		randFloat:      randFloat,
		skipValidation: cfg.SkipValidation,
	}, nil
}

// MustNew is New for package-level initialisation; it panics on a bad config.
func MustNew(cfg Config) *Client {
	c, err := New(cfg)
	if err != nil {
		panic(err)
	}
	return c
}

// BaseURL returns the configured server, without the /v1/api prefix.
func (c *Client) BaseURL() string { return c.baseURL.String() }

// LastMeta returns the metadata of the most recent response the client
// received, including errors. The zero value means nothing has been sent yet.
func (c *Client) LastMeta() ResponseMeta {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastMeta
}

func (c *Client) recordMeta(m ResponseMeta) {
	c.mu.Lock()
	c.lastMeta = m
	c.mu.Unlock()
}

// validate runs a Validatable unless validation is disabled.
func (c *Client) validate(v Validatable) error {
	if c.skipValidation || v == nil {
		return nil
	}
	return v.Validate()
}

// sleepCtx waits for d, or until ctx is done.
func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// ---------------------------------------------------------------------------
// Request plumbing
// ---------------------------------------------------------------------------

// request describes one API call.
type request struct {
	// op is the OpenAPI operationId, used in errors.
	op string
	// method is the HTTP method.
	method string
	// path is appended to the /v1/api prefix, e.g. "/accounts".
	path string
	// query holds the already-serialised query parameters.
	query url.Values
	// body is marshalled to JSON when non-nil.
	body any
	// batch marks an endpoint whose 200, 207, 400 and 500 responses all carry
	// the same envelope, so those statuses decode instead of erroring.
	batch bool
}

// url builds the absolute URL for the request.
func (c *Client) url(r request) string {
	u := *c.baseURL
	u.Path = u.Path + apiPrefix + r.path
	if len(r.query) > 0 {
		u.RawQuery = r.query.Encode()
	}
	return u.String()
}

// do executes a request with retries and decodes the response into out.
func (c *Client) do(ctx context.Context, r request, out metaSetter) error {
	if ctx == nil {
		ctx = context.Background()
	}

	var payload []byte
	if r.body != nil {
		var err error
		payload, err = json.Marshal(r.body)
		if err != nil {
			return &Error{Op: r.op, Kind: KindDecode, Message: "cannot encode request body", Err: err}
		}
	}

	endpoint := c.url(r)
	maxRetries := c.retry.maxRetries()

	var lastErr *Error
	for attempt := 0; ; attempt++ {
		// A cancelled context short-circuits before burning an attempt.
		if err := ctx.Err(); err != nil {
			if lastErr != nil {
				return lastErr
			}
			return &Error{Op: r.op, Kind: KindTransport, Message: "context done before request", Err: err}
		}

		err := c.attempt(ctx, r, endpoint, payload, out, attempt+1)
		if err == nil {
			return nil
		}
		var apiErr *Error
		if !errors.As(err, &apiErr) {
			// attempt only ever returns *Error; this is belt and braces.
			return err
		}
		lastErr = apiErr

		if attempt >= maxRetries || !c.shouldRetry(r, attempt, apiErr) {
			return apiErr
		}
		delay := c.backoff(attempt, apiErr)
		if err := c.sleep(ctx, delay); err != nil {
			// The context died while waiting: report the failure that caused
			// the wait, not the cancellation, since it is the real cause.
			return apiErr
		}
	}
}

// attempt performs one HTTP round trip.
func (c *Client) attempt(ctx context.Context, r request, endpoint string, payload []byte, out metaSetter, attemptNo int) error {
	var bodyReader io.Reader
	if payload != nil {
		bodyReader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, r.method, endpoint, bodyReader)
	if err != nil {
		return &Error{Op: r.op, Kind: KindTransport, Message: "cannot build request", Err: err}
	}
	for k, vs := range c.header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
		req.ContentLength = int64(len(payload))
		// GetBody lets the stdlib replay the body across redirects, and lets
		// our own retries reuse the same request shape safely.
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(payload)), nil
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		kind := KindTransport
		msg := "request failed"
		// A cancelled or expired context is reported as such, so a caller can
		// tell a deadline from a network fault.
		if ctxErr := ctx.Err(); ctxErr != nil {
			msg = "request cancelled"
			if errors.Is(ctxErr, context.DeadlineExceeded) {
				msg = "request deadline exceeded"
			}
		}
		return &Error{Op: r.op, Kind: kind, Message: msg, Err: err}
	}
	defer func() {
		// Drain a little so the connection can be reused, then close.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
		_ = resp.Body.Close()
	}()

	meta := parseMeta(resp, attemptNo)
	c.recordMeta(meta)

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if readErr != nil {
		return &Error{
			Op: r.op, Kind: KindTransport, StatusCode: resp.StatusCode,
			Message: "cannot read response body", Meta: meta, Err: readErr,
			RetryAfter: meta.RetryAfter,
		}
	}

	return c.handle(r, resp, meta, body, out)
}

// handle turns one response into either a decoded envelope or a typed error.
func (c *Client) handle(r request, resp *http.Response, meta ResponseMeta, body []byte, out metaSetter) error {
	status := resp.StatusCode

	// A batch endpoint answers 200/207/400/500 with the same envelope. Decode
	// those rather than erroring — but only when the body really is a batch
	// envelope, because a malformed request still yields a plain
	// ValidationError with the same 400.
	if r.batch && isBatchStatus(status) && looksLikeBatchEnvelope(body) {
		if err := decodeInto(r.op, meta, body, out); err != nil {
			return err
		}
		return nil
	}

	if status >= 200 && status < 300 {
		if out == nil {
			return nil
		}
		// 204 and an empty 200 leave the envelope zeroed but still carry meta.
		if len(bytes.TrimSpace(body)) == 0 {
			out.setMeta(meta)
			return nil
		}
		return decodeInto(r.op, meta, body, out)
	}

	return c.apiError(r, status, meta, body)
}

// apiError classifies a non-2xx response.
func (c *Client) apiError(r request, status int, meta ResponseMeta, body []byte) *Error {
	e := &Error{
		Op:         r.op,
		StatusCode: status,
		Meta:       meta,
		RawBody:    truncate(string(body), maxErrorBodyBytes),
		RetryAfter: meta.RetryAfter,
	}
	e.Message = extractErrorMessage(resp0ContentType(meta), body)

	switch {
	case status == http.StatusBadRequest:
		e.Kind = KindValidation
	case status == http.StatusUnauthorized:
		e.Kind = KindUnauthorized
		if e.Message == "" {
			e.Message = "invalid or expired token"
		}
	case status == http.StatusForbidden:
		e.Kind = KindForbidden
		if e.Message == "" {
			e.Message = "token belongs to the other app; use a token minted for this API flavour"
		}
	case status == http.StatusNotFound:
		e.Kind = KindNotFound
		if e.Message == "" {
			e.Message = "endpoint or entity type not found"
		}
	case status == http.StatusTooManyRequests:
		e.Kind = KindRateLimited
		if e.Message == "" {
			e.Message = "rate limit exceeded"
		}
	case status >= 500:
		e.Kind = KindServer
		if e.Message == "" {
			e.Message = "internal server error"
		}
	default:
		e.Kind = KindUnexpectedStatus
		if e.Message == "" {
			e.Message = fmt.Sprintf("unexpected status %d", status)
		}
	}
	return e
}

func resp0ContentType(meta ResponseMeta) string {
	if meta.Header == nil {
		return ""
	}
	return meta.Header.Get("Content-Type")
}

// extractErrorMessage pulls the message out of a JSON {"error": "..."} body,
// falling back to the raw text for the text/plain 403 and for anything the
// API returns that is not JSON at all (a proxy's HTML error page, say).
func extractErrorMessage(contentType string, body []byte) string {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return ""
	}
	if strings.Contains(strings.ToLower(contentType), "json") || trimmed[0] == '{' {
		var envelope struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(trimmed, &envelope); err == nil {
			if envelope.Error != "" {
				return envelope.Error
			}
			if envelope.Message != "" {
				return envelope.Message
			}
		}
	}
	return truncate(strings.TrimSpace(string(trimmed)), 512)
}

// decodeInto unmarshals a body into an envelope and attaches the metadata.
func decodeInto(op string, meta ResponseMeta, body []byte, out metaSetter) error {
	if out == nil {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	if err := dec.Decode(out); err != nil {
		return &Error{
			Op: op, Kind: KindDecode, StatusCode: meta.StatusCode, Meta: meta,
			Message: "cannot decode response body",
			RawBody: truncate(string(body), maxErrorBodyBytes),
			Err:     err,
		}
	}
	out.setMeta(meta)
	return nil
}

// isBatchStatus reports whether a status can carry a batch envelope.
func isBatchStatus(status int) bool {
	switch status {
	case http.StatusOK, http.StatusMultiStatus, http.StatusBadRequest, http.StatusInternalServerError:
		return true
	default:
		return false
	}
}

// looksLikeBatchEnvelope distinguishes a per-item batch body from a plain
// ValidationError. Both can arrive as 400: the batch body carries "results"
// or "summary", the validation error carries only "error".
func looksLikeBatchEnvelope(body []byte) bool {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return false
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &probe); err != nil {
		return false
	}
	_, hasResults := probe["results"]
	_, hasSummary := probe["summary"]
	return hasResults || hasSummary
}

// ---------------------------------------------------------------------------
// Retry policy
// ---------------------------------------------------------------------------

// shouldRetry applies the policy to one failure.
func (c *Client) shouldRetry(r request, attempt int, err *Error) bool {
	if c.retry.RetryOn != nil {
		return c.retry.RetryOn(attempt, err)
	}
	if !err.Retryable() {
		return false
	}
	// A batch endpoint's 500 carries per-item detail the caller needs; it is
	// already surfaced as an envelope, not an error, so a 500 that reaches
	// here had no envelope and is a genuine server fault.
	switch r.method {
	case http.MethodGet, http.MethodHead, http.MethodDelete:
		return true
	default:
		return c.retry.RetryWrites
	}
}

// backoff computes the wait before the next attempt: Retry-After when the
// server sent one, otherwise exponential backoff with full jitter, capped.
func (c *Client) backoff(attempt int, err *Error) time.Duration {
	if !c.retry.DisableRetryAfter && err != nil && err.RetryAfter > 0 {
		if err.RetryAfter > c.retry.maxDelay() {
			return c.retry.maxDelay()
		}
		return err.RetryAfter
	}
	base := float64(c.retry.baseDelay())
	// 2^attempt, guarded against overflow on a pathological MaxRetries.
	factor := math.Pow(2, float64(attempt))
	if math.IsInf(factor, 1) || factor > 1e6 {
		factor = 1e6
	}
	d := time.Duration(base * factor)
	if d > c.retry.maxDelay() || d <= 0 {
		d = c.retry.maxDelay()
	}
	if c.retry.NoJitter {
		return d
	}
	// Full jitter: uniform in [d/2, d], which keeps a floor while still
	// spreading a thundering herd.
	half := d / 2
	return half + time.Duration(c.randFloat()*float64(half))
}
