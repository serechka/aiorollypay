package rollypay

import (
	"net/http"
	"time"
)

const (
	defaultMaxAttempts = 3 // 1 try + 2 retries
	defaultBackoff     = 500 * time.Millisecond
)

// Option configures a Client returned by New.
type Option func(*Client)

// WithBaseURL overrides the default API base URL
// (DefaultBaseURL). A trailing slash is trimmed.
func WithBaseURL(u string) Option {
	return func(c *Client) {
		if u != "" {
			c.baseURL = u
		}
	}
}

// WithHTTPClient supplies a custom *http.Client, e.g. one with a proxy or
// custom dialer. A nil client is ignored.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		if hc != nil {
			c.httpClient = hc
		}
	}
}

// WithTimeout sets the per-attempt timeout applied to requests whose context
// has no deadline (DefaultTimeout when unset; d <= 0 disables the timeout).
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.timeout = d }
}

// WithTestMode makes every request run in RollyPay sandbox: the client sends
// the X-Test-Mode: true header. Individual calls can still opt in or out via
// the Test field of their input structs.
func WithTestMode(on bool) Option {
	return func(c *Client) { c.testMode = on }
}

// WithRetry configures automatic retries of idempotent GET requests that fail
// with 429 or 5xx responses or transport errors. maxAttempts includes the
// first try (values < 1 are treated as 1, which disables retries); backoff is
// the base delay, doubled on every subsequent attempt. A Retry-After header
// on 429 responses overrides the computed delay. POST, PUT and other
// non-idempotent methods are never retried automatically.
func WithRetry(maxAttempts int, backoff time.Duration) Option {
	return func(c *Client) {
		if maxAttempts < 1 {
			maxAttempts = 1
		}
		c.maxAttempts = maxAttempts
		if backoff > 0 {
			c.backoff = backoff
		}
	}
}
