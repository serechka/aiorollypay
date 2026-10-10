// Package rollypay is a fast, zero-dependency Go client for the RollyPay
// payment API (https://docs.rollypay.io).
//
// A Client is safe for concurrent use by multiple goroutines; every method
// takes a context and runs synchronously; compose concurrency with
// goroutines as usual:
//
//	client, err := rollypay.New(apiKey)
//	if err != nil { ... }
//
//	payment, err := client.Payments.Create(ctx, rollypay.CreatePaymentInput{
//		Amount:         "1500.00",
//		OrderID:        "order_12345",
//		PaymentMethod:  rollypay.PaymentMethodSBP,
//		Description:    "Оплата заказа #12345",
//		SuccessRedirectURL: "https://myshop.com/success",
//	})
//	if rollypay.IsRateLimited(err) { ... }
//
// Webhook signature verification lives in the webhooks subpackage.
package rollypay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/serechka/aiorollypay/internal/uuid"
)

const (
	// DefaultBaseURL is the public RollyPay API endpoint.
	DefaultBaseURL = "https://rollypay.io/api/v1"
	// DefaultTimeout is applied to every attempt whose context has no deadline.
	DefaultTimeout = 30 * time.Second
	// Version is the SDK version reported in the User-Agent header.
	Version = "0.1.0"

	// maxResponseBody caps how many bytes are read from a response before
	// decoding, guarding against runaway payloads.
	maxResponseBody = 10 << 20
)

const (
	headerAPIKey         = "X-API-Key"
	headerNonce          = "X-Nonce"
	headerTestMode       = "X-Test-Mode"
	headerIdempotencyKey = "Idempotency-Key"
	headerRetryAfter     = "Retry-After"
	userAgent            = "aiorollypay-go/" + Version
)

// Client is a RollyPay API client. It is thread-safe; create it once with New
// and share it. All request methods accept a context for cancellation and
// deadlines.
type Client struct {
	apiKey      string
	baseURL     string
	httpClient  *http.Client
	timeout     time.Duration
	testMode    bool
	maxAttempts int
	backoff     time.Duration

	Payments      *PaymentsService
	Terminals     *TerminalsService
	Stats         *StatsService
	Payouts       *PayoutsService
	Subscriptions *SubscriptionsService
	Misc          *MiscService
}

// New creates a Client anchored at apiKey. Options adjust the base URL,
// HTTP transport, timeouts, sandbox mode and retry behaviour.
func New(apiKey string, opts ...Option) (*Client, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("rollypay: api key is required")
	}
	c := &Client{
		apiKey:      apiKey,
		baseURL:     DefaultBaseURL,
		httpClient:  defaultHTTPClient(),
		timeout:     DefaultTimeout,
		maxAttempts: defaultMaxAttempts,
		backoff:     defaultBackoff,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	c.baseURL = strings.TrimRight(c.baseURL, "/")

	c.Payments = &PaymentsService{client: c}
	c.Terminals = &TerminalsService{client: c}
	c.Stats = &StatsService{client: c}
	c.Payouts = &PayoutsService{client: c}
	c.Subscriptions = &SubscriptionsService{client: c}
	c.Misc = &MiscService{client: c}
	return c, nil
}

// defaultHTTPClient returns a transport tuned for many concurrent calls to a
// single host: keep-alive pool sized for parallelism and HTTP/2 enabled.
func defaultHTTPClient() *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = 100
	t.MaxIdleConnsPerHost = 100
	t.ForceAttemptHTTP2 = true
	return &http.Client{Transport: t}
}

// requestOptions carries the per-call payload through do and its wrappers.
type requestOptions struct {
	query          url.Values
	body           any
	out            any
	idempotencyKey string
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	return c.do(ctx, http.MethodGet, path, requestOptions{out: out})
}

func (c *Client) getQuery(ctx context.Context, path string, query url.Values, out any) error {
	return c.do(ctx, http.MethodGet, path, requestOptions{query: query, out: out})
}

func (c *Client) post(ctx context.Context, path string, body, out any) error {
	return c.do(ctx, http.MethodPost, path, requestOptions{body: body, out: out})
}

func (c *Client) postIdempotent(ctx context.Context, path string, body, out any, idempotencyKey string) error {
	return c.do(ctx, http.MethodPost, path, requestOptions{body: body, out: out, idempotencyKey: idempotencyKey})
}

func (c *Client) put(ctx context.Context, path string, body, out any) error {
	return c.do(ctx, http.MethodPut, path, requestOptions{body: body, out: out})
}

// do executes one API call: it serializes the body once, then runs attempts
// with a fresh nonce each, retrying idempotent GETs on 429/5xx and transport
// errors.
func (c *Client) do(ctx context.Context, method, path string, opts requestOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}
	endpoint := c.baseURL + "/" + strings.TrimLeft(path, "/")
	if len(opts.query) > 0 {
		endpoint += "?" + opts.query.Encode()
	}

	var bodyBytes []byte
	if opts.body != nil {
		b, err := json.Marshal(opts.body)
		if err != nil {
			return &Error{Message: "marshal request: " + err.Error()}
		}
		bodyBytes = b
	}

	for attempt := 1; ; attempt++ {
		apiErr := c.attempt(ctx, method, endpoint, bodyBytes, opts)
		if apiErr == nil {
			return nil
		}
		if !c.retryable(method, apiErr) || attempt >= c.maxAttempts {
			return apiErr
		}
		if err := sleepCtx(ctx, c.retryDelay(attempt, apiErr)); err != nil {
			return err
		}
	}
}

// attempt performs a single HTTP exchange and decodes a success response.
func (c *Client) attempt(ctx context.Context, method, endpoint string, body []byte, opts requestOptions) *Error {
	attemptCtx, cancel := c.attemptContext(ctx)
	defer cancel()

	req, err := http.NewRequestWithContext(attemptCtx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return &Error{Message: "build request: " + err.Error()}
	}
	req.Header.Set(headerAPIKey, c.apiKey)
	req.Header.Set(headerNonce, uuid.NewV4())
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.testMode {
		req.Header.Set(headerTestMode, "true")
	}
	if opts.idempotencyKey != "" {
		req.Header.Set(headerIdempotencyKey, opts.idempotencyKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &Error{Message: "do request: " + err.Error()}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return &Error{StatusCode: resp.StatusCode, Message: "read response: " + err.Error()}
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return newAPIError(resp.StatusCode, respBody, resp.Header.Get(headerRetryAfter))
	}
	if opts.out != nil && len(bytes.TrimSpace(respBody)) > 0 {
		if err := json.Unmarshal(respBody, opts.out); err != nil {
			return &Error{StatusCode: resp.StatusCode, Message: "decode response: " + err.Error(), Body: respBody}
		}
	}
	return nil
}

// attemptContext applies the configured per-attempt timeout unless the caller
// already set a deadline on the context.
func (c *Client) attemptContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.timeout <= 0 {
		return ctx, func() {}
	}
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, c.timeout)
}

// retryable reports whether a failed request may be attempted again: only
// idempotent GETs, only transport failures, 429 or 5xx.
func (c *Client) retryable(method string, apiErr *Error) bool {
	if method != http.MethodGet {
		return false
	}
	return apiErr.StatusCode == 0 || apiErr.StatusCode == http.StatusTooManyRequests || apiErr.StatusCode >= 500
}

// retryDelay picks the wait before the next attempt: the server's
// Retry-After hint when present, otherwise exponential backoff.
func (c *Client) retryDelay(failedAttempt int, apiErr *Error) time.Duration {
	if apiErr.RetryAfter > 0 {
		return apiErr.RetryAfter
	}
	return c.backoff << (failedAttempt - 1)
}

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
