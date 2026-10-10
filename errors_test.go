package rollypay

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestErrorFormatting(t *testing.T) {
	tests := []struct {
		name string
		err  *Error
		want string
	}{
		{
			name: "transport error",
			err:  &Error{Message: "connection refused"},
			want: "rollypay: connection refused",
		},
		{
			name: "api error",
			err:  &Error{StatusCode: 404, Message: "payment not found"},
			want: "rollypay: API error 404: payment not found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewAPIErrorParsesBody(t *testing.T) {
	e := newAPIError(400, []byte(`{"error":"amount must be positive"}`), "")
	if e.Message != "amount must be positive" {
		t.Errorf("Message = %q, want API error text", e.Message)
	}
	if e.RetryAfter != 0 {
		t.Errorf("RetryAfter = %v, want 0", e.RetryAfter)
	}
}

func TestNewAPIErrorFallbacks(t *testing.T) {
	e := newAPIError(502, []byte("  "), "")
	if e.Message != "Bad Gateway" {
		t.Errorf("Message = %q, want status text", e.Message)
	}
	e = newAPIError(500, []byte(`internal boom`), "")
	if e.Message != "internal boom" {
		t.Errorf("Message = %q, want raw body", e.Message)
	}
}

func TestNewAPIErrorRetryAfter(t *testing.T) {
	e := newAPIError(429, []byte(`{"error":"too many requests"}`), "3")
	if e.RetryAfter != 3*time.Second {
		t.Errorf("RetryAfter = %v, want 3s", e.RetryAfter)
	}
}

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
	}{
		{"", 0},
		{"abc", 0},
		{"-5", 0},
		{"0", 0},
		{"7", 7 * time.Second},
		{" 2 ", 2 * time.Second},
	}
	for _, tt := range tests {
		if got := parseRetryAfter(tt.in); got != tt.want {
			t.Errorf("parseRetryAfter(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestErrorPredicates(t *testing.T) {
	wrap := func(e *Error) error { return fmt.Errorf("call failed: %w", e) }

	tests := []struct {
		name  string
		err   error
		check func(error) bool
		want  bool
	}{
		{"400 is bad request", wrap(&Error{StatusCode: 400}), IsBadRequest, true},
		{"401 is auth", wrap(&Error{StatusCode: 401}), IsAuthentication, true},
		{"403 is forbidden", wrap(&Error{StatusCode: 403}), IsForbidden, true},
		{"404 is not found", wrap(&Error{StatusCode: 404}), IsNotFound, true},
		{"409 is conflict", wrap(&Error{StatusCode: 409}), IsConflict, true},
		{"429 is rate limited", wrap(&Error{StatusCode: 429}), IsRateLimited, true},
		{"500 is server", wrap(&Error{StatusCode: 500}), IsServer, true},
		{"503 is server", wrap(&Error{StatusCode: 503}), IsServer, true},
		{"0 is transport", wrap(&Error{Message: "timeout"}), IsTransport, true},
		{"any Error is api error", wrap(&Error{StatusCode: 400}), IsAPIError, true},
		{"400 is not auth", wrap(&Error{StatusCode: 400}), IsAuthentication, false},
		{"404 is not server", wrap(&Error{StatusCode: 404}), IsServer, false},
		{"plain error is not api error", errors.New("boom"), IsAPIError, false},
		{"nil is not api error", nil, IsAPIError, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.check(tt.err); got != tt.want {
				t.Errorf("predicate = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrorMessageJSONRoundTrip(t *testing.T) {
	var v struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(`{"error":"x"}`), &v); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if v.Error != "x" {
		t.Errorf("v.Error = %q, want x", v.Error)
	}
}
