package webhooks

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testSecret = "whsec_test_0123456789abcdef"

const testPayload = `{"event_type":"payment.paid","payment_id":"pay_a1b2c3d4","order_id":"order_12345","status":"paid","amount":"1500.00","currency":"RUB","test":false}`

// hmacSHA256 is an independent reference implementation used to cross-check Sign.
func hmacSHA256(key, msg []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(msg)
	return mac.Sum(nil)
}

func signRequest(t *testing.T, secret, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/webhooks/rollypay", strings.NewReader(body))
	req.Header.Set(HeaderTimestamp, "1740441600")
	req.Header.Set(HeaderSignature, Sign(secret, []byte("1740441600"), []byte(body)))
	return req
}

func TestSignMatchesReferenceAlgorithm(t *testing.T) {
	// Independent reimplementation of the documented algorithm:
	// hex(HMAC-SHA256(secret, timestamp + "." + body)).
	mac := hmacSHA256([]byte(testSecret), []byte("1740441600."+testPayload))
	want := fmt.Sprintf("%x", mac)
	if got := Sign(testSecret, []byte("1740441600"), []byte(testPayload)); got != want {
		t.Errorf("Sign() = %q, want %q", got, want)
	}
}

func TestVerifySignature(t *testing.T) {
	ts, body := []byte("1740441600"), []byte(testPayload)
	sig := Sign(testSecret, ts, body)

	if !VerifySignature(testSecret, ts, body, sig) {
		t.Error("valid signature rejected")
	}
	if VerifySignature("wrong-secret", ts, body, sig) {
		t.Error("wrong secret accepted")
	}
	if VerifySignature(testSecret, []byte("1740441601"), body, sig) {
		t.Error("tampered timestamp accepted")
	}
	if VerifySignature(testSecret, ts, []byte(testPayload+" "), sig) {
		t.Error("tampered body accepted")
	}
	if VerifySignature(testSecret, ts, body, "") {
		t.Error("empty signature accepted")
	}
	if VerifySignature(testSecret, ts, body, sig+"0") {
		t.Error("modified signature accepted")
	}
}

func TestParseEvent(t *testing.T) {
	e, err := ParseEvent([]byte(testPayload))
	if err != nil {
		t.Fatalf("ParseEvent: %v", err)
	}
	if e.EventType != EventPaymentPaid || e.PaymentID != "pay_a1b2c3d4" ||
		e.OrderID != "order_12345" || e.Status != "paid" ||
		e.Amount != "1500.00" || e.Currency != "RUB" || e.Sandbox() {
		t.Errorf("event = %+v", e)
	}
}

func TestParseEventExpired(t *testing.T) {
	body := `{"event_type":"payment.expired","payment_id":"pay_1","status":"expired","amount":"10.00","currency":"RUB","test":true,"metadata":{"ref":"X"}}`
	e, err := ParseEvent([]byte(body))
	if err != nil {
		t.Fatalf("ParseEvent: %v", err)
	}
	if e.EventType != EventPaymentExpired || e.Status != "expired" || !e.Sandbox() {
		t.Errorf("event = %+v", e)
	}
	if e.Metadata["ref"] != "X" {
		t.Errorf("metadata = %v", e.Metadata)
	}
}

func TestParseEventFailures(t *testing.T) {
	if _, err := ParseEvent([]byte(`not json`)); err == nil {
		t.Error("invalid JSON should fail")
	}
	if _, err := ParseEvent([]byte(`{"status":"paid"}`)); err == nil {
		t.Error("missing event_type should fail")
	}
}

func TestHandlerHappyPath(t *testing.T) {
	var received *Event
	h := Handler(testSecret, func(ctx context.Context, e *Event) error {
		received = e
		return nil
	})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, signRequest(t, testSecret, testPayload))

	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, body = %q", rec.Code, rec.Body.String())
	}
	if received == nil || received.PaymentID != "pay_a1b2c3d4" {
		t.Errorf("received = %+v", received)
	}
}

func TestHandlerRejectsBadSignature(t *testing.T) {
	h := Handler(testSecret, func(ctx context.Context, e *Event) error {
		t.Error("handler must not run on bad signature")
		return nil
	})

	req := signRequest(t, testSecret, testPayload)
	req.Header.Set(HeaderSignature, "deadbeef")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("code = %d, want 403", rec.Code)
	}

	req = signRequest(t, testSecret, testPayload)
	req.Header.Del(HeaderSignature)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("missing signature: code = %d, want 403", rec.Code)
	}
}

func TestHandlerRejectsWrongMethodAndBody(t *testing.T) {
	h := Handler(testSecret, func(ctx context.Context, e *Event) error { return nil })

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/webhooks", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET code = %d, want 405", rec.Code)
	}

	req := signRequest(t, testSecret, testPayload)
	req.Header.Set(HeaderTimestamp, "1")
	req.Header.Set(HeaderSignature, Sign(testSecret, []byte("1"), []byte(strings.Repeat("x", 2<<20))))
	big := httptest.NewRequest(http.MethodPost, "/webhooks", strings.NewReader(strings.Repeat("x", 2<<20)))
	big.Header = req.Header
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, big)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized code = %d, want 413", rec.Code)
	}

	req = signRequest(t, testSecret, "not json")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("bad json code = %d, want 400", rec.Code)
	}
}

func TestHandlerReturns500OnFnError(t *testing.T) {
	h := Handler(testSecret, func(ctx context.Context, e *Event) error {
		return errors.New("db down")
	})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, signRequest(t, testSecret, testPayload))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("code = %d, want 500", rec.Code)
	}
}
