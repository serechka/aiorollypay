package rollypay

import (
	"context"
	"errors"
	"net/url"
	"time"
)

// PaymentsService manages payments: creation, lookup, listing and aggregate
// statistics. Access it via Client.Payments.
type PaymentsService struct {
	client *Client
}

// CreatePaymentInput is the payload of Payments.Create. Amount and OrderID
// are required; PaymentCurrency defaults to RUB when empty.
type CreatePaymentInput struct {
	// Amount is the fiat amount as a string, e.g. "1500.00".
	Amount string `json:"amount"`
	// OrderID is the unique order ID in the merchant system: one order_id
	// produces exactly one payment.
	OrderID string `json:"order_id"`
	// PaymentCurrency defaults to RUB. EUR is only valid together with
	// PaymentMethodIntlCard.
	PaymentCurrency string `json:"payment_currency"`
	// PaymentMethod: sbp, card, intl_card or crypto. Empty lets the
	// customer choose on the payment form.
	PaymentMethod string `json:"payment_method,omitempty"`
	// TerminalID is optional when the API key is bound to a terminal.
	TerminalID string `json:"terminal_id,omitempty"`
	// Description shown to the customer.
	Description string `json:"description,omitempty"`
	// CustomerID identifies the payer, e.g. an email or internal ID.
	CustomerID string `json:"customer_id,omitempty"`
	// RedirectURL is the generic post-payment redirect.
	RedirectURL string `json:"redirect_url,omitempty"`
	// SuccessRedirectURL wins over RedirectURL on success.
	SuccessRedirectURL string `json:"success_redirect_url,omitempty"`
	// FailRedirectURL is used on expired or canceled payments.
	FailRedirectURL string `json:"fail_redirect_url,omitempty"`
	// Metadata is returned verbatim in webhooks.
	Metadata map[string]any `json:"metadata,omitempty"`
	// Test creates a sandbox payment (no real money moves).
	Test bool `json:"test,omitempty"`
}

// Create opens a new payment and returns it with PayURL to redirect the
// customer to. It validates the required fields before any network call.
func (s *PaymentsService) Create(ctx context.Context, in CreatePaymentInput) (*Payment, error) {
	if in.Amount == "" {
		return nil, errors.New("rollypay: amount is required")
	}
	if in.OrderID == "" {
		return nil, errors.New("rollypay: order_id is required")
	}
	if in.PaymentCurrency == "" {
		in.PaymentCurrency = CurrencyRUB
	}
	var out Payment
	if err := s.client.post(ctx, "payments", in, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get fetches one payment by ID.
func (s *PaymentsService) Get(ctx context.Context, paymentID string) (*Payment, error) {
	if paymentID == "" {
		return nil, errors.New("rollypay: payment_id is required")
	}
	var out Payment
	if err := s.client.get(ctx, "payments/"+url.PathEscape(paymentID), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PaymentsFilter narrows Payments.List and Payments.Stats. Zero fields are
// omitted from the query; Limit and Page fall back to the API defaults
// (20 and 1) when the filter is nil or zero.
type PaymentsFilter struct {
	Limit      int
	Page       int
	AccountID  string
	TerminalID string
	OrderID    string
	Status     string
	DateFrom   time.Time
	DateTo     time.Time
	AmountFrom string
	AmountTo   string
	Search     string
	// Environment selects payments: production by default,
	// EnvironmentSandbox for test payments, EnvironmentAll for both.
	Environment string
}

func (f *PaymentsFilter) apply(q url.Values) {
	limit, page := f.Limit, f.Page
	if limit <= 0 {
		limit = 20
	}
	if page <= 0 {
		page = 1
	}
	q.Set("limit", itoa(limit))
	q.Set("page", itoa(page))
	setIfNotEmpty(q, "account_id", f.AccountID)
	setIfNotEmpty(q, "terminal_id", f.TerminalID)
	setIfNotEmpty(q, "order_id", f.OrderID)
	setIfNotEmpty(q, "status", f.Status)
	if !f.DateFrom.IsZero() {
		q.Set("from", f.DateFrom.UTC().Format(time.RFC3339))
	}
	if !f.DateTo.IsZero() {
		q.Set("to", f.DateTo.UTC().Format(time.RFC3339))
	}
	setIfNotEmpty(q, "amount_from", f.AmountFrom)
	setIfNotEmpty(q, "amount_to", f.AmountTo)
	setIfNotEmpty(q, "search", f.Search)
	setIfNotEmpty(q, "environment", f.Environment)
}

// List returns a page of payments matching the filter. A nil filter means
// defaults (20 per page, page 1, production only).
func (s *PaymentsService) List(ctx context.Context, f *PaymentsFilter) (*PaymentList, error) {
	if f == nil {
		f = &PaymentsFilter{}
	}
	q := url.Values{}
	f.apply(q)
	var out PaymentList
	if err := s.client.getQuery(ctx, "payments", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Stats returns aggregate payment statistics for the filter; limit and page
// do not apply here.
func (s *PaymentsService) Stats(ctx context.Context, f *PaymentsFilter) (map[string]any, error) {
	q := url.Values{}
	if f != nil {
		f.apply(q)
		q.Del("limit")
		q.Del("page")
	}
	var out map[string]any
	if err := s.client.getQuery(ctx, "payments/stats", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}
