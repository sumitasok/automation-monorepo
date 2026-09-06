package wallet

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"
)

// capturedRequest records everything the mock server saw, so a test can assert
// on the exact wire form the SDK produced.
type capturedRequest struct {
	Method string
	Path   string
	Query  url.Values
	Header http.Header
	Body   []byte
}

// JSON decodes the captured body into v, failing the test on bad JSON.
func (c capturedRequest) JSON(t *testing.T, v any) {
	t.Helper()
	if err := json.Unmarshal(c.Body, v); err != nil {
		t.Fatalf("captured body is not valid JSON: %v\nbody: %s", err, c.Body)
	}
}

// mockResponse is one canned reply.
type mockResponse struct {
	// Status is the HTTP status to send. Zero means 200.
	Status int
	// Body is sent verbatim when non-empty.
	Body string
	// JSONBody is marshalled and sent when Body is empty.
	JSONBody any
	// Header holds extra response headers.
	Header http.Header
	// ContentType overrides the default application/json.
	ContentType string
}

// mockServer is an httptest server that replays a queue of canned responses
// and records every request it received.
type mockServer struct {
	t *testing.T
	// server is the live test server.
	server *httptest.Server

	mu sync.Mutex
	// queue holds the responses still to be sent, in order.
	queue []mockResponse
	// requests holds every request received, in order.
	requests []capturedRequest
	// handler, when set, takes precedence over the queue.
	handler func(w http.ResponseWriter, r *http.Request, n int)
}

// newMockServer starts a server that replays responses in order. When the
// queue runs dry the last response is repeated, which keeps a retry test from
// depending on the exact attempt count.
func newMockServer(t *testing.T, responses ...mockResponse) *mockServer {
	t.Helper()
	m := &mockServer{t: t, queue: responses}
	m.server = httptest.NewServer(http.HandlerFunc(m.serve))
	t.Cleanup(m.server.Close)
	return m
}

// newMockHandler starts a server driven by a handler func, for tests that need
// to branch on the request. n is the 0-based request ordinal.
func newMockHandler(t *testing.T, handler func(w http.ResponseWriter, r *http.Request, n int)) *mockServer {
	t.Helper()
	m := &mockServer{t: t, handler: handler}
	m.server = httptest.NewServer(http.HandlerFunc(m.serve))
	t.Cleanup(m.server.Close)
	return m
}

func (m *mockServer) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	m.mu.Lock()
	n := len(m.requests)
	m.requests = append(m.requests, capturedRequest{
		Method: r.Method,
		Path:   r.URL.Path,
		Query:  r.URL.Query(),
		Header: r.Header.Clone(),
		Body:   body,
	})
	handler := m.handler
	var resp mockResponse
	hasResp := false
	if handler == nil {
		switch {
		case len(m.queue) == 0:
			resp = mockResponse{Status: 200, Body: `{}`}
		case len(m.queue) == 1:
			resp = m.queue[0] // repeat the last one forever
		default:
			resp = m.queue[0]
			m.queue = m.queue[1:]
		}
		hasResp = true
	}
	m.mu.Unlock()

	if handler != nil {
		handler(w, r, n)
		return
	}
	if hasResp {
		writeMock(w, resp)
	}
}

func writeMock(w http.ResponseWriter, resp mockResponse) {
	for k, vs := range resp.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	ct := resp.ContentType
	if ct == "" {
		ct = "application/json"
	}
	w.Header().Set("Content-Type", ct)
	status := resp.Status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	switch {
	case resp.Body != "":
		_, _ = io.WriteString(w, resp.Body)
	case resp.JSONBody != nil:
		_ = json.NewEncoder(w).Encode(resp.JSONBody)
	}
}

// URL is the base URL a Client should be pointed at.
func (m *mockServer) URL() string { return m.server.URL }

// Requests returns a snapshot of everything received so far.
func (m *mockServer) Requests() []capturedRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]capturedRequest, len(m.requests))
	copy(out, m.requests)
	return out
}

// Count returns how many requests were received.
func (m *mockServer) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.requests)
}

// Last returns the most recent request, failing the test if there was none.
func (m *mockServer) Last(t *testing.T) capturedRequest {
	t.Helper()
	reqs := m.Requests()
	if len(reqs) == 0 {
		t.Fatal("no requests were received by the mock server")
	}
	return reqs[len(reqs)-1]
}

// First returns the first request, failing the test if there was none.
func (m *mockServer) First(t *testing.T) capturedRequest {
	t.Helper()
	reqs := m.Requests()
	if len(reqs) == 0 {
		t.Fatal("no requests were received by the mock server")
	}
	return reqs[0]
}

// fixedTime is the clock every test shares, so nothing depends on wall time.
var fixedTime = time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

// newTestClient builds a client pointed at the mock server, with retries off,
// a frozen clock and an instant sleeper, so tests never actually wait.
func newTestClient(t *testing.T, m *mockServer, tweak ...func(*Config)) *Client {
	t.Helper()
	slept := &sleepRecorder{}
	cfg := Config{
		Token:   "test-token",
		BaseURL: m.URL(),
		Retry:   RetryConfig{MaxRetries: -1},
		Now:     func() time.Time { return fixedTime },
		Sleep:   slept.Sleep,
		Rand:    func() float64 { return 0.5 },
	}
	for _, fn := range tweak {
		fn(&cfg)
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

// sleepRecorder stands in for the backoff sleep, recording the delays asked
// for without spending any real time.
type sleepRecorder struct {
	mu     sync.Mutex
	delays []time.Duration
}

func (s *sleepRecorder) Sleep(ctx context.Context, d time.Duration) error {
	s.mu.Lock()
	s.delays = append(s.delays, d)
	s.mu.Unlock()
	return ctx.Err()
}

func (s *sleepRecorder) Delays() []time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]time.Duration, len(s.delays))
	copy(out, s.delays)
	return out
}

// ctx is the context every test uses.
func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// mustAPIError asserts that err is a *Error of the expected kind and returns it.
func mustAPIError(t *testing.T, err error, kind ErrorKind) *Error {
	t.Helper()
	if err == nil {
		t.Fatalf("expected a %s error, got nil", kind)
	}
	var apiErr *Error
	if !asError(err, &apiErr) {
		t.Fatalf("expected *wallet.Error, got %T: %v", err, err)
	}
	if apiErr.Kind != kind {
		t.Fatalf("expected kind %q, got %q (%v)", kind, apiErr.Kind, apiErr)
	}
	return apiErr
}

// asError is errors.As with a narrower signature, kept here so the helper file
// owns the import.
func asError(err error, target **Error) bool {
	for err != nil {
		if e, ok := err.(*Error); ok {
			*target = e
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

// mustInvalidRequest asserts that err is a client-side validation failure and
// returns the individual problems.
func mustInvalidRequest(t *testing.T, err error) InvalidRequestErrors {
	t.Helper()
	if err == nil {
		t.Fatal("expected a validation error, got nil")
	}
	var errs InvalidRequestErrors
	switch v := err.(type) {
	case InvalidRequestErrors:
		errs = v
	case *InvalidRequestError:
		errs = InvalidRequestErrors{v}
	default:
		t.Fatalf("expected a validation error, got %T: %v", err, err)
	}
	return errs
}

// hasField reports whether any problem names the given field path.
func (e InvalidRequestErrors) hasField(field string) bool {
	for _, v := range e {
		if v.Field == field {
			return true
		}
	}
	return false
}

// fields lists every field path mentioned, for failure messages.
func (e InvalidRequestErrors) fields() []string {
	out := make([]string, len(e))
	for i, v := range e {
		out[i] = v.Field
	}
	return out
}

// wantField fails the test unless a problem names the given field path.
func wantField(t *testing.T, errs InvalidRequestErrors, field string) {
	t.Helper()
	if !errs.hasField(field) {
		t.Errorf("expected a problem on field %q, got problems on %v\n%v", field, errs.fields(), errs)
	}
}
