package wallet

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ErrorKind classifies a failure. It maps 1:1 onto the sentinels below so
// callers can either switch on the kind or use errors.Is.
type ErrorKind string

const (
	// KindValidation is HTTP 400 — the server rejected a parameter or body.
	KindValidation ErrorKind = "validation"
	// KindUnauthorized is HTTP 401 — missing, invalid or expired token.
	KindUnauthorized ErrorKind = "unauthorized"
	// KindForbidden is HTTP 403 — the token belongs to the other app
	// (a Board token used on /wallet, or vice versa). Re-authenticating
	// against the same app does not help. The body is text/plain.
	KindForbidden ErrorKind = "forbidden"
	// KindNotFound is HTTP 404 — e.g. DELETE /v1/api/orders on Wallet.
	KindNotFound ErrorKind = "not_found"
	// KindRateLimited is HTTP 429. RetryAfter carries the Retry-After header.
	KindRateLimited ErrorKind = "rate_limited"
	// KindServer is HTTP 5xx on a non-batch endpoint.
	KindServer ErrorKind = "server"
	// KindUnexpectedStatus is any other status the spec does not document.
	KindUnexpectedStatus ErrorKind = "unexpected_status"
	// KindDecode means the response arrived but could not be decoded.
	KindDecode ErrorKind = "decode"
	// KindTransport means the request never produced an HTTP response.
	KindTransport ErrorKind = "transport"
)

// Sentinels for errors.Is.
var (
	ErrValidation       = errors.New("wallet: validation error (400)")
	ErrUnauthorized     = errors.New("wallet: unauthorized (401)")
	ErrForbidden        = errors.New("wallet: forbidden (403)")
	ErrNotFound         = errors.New("wallet: not found (404)")
	ErrRateLimited      = errors.New("wallet: rate limit exceeded (429)")
	ErrServer           = errors.New("wallet: server error (5xx)")
	ErrUnexpectedStatus = errors.New("wallet: unexpected status")
	ErrDecode           = errors.New("wallet: response decode error")
	ErrTransport        = errors.New("wallet: transport error")

	// ErrInvalidRequest is returned before anything is sent, when the SDK's
	// own validation rejects the arguments.
	ErrInvalidRequest = errors.New("wallet: invalid request")
)

var kindSentinels = map[ErrorKind]error{
	KindValidation:       ErrValidation,
	KindUnauthorized:     ErrUnauthorized,
	KindForbidden:        ErrForbidden,
	KindNotFound:         ErrNotFound,
	KindRateLimited:      ErrRateLimited,
	KindServer:           ErrServer,
	KindUnexpectedStatus: ErrUnexpectedStatus,
	KindDecode:           ErrDecode,
	KindTransport:        ErrTransport,
}

// Error is the single error type for everything that happens on the wire.
type Error struct {
	// Op is the OpenAPI operationId that failed, e.g. "getRecords".
	Op string
	// Kind classifies the failure.
	Kind ErrorKind
	// StatusCode is the HTTP status, or 0 for transport failures.
	StatusCode int
	// Message is the server's "error" field, the text/plain body, or a
	// locally generated description.
	Message string
	// RawBody is the response body verbatim, truncated to 8 KiB.
	RawBody string
	// RetryAfter is the parsed Retry-After header (429 and some 503s).
	RetryAfter time.Duration
	// Meta carries the response headers the API documents.
	Meta ResponseMeta
	// Err is the wrapped cause for transport and decode failures.
	Err error
}

func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString("wallet: ")
	if e.Op != "" {
		b.WriteString(e.Op)
		b.WriteString(": ")
	}
	if e.StatusCode != 0 {
		fmt.Fprintf(&b, "%d %s: ", e.StatusCode, http.StatusText(e.StatusCode))
	}
	if e.Message != "" {
		b.WriteString(e.Message)
	} else {
		b.WriteString(string(e.Kind))
	}
	if e.Kind == KindRateLimited && e.RetryAfter > 0 {
		fmt.Fprintf(&b, " (retry after %s)", e.RetryAfter)
	}
	if e.Err != nil {
		b.WriteString(": ")
		b.WriteString(e.Err.Error())
	}
	return b.String()
}

// Unwrap exposes the cause of transport and decode failures.
func (e *Error) Unwrap() error { return e.Err }

// Is lets errors.Is match both the kind sentinels and the wrapped cause.
func (e *Error) Is(target error) bool {
	if s, ok := kindSentinels[e.Kind]; ok && target == s {
		return true
	}
	return false
}

// Retryable reports whether retrying the identical request could plausibly
// succeed. It is advisory: the client's retry policy also weighs the HTTP
// method, because retrying a non-idempotent write can duplicate data.
func (e *Error) Retryable() bool {
	switch e.Kind {
	case KindRateLimited, KindServer, KindTransport:
		return true
	default:
		return false
	}
}

// InvalidRequestError is one client-side validation failure.
type InvalidRequestError struct {
	// Field is a dotted path into the request, e.g. "records[2].amount.value".
	Field string
	// Reason explains the constraint that was violated.
	Reason string
}

func (e *InvalidRequestError) Error() string {
	if e.Field == "" {
		return "wallet: invalid request: " + e.Reason
	}
	return "wallet: invalid request: " + e.Field + ": " + e.Reason
}

// Is makes every validation failure match ErrInvalidRequest.
func (e *InvalidRequestError) Is(target error) bool { return target == ErrInvalidRequest }

// InvalidRequestErrors collects every validation failure found in one
// request, so a caller fixing a 50-record batch sees all the problems at once
// instead of one per round trip.
type InvalidRequestErrors []*InvalidRequestError

func (e InvalidRequestErrors) Error() string {
	if len(e) == 1 {
		return e[0].Error()
	}
	parts := make([]string, 0, len(e))
	for _, v := range e {
		if v.Field == "" {
			parts = append(parts, v.Reason)
			continue
		}
		parts = append(parts, v.Field+": "+v.Reason)
	}
	return fmt.Sprintf("wallet: invalid request: %d problems: %s", len(e), strings.Join(parts, "; "))
}

// Is makes the collection match ErrInvalidRequest too.
func (e InvalidRequestErrors) Is(target error) bool { return target == ErrInvalidRequest }

// Unwrap exposes the individual failures to errors.As.
func (e InvalidRequestErrors) Unwrap() []error {
	out := make([]error, len(e))
	for i, v := range e {
		out[i] = v
	}
	return out
}

// errorf builds a single validation failure.
func errorf(field, format string, args ...any) *InvalidRequestError {
	return &InvalidRequestError{Field: field, Reason: fmt.Sprintf(format, args...)}
}

// problems accumulates validation failures and collapses to nil when empty,
// so callers can write `return p.err()` unconditionally.
type problems struct {
	list InvalidRequestErrors
}

func (p *problems) add(field, format string, args ...any) {
	p.list = append(p.list, errorf(field, format, args...))
}

func (p *problems) addErr(err error) {
	if err == nil {
		return
	}
	var one *InvalidRequestError
	var many InvalidRequestErrors
	switch {
	case errors.As(err, &many):
		p.list = append(p.list, many...)
	case errors.As(err, &one):
		p.list = append(p.list, one)
	default:
		p.list = append(p.list, &InvalidRequestError{Reason: err.Error()})
	}
}

// prefixed folds another validatable's failures in under a path prefix.
func (p *problems) prefixed(prefix string, err error) {
	if err == nil {
		return
	}
	var many InvalidRequestErrors
	var one *InvalidRequestError
	switch {
	case errors.As(err, &many):
		for _, v := range many {
			p.list = append(p.list, &InvalidRequestError{Field: joinPath(prefix, v.Field), Reason: v.Reason})
		}
	case errors.As(err, &one):
		p.list = append(p.list, &InvalidRequestError{Field: joinPath(prefix, one.Field), Reason: one.Reason})
	default:
		p.list = append(p.list, &InvalidRequestError{Field: prefix, Reason: err.Error()})
	}
}

func joinPath(prefix, field string) string {
	switch {
	case prefix == "":
		return field
	case field == "":
		return prefix
	case strings.HasPrefix(field, "["):
		return prefix + field
	default:
		return prefix + "." + field
	}
}

func (p *problems) err() error {
	if len(p.list) == 0 {
		return nil
	}
	return p.list
}
