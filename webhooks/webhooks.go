// Package webhooks verifies and parses RollyPay webhook callbacks
// (docs.rollypay.io/api/callbacks).
//
// RollyPay signs every callback with the terminal's signing_secret:
// X-Signature is hex-encoded HMAC-SHA256 over "timestamp.body", where
// timestamp comes from the X-Timestamp header. Verification is mandatory
// before trusting a payload.
//
// The easiest integration is Handler:
//
//	http.Handle("/webhooks/rollypay", webhooks.Handler(signingSecret,
//		func(ctx context.Context, e *webhooks.Event) error {
//			log.Printf("event %s payment %s", e.EventType, e.PaymentID)
//			return nil
//		}))
//
// For full control over the raw exchange, use VerifySignature and ParseEvent
// directly. Handle events idempotently: deliveries are retried with backoff
// unless your endpoint answers 2xx within 10 seconds, and statuses are not
// permanently final (an expired payment can still become paid).
package webhooks

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// Webhook headers defined by the RollyPay API.
const (
	HeaderSignature = "X-Signature" // hex HMAC-SHA256 of "timestamp.body"
	HeaderTimestamp = "X-Timestamp" // unix timestamp of sending
	HeaderTestMode  = "X-Test-Mode" // "true" for sandbox callbacks
)

// Webhook event types (docs.rollypay.io/api/callbacks).
const (
	EventPaymentCreated         = "payment.created"
	EventPaymentPaid            = "payment.paid"
	EventPaymentCanceled        = "payment.canceled"
	EventPaymentExpired         = "payment.expired"
	EventPaymentChargeback      = "payment.chargeback"
	EventPaymentRefunded        = "payment.refunded"
	EventPayoutCompleted        = "payout.completed"
	EventRefundRequestCompleted = "refund_request.completed"
)

// maxBodySize caps how many callback bytes Handler will read.
const maxBodySize = 1 << 20

// Event is a decoded RollyPay webhook payload. Amount is a decimal string
// exactly as sent; Metadata appears only when it was set at payment creation.
type Event struct {
	EventType string         `json:"event_type"`
	PaymentID string         `json:"payment_id,omitempty"`
	OrderID   string         `json:"order_id,omitempty"`
	Status    string         `json:"status,omitempty"`
	Amount    string         `json:"amount,omitempty"`
	Currency  string         `json:"currency,omitempty"`
	Test      bool           `json:"test,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// Sandbox reports whether the event originated from a test payment.
func (e *Event) Sandbox() bool { return e.Test }

// Sign computes the expected X-Signature value for a callback:
// hex(HMAC-SHA256(secret, timestamp + "." + body)).
func Sign(signingSecret string, timestamp, body []byte) string {
	mac := hmac.New(sha256.New, []byte(signingSecret))
	mac.Write(timestamp)
	mac.Write([]byte("."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature reports whether signatureHex is the valid X-Signature for
// timestamp and body under signingSecret. The comparison is constant-time.
// An empty signature never verifies.
func VerifySignature(signingSecret string, timestamp, body []byte, signatureHex string) bool {
	if signatureHex == "" {
		return false
	}
	return hmac.Equal([]byte(Sign(signingSecret, timestamp, body)), []byte(signatureHex))
}

// ParseEvent decodes a callback body and requires the event_type field.
func ParseEvent(body []byte) (*Event, error) {
	var e Event
	if err := json.Unmarshal(body, &e); err != nil {
		return nil, fmt.Errorf("rollypay/webhooks: decode payload: %w", err)
	}
	if e.EventType == "" {
		return nil, errors.New("rollypay/webhooks: payload has no event_type")
	}
	return &e, nil
}

// Handler wraps fn into an http.Handler suitable for webhook endpoints:
//
//   - non-POST requests get 405;
//   - bodies larger than 1 MiB get 413;
//   - invalid or missing signatures get 403;
//   - malformed payloads get 400;
//   - fn errors produce 500, which makes RollyPay retry the delivery;
//   - success answers 200.
func Handler(signingSecret string, fn func(ctx context.Context, e *Event) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodySize))
		if err != nil {
			http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
			return
		}
		timestamp := r.Header.Get(HeaderTimestamp)
		signature := r.Header.Get(HeaderSignature)
		if !VerifySignature(signingSecret, []byte(timestamp), body, signature) {
			http.Error(w, "Invalid signature", http.StatusForbidden)
			return
		}
		event, err := ParseEvent(body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := fn(r.Context(), event); err != nil {
			http.Error(w, "handler failed", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
}
