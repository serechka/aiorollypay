package rollypay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// newTestClient returns a client pointed at a local test server, with fast
// retries so tests stay snappy.
func newTestClient(t *testing.T, h http.Handler) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := New("rpk_test_key",
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
		WithRetry(3, time.Millisecond),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c, srv
}

func readAll(t *testing.T, r io.Reader) []byte {
	t.Helper()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return b
}

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestNewRequiresAPIKey(t *testing.T) {
	for _, key := range []string{"", "   "} {
		if _, err := New(key); err == nil {
			t.Errorf("New(%q) should reject an empty api key", key)
		}
	}
}

func TestNewDefaults(t *testing.T) {
	c, err := New("key")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.baseURL != DefaultBaseURL {
		t.Errorf("baseURL = %q, want %q", c.baseURL, DefaultBaseURL)
	}
	if c.timeout != DefaultTimeout {
		t.Errorf("timeout = %v, want %v", c.timeout, DefaultTimeout)
	}
	if c.maxAttempts != defaultMaxAttempts {
		t.Errorf("maxAttempts = %d, want %d", c.maxAttempts, defaultMaxAttempts)
	}
	if c.Payments == nil || c.Terminals == nil || c.Stats == nil || c.Payouts == nil || c.Subscriptions == nil || c.Misc == nil {
		t.Error("New must initialize all services")
	}
}

func TestBaseURLTrailingSlashTrimmed(t *testing.T) {
	c, err := New("key", WithBaseURL("http://example.com/api/v1/"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.baseURL != "http://example.com/api/v1" {
		t.Errorf("baseURL = %q, want trailing slash trimmed", c.baseURL)
	}
}

func TestOptionsIgnoreZeroValues(t *testing.T) {
	c, err := New("key",
		WithBaseURL(""),
		WithHTTPClient(nil),
		WithRetry(0, 0),
		nil,
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.baseURL != DefaultBaseURL {
		t.Errorf("WithBaseURL(\"\") changed baseURL to %q", c.baseURL)
	}
	if c.maxAttempts != 1 {
		t.Errorf("WithRetry(0) → maxAttempts = %d, want 1", c.maxAttempts)
	}
}

func TestRequestHeadersAndAuth(t *testing.T) {
	var got http.Header
	var path string
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		path = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))

	err := c.get(context.Background(), "me", nil)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if path != "/me" {
		t.Errorf("path = %q, want /me", path)
	}
	if k := got.Get("X-API-Key"); k != "rpk_test_key" {
		t.Errorf("X-API-Key = %q", k)
	}
	if !uuidPattern.MatchString(got.Get("X-Nonce")) {
		t.Errorf("X-Nonce = %q, want UUID v4", got.Get("X-Nonce"))
	}
	if ua := got.Get("User-Agent"); !strings.Contains(ua, "aiorollypay-go/") {
		t.Errorf("User-Agent = %q", ua)
	}
	if ct := got.Get("Content-Type"); ct != "" {
		t.Errorf("Content-Type = %q on a bodyless GET, want empty", ct)
	}
	if accept := got.Get("Accept"); accept != "application/json" {
		t.Errorf("Accept = %q", accept)
	}
}

func TestNonceUniquePerRequest(t *testing.T) {
	var mu sync.Mutex
	seen := make(map[string]struct{})
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen[r.Header.Get("X-Nonce")] = struct{}{}
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))

	for i := 0; i < 5; i++ {
		if err := c.get(context.Background(), "me", nil); err != nil {
			t.Fatalf("get #%d: %v", i, err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 5 {
		t.Errorf("got %d distinct nonces across 5 requests, want 5", len(seen))
	}
}

func TestTestModeHeader(t *testing.T) {
	var testMode []string
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testMode = r.Header.Values("X-Test-Mode")
		w.WriteHeader(http.StatusNoContent)
	}))
	c.testMode = true

	if err := c.get(context.Background(), "me", nil); err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(testMode) != 1 || testMode[0] != "true" {
		t.Errorf("X-Test-Mode = %v, want [true]", testMode)
	}
}

func TestErrorMapping(t *testing.T) {
	tests := []struct {
		status int
		check  func(error) bool
	}{
		{400, IsBadRequest},
		{401, IsAuthentication},
		{403, IsForbidden},
		{404, IsNotFound},
		{409, IsConflict},
		{429, IsRateLimited},
		{500, IsServer},
		{503, IsServer},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("status %d", tt.status), func(t *testing.T) {
			c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(`{"error":"boom"}`))
			}))

			err := c.get(context.Background(), "me", nil)
			if err == nil {
				t.Fatal("expected error")
			}
			if !tt.check(err) {
				t.Errorf("predicate false for status %d, err = %v", tt.status, err)
			}
			var apiErr *Error
			if !errors.As(err, &apiErr) {
				t.Fatalf("errors.As(*Error) failed for %v", err)
			}
			if apiErr.Message != "boom" {
				t.Errorf("Message = %q, want boom", apiErr.Message)
			}
		})
	}
}

func TestRateLimitRetryAfterParsed(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "5")
		w.WriteHeader(http.StatusTooManyRequests)
	}))

	err := c.get(context.Background(), "me", nil)
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("want *Error, got %v", err)
	}
	if apiErr.RetryAfter != 5*time.Second {
		t.Errorf("RetryAfter = %v, want 5s", apiErr.RetryAfter)
	}
}

func TestGetRetriesThenSucceeds(t *testing.T) {
	var attempts int32
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&attempts, 1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))

	var out map[string]any
	if err := c.get(context.Background(), "me", &out); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got := atomic.LoadInt32(&attempts); got != 3 {
		t.Errorf("attempts = %d, want 3", got)
	}
}

func TestPostIsNeverRetried(t *testing.T) {
	var attempts int32
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))

	err := c.post(context.Background(), "payments", map[string]string{"x": "1"}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if got := atomic.LoadInt32(&attempts); got != 1 {
		t.Errorf("attempts = %d, want exactly 1 (no retry for POST)", got)
	}
}

func TestGet429NotRetriedBeyondMaxAttempts(t *testing.T) {
	var attempts int32
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))

	if err := c.get(context.Background(), "me", nil); !IsRateLimited(err) {
		t.Fatalf("want rate limit error, got %v", err)
	}
	if got := atomic.LoadInt32(&attempts); got != 3 {
		t.Errorf("attempts = %d, want 3", got)
	}
}

func TestTransportErrorIsRetriedAndWrapped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // nothing listens anymore → transport errors

	c, err := New("key", WithBaseURL(url), WithRetry(2, time.Millisecond), WithTimeout(2*time.Second))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = c.get(context.Background(), "me", nil)
	if !IsTransport(err) {
		t.Fatalf("want transport error, got %v", err)
	}
}

func TestContextCancellationAborts(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := c.get(ctx, "me", nil)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want DeadlineExceeded", err)
	}
}

func TestDecodesJSONResponse(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"rate":"93.89","updated_at":"2026-02-23T12:00:00Z"}`))
	}))

	var out Rate
	if err := c.get(context.Background(), "rate", &out); err != nil {
		t.Fatalf("get: %v", err)
	}
	if out.Rate != "93.89" {
		t.Errorf("Rate = %q", out.Rate)
	}
}

func TestDecodeFailureIsReported(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))

	err := c.get(context.Background(), "me", &struct{}{})
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 200 {
		t.Fatalf("want *Error with status 200, got %v", err)
	}
	if !strings.Contains(apiErr.Message, "decode response") {
		t.Errorf("Message = %q, want decode failure", apiErr.Message)
	}
}

func TestRequestBodyMarshalled(t *testing.T) {
	var body map[string]any
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.Unmarshal(readAll(t, r.Body), &body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	}))

	if err := c.post(context.Background(), "payments", map[string]any{"amount": "10.00"}, nil); err != nil {
		t.Fatalf("post: %v", err)
	}
	if body["amount"] != "10.00" {
		t.Errorf("body = %v", body)
	}
}

func TestIdempotencyKeyHeader(t *testing.T) {
	var got string
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Idempotency-Key")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	}))

	if err := c.postIdempotent(context.Background(), "subscriptions", map[string]any{}, nil, "customer-123-attempt-1"); err != nil {
		t.Fatalf("postIdempotent: %v", err)
	}
	if got != "customer-123-attempt-1" {
		t.Errorf("Idempotency-Key = %q", got)
	}
}

func TestQueryParamsEncoded(t *testing.T) {
	var rawQuery string
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusNoContent)
	}))

	q := map[string][]string{"status": {"paid"}, "limit": {"20"}}
	if err := c.getQuery(context.Background(), "payments", q, nil); err != nil {
		t.Fatalf("getQuery: %v", err)
	}
	if !strings.Contains(rawQuery, "status=paid") || !strings.Contains(rawQuery, "limit=20") {
		t.Errorf("rawQuery = %q", rawQuery)
	}
}

func TestConcurrentRequestsAreSafe(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	done := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func() { done <- c.get(context.Background(), "me", nil) }()
	}
	for i := 0; i < 8; i++ {
		if err := <-done; err != nil {
			t.Fatalf("concurrent get: %v", err)
		}
	}
}
