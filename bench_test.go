package rollypay

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/serechka/aiorollypay/webhooks"
)

// BenchmarkClientRoundTrip measures a full API exchange against a local test
// server: request build, headers with a fresh nonce, JSON decode. It upper-
// bounds the SDK overhead per call on a real network path.
func BenchmarkClientRoundTrip(b *testing.B) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"payment_id":"pay_1","status":"created","amount":"1500.00","pay_url":"https://pay.rollypay.io/pay/tok_1"}`))
	}))
	defer srv.Close()

	c, err := New("rpk_bench", WithBaseURL(srv.URL), WithHTTPClient(srv.Client()), WithRetry(1, 0))
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := c.Payments.Get(ctx, "pay_1"); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCreatePaymentInputMarshal isolates request serialization cost.
func BenchmarkCreatePaymentInputMarshal(b *testing.B) {
	in := CreatePaymentInput{
		Amount:             "1500.00",
		OrderID:            "order_12345",
		PaymentMethod:      PaymentMethodSBP,
		Description:        "Оплата заказа #12345",
		SuccessRedirectURL: "https://myshop.com/success",
		Metadata:           map[string]any{"internal_ref": "INV-2026-0042"},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := json.Marshal(in); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkWebhookVerify measures signature verification of a typical
// callback payload.
func BenchmarkWebhookVerify(b *testing.B) {
	const secret = "whsec_bench"
	const body = `{"event_type":"payment.paid","payment_id":"pay_a1b2c3d4-e5f6-7890-abcd-ef1234567890","order_id":"order_12345","status":"paid","amount":"1500.00","currency":"RUB","test":false}`
	sig := webhooks.Sign(secret, []byte("1740441600"), []byte(body))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !webhooks.VerifySignature(secret, []byte("1740441600"), []byte(body), sig) {
			b.Fatal("signature should verify")
		}
	}
}
