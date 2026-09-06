package wallet

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestErrorStringIncludesEveryPart(t *testing.T) {
	e := &Error{
		Op:         "getRecords",
		Kind:       KindRateLimited,
		StatusCode: 429,
		Message:    "slow down",
		RetryAfter: 30 * time.Second,
	}
	got := e.Error()
	for _, want := range []string{"wallet:", "getRecords", "429", "slow down", "retry after 30s"} {
		if !strings.Contains(got, want) {
			t.Errorf("Error() = %q, want it to contain %q", got, want)
		}
	}
}

func TestErrorStringWithoutAnOperationOrStatus(t *testing.T) {
	e := &Error{Kind: KindTransport, Message: "request failed", Err: errors.New("dial tcp: refused")}
	got := e.Error()
	if !strings.Contains(got, "request failed") || !strings.Contains(got, "dial tcp: refused") {
		t.Errorf("Error() = %q, want the message and the wrapped cause", got)
	}
	if strings.Contains(got, "0 ") {
		t.Errorf("Error() = %q, should not print a zero status", got)
	}
}

func TestErrorStringFallsBackToTheKind(t *testing.T) {
	e := &Error{Kind: KindDecode}
	if got := e.Error(); !strings.Contains(got, "decode") {
		t.Errorf("Error() = %q, want the kind when there is no message", got)
	}
}

func TestErrorIsMatchesOnlyItsOwnSentinel(t *testing.T) {
	e := &Error{Kind: KindNotFound}
	if !errors.Is(e, ErrNotFound) {
		t.Error("a not_found error should match ErrNotFound")
	}
	if errors.Is(e, ErrServer) {
		t.Error("a not_found error must not match ErrServer")
	}
	if errors.Is(e, ErrInvalidRequest) {
		t.Error("a wire error must not match ErrInvalidRequest")
	}
	unknown := &Error{Kind: "brand_new"}
	if errors.Is(unknown, ErrServer) {
		t.Error("an unmapped kind should match nothing")
	}
}

func TestErrorUnwrapReachesTheCause(t *testing.T) {
	cause := context.DeadlineExceeded
	e := &Error{Kind: KindTransport, Err: cause}
	if e.Unwrap() != cause {
		t.Errorf("Unwrap() = %v, want the cause", e.Unwrap())
	}
	if !errors.Is(e, context.DeadlineExceeded) {
		t.Error("errors.Is should reach the wrapped cause")
	}
	if (&Error{}).Unwrap() != nil {
		t.Error("an error with no cause unwraps to nil")
	}
}

func TestInvalidRequestErrorFormatting(t *testing.T) {
	withField := &InvalidRequestError{Field: "limit", Reason: "must be between 1 and 200"}
	if got := withField.Error(); got != "wallet: invalid request: limit: must be between 1 and 200" {
		t.Errorf("Error() = %q", got)
	}
	withoutField := &InvalidRequestError{Reason: "request body is required"}
	if got := withoutField.Error(); got != "wallet: invalid request: request body is required" {
		t.Errorf("Error() = %q", got)
	}
	if !errors.Is(withField, ErrInvalidRequest) {
		t.Error("a validation failure should match ErrInvalidRequest")
	}
}

func TestInvalidRequestErrorsFormatting(t *testing.T) {
	single := InvalidRequestErrors{{Field: "limit", Reason: "too big"}}
	if got := single.Error(); !strings.Contains(got, "limit: too big") || strings.Contains(got, "problems") {
		t.Errorf("a single problem should read like one: %q", got)
	}
	many := InvalidRequestErrors{
		{Field: "limit", Reason: "too big"},
		{Reason: "body is required"},
	}
	got := many.Error()
	for _, want := range []string{"2 problems", "limit: too big", "body is required"} {
		if !strings.Contains(got, want) {
			t.Errorf("Error() = %q, want it to contain %q", got, want)
		}
	}
	if !errors.Is(many, ErrInvalidRequest) {
		t.Error("a collection should match ErrInvalidRequest")
	}
}

func TestInvalidRequestErrorsUnwrapExposesEachProblem(t *testing.T) {
	errs := InvalidRequestErrors{
		{Field: "a", Reason: "one"},
		{Field: "b", Reason: "two"},
	}
	unwrapped := errs.Unwrap()
	if len(unwrapped) != 2 {
		t.Fatalf("Unwrap() returned %d errors, want 2", len(unwrapped))
	}
	var single *InvalidRequestError
	if !errors.As(errs, &single) {
		t.Fatal("errors.As should find an individual problem")
	}
	if single.Field != "a" {
		t.Errorf("errors.As found %q, want the first problem", single.Field)
	}
}

func TestProblemsAggregation(t *testing.T) {
	var p problems
	if p.err() != nil {
		t.Error("an empty accumulator should collapse to nil")
	}
	p.add("a", "reason %d", 1)
	p.addErr(nil)
	p.addErr(errorf("b", "reason 2"))
	p.addErr(InvalidRequestErrors{{Field: "c", Reason: "reason 3"}})
	p.addErr(errors.New("a plain error"))
	errs, ok := p.err().(InvalidRequestErrors)
	if !ok {
		t.Fatalf("err() = %T, want InvalidRequestErrors", p.err())
	}
	if len(errs) != 4 {
		t.Fatalf("collected %d problems, want 4", len(errs))
	}
	if errs[3].Field != "" || errs[3].Reason != "a plain error" {
		t.Errorf("a plain error should be wrapped verbatim, got %+v", errs[3])
	}
}

func TestProblemsPrefixing(t *testing.T) {
	var p problems
	p.prefixed("items[0]", errorf("name", "too long"))
	p.prefixed("items[1]", InvalidRequestErrors{{Field: "amount.value", Reason: "zero"}})
	p.prefixed("items[2]", InvalidRequestErrors{{Field: "", Reason: "nothing to update"}})
	p.prefixed("items[3]", errors.New("plain"))
	p.prefixed("root", InvalidRequestErrors{{Field: "[0]", Reason: "indexed"}})
	p.prefixed("ignored", nil)

	errs := p.err().(InvalidRequestErrors)
	want := []string{"items[0].name", "items[1].amount.value", "items[2]", "items[3]", "root[0]"}
	if len(errs) != len(want) {
		t.Fatalf("collected %d problems, want %d: %v", len(errs), len(want), errs.fields())
	}
	for i, field := range want {
		if errs[i].Field != field {
			t.Errorf("problem %d field = %q, want %q", i, errs[i].Field, field)
		}
	}
}

func TestJoinPath(t *testing.T) {
	tests := []struct{ prefix, field, want string }{
		{"", "name", "name"},
		{"items[0]", "", "items[0]"},
		{"items[0]", "name", "items[0].name"},
		{"items", "[0]", "items[0]"},
		{"", "", ""},
	}
	for _, tc := range tests {
		if got := joinPath(tc.prefix, tc.field); got != tc.want {
			t.Errorf("joinPath(%q, %q) = %q, want %q", tc.prefix, tc.field, got, tc.want)
		}
	}
}

func TestSleepCtxWaitsAndHonoursCancellation(t *testing.T) {
	start := time.Now()
	if err := sleepCtx(context.Background(), 20*time.Millisecond); err != nil {
		t.Fatalf("sleepCtx: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 15*time.Millisecond {
		t.Errorf("sleepCtx returned after %v, want it to wait ~20ms", elapsed)
	}

	// A zero or negative delay returns immediately with the context's state.
	if err := sleepCtx(context.Background(), 0); err != nil {
		t.Errorf("a zero delay on a live context should return nil, got %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := sleepCtx(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Errorf("sleepCtx on a cancelled context = %v, want context.Canceled", err)
	}
}

func TestCreateRecordValidateUsesTheWallClock(t *testing.T) {
	rec := validRecord()
	rec.RecordDate = NewTime(time.Now().Add(-time.Hour))
	if err := rec.Validate(); err != nil {
		t.Errorf("a record dated an hour ago should validate: %v", err)
	}
	rec.RecordDate = NewTime(time.Now().Add(48 * time.Hour))
	if err := rec.Validate(); err == nil {
		t.Error("a record dated two days ahead should be rejected")
	}
}

func TestResp0ContentTypeToleratesAMissingHeaderSet(t *testing.T) {
	if got := resp0ContentType(ResponseMeta{}); got != "" {
		t.Errorf("= %q, want empty when no headers were captured", got)
	}
}

func TestLooksLikeBatchEnvelope(t *testing.T) {
	tests := map[string]bool{
		`{"results":[],"summary":{}}`:   true,
		`{"results":[]}`:                true,
		`{"summary":{"total":1}}`:       true,
		`{"error":"invalid parameter"}`: false,
		`{}`:                            false,
		`[]`:                            false,
		``:                              false,
		`not json`:                      false,
		`{"results"`:                    false,
	}
	for body, want := range tests {
		if got := looksLikeBatchEnvelope([]byte(body)); got != want {
			t.Errorf("looksLikeBatchEnvelope(%q) = %v, want %v", body, got, want)
		}
	}
}

func TestIsBatchStatus(t *testing.T) {
	tests := map[int]bool{200: true, 207: true, 400: true, 500: true, 201: false, 401: false, 403: false, 429: false, 503: false}
	for status, want := range tests {
		if got := isBatchStatus(status); got != want {
			t.Errorf("isBatchStatus(%d) = %v, want %v", status, got, want)
		}
	}
}

func TestExtractErrorMessage(t *testing.T) {
	tests := []struct {
		name, contentType, body, want string
	}{
		{"json error", "application/json", `{"error":"invalid parameter"}`, "invalid parameter"},
		{"json message", "application/json", `{"message":"something"}`, "something"},
		{"json error wins", "application/json", `{"error":"a","message":"b"}`, "a"},
		{"plain text", "text/plain", "Forbidden", "Forbidden"},
		{"json sniffed without a header", "", `{"error":"sniffed"}`, "sniffed"},
		{"empty", "application/json", "", ""},
		{"whitespace only", "application/json", "   \n ", ""},
		{"unparseable json", "application/json", `{broken`, "{broken"},
		{"json without a known key", "application/json", `{"detail":"x"}`, `{"detail":"x"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractErrorMessage(tc.contentType, []byte(tc.body)); got != tc.want {
				t.Errorf("= %q, want %q", got, tc.want)
			}
		})
	}
}
