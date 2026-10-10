package rollypay

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Error is the error type returned for every failed API call. Transport
// failures carry StatusCode 0; HTTP failures carry the response status code
// and the message produced by the API.
//
// Use errors.As to unwrap it:
//
//	var apiErr *rollypay.Error
//	if errors.As(err, &apiErr) { ... }
type Error struct {
	// StatusCode is the HTTP status of the response, or 0 when the request
	// failed on the transport level (DNS, TLS, timeout, connection reset).
	StatusCode int
	// Message is the human-readable error text: the API "error" field when
	// present, otherwise the raw body or the transport message.
	Message string
	// RetryAfter is parsed from the Retry-After header of 429 responses.
	// Zero when absent or unparseable.
	RetryAfter time.Duration
	// Body keeps the raw response body for debugging unexpected payloads.
	Body []byte
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.StatusCode == 0 {
		return "rollypay: " + e.Message
	}
	return fmt.Sprintf("rollypay: API error %d: %s", e.StatusCode, e.Message)
}

// newAPIError builds an *Error from a non-2xx response.
func newAPIError(statusCode int, body []byte, retryAfter string) *Error {
	e := &Error{StatusCode: statusCode, Body: body}

	var parsed struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil && parsed.Error != "" {
		e.Message = parsed.Error
	} else {
		e.Message = strings.TrimSpace(string(body))
	}
	if e.Message == "" {
		e.Message = http.StatusText(statusCode)
	}
	if statusCode == http.StatusTooManyRequests {
		e.RetryAfter = parseRetryAfter(retryAfter)
	}
	return e
}

// parseRetryAfter understands the seconds form of the Retry-After header.
func parseRetryAfter(v string) time.Duration {
	secs, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || secs < 0 {
		return 0
	}
	return time.Duration(secs) * time.Second
}

func statusCodeOf(err error) int {
	var e *Error
	if errors.As(err, &e) {
		return e.StatusCode
	}
	return 0
}

// IsAPIError reports whether err is any RollyPay error (API or transport).
func IsAPIError(err error) bool {
	var e *Error
	return errors.As(err, &e)
}

// IsTransport reports whether err is a transport-level failure (network,
// timeout) rather than an API response error.
func IsTransport(err error) bool { return statusCodeOf(err) == 0 && IsAPIError(err) }

// IsBadRequest reports whether the API rejected the input (HTTP 400).
func IsBadRequest(err error) bool { return statusCodeOf(err) == http.StatusBadRequest }

// IsAuthentication reports whether the API key or nonce was rejected (401).
func IsAuthentication(err error) bool { return statusCodeOf(err) == http.StatusUnauthorized }

// IsForbidden reports whether the key lacks permission (HTTP 403).
func IsForbidden(err error) bool { return statusCodeOf(err) == http.StatusForbidden }

// IsNotFound reports whether the resource does not exist (HTTP 404).
func IsNotFound(err error) bool { return statusCodeOf(err) == http.StatusNotFound }

// IsConflict reports whether the request conflicted with existing state,
// e.g. a changed idempotency key payload (HTTP 409).
func IsConflict(err error) bool { return statusCodeOf(err) == http.StatusConflict }

// IsRateLimited reports whether the request hit a rate limit (HTTP 429).
// The *Error RetryAfter field carries the server hint when present.
func IsRateLimited(err error) bool { return statusCodeOf(err) == http.StatusTooManyRequests }

// IsServer reports whether the failure is on the API side (HTTP 5xx).
func IsServer(err error) bool { return statusCodeOf(err) >= 500 }
