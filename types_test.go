package rollypay

import (
	"encoding/json"
	"testing"
	"time"
)

func TestUnmarshalListOrEnvelope(t *testing.T) {
	t.Run("envelope with total", func(t *testing.T) {
		var l PaymentList
		err := json.Unmarshal([]byte(`{"items":[{"payment_id":"a"},{"payment_id":"b"}],"total":99}`), &l)
		if err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if l.Total != 99 || len(l.Items) != 2 || l.Items[1].PaymentID != "b" {
			t.Errorf("list = %+v", l)
		}
	})
	t.Run("envelope without total", func(t *testing.T) {
		var l PayoutList
		err := json.Unmarshal([]byte(`{"items":[{"id":"a"}]}`), &l)
		if err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if l.Total != 1 {
			t.Errorf("total = %d, want item count", l.Total)
		}
	})
	t.Run("bare array", func(t *testing.T) {
		var l SubscriptionList
		err := json.Unmarshal([]byte(`[{"id":"s1"},{"id":"s2"}]`), &l)
		if err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if l.Total != 2 || l.Items[0].ID != "s1" {
			t.Errorf("list = %+v", l)
		}
	})
	t.Run("invalid payload", func(t *testing.T) {
		var l PaymentList
		if err := json.Unmarshal([]byte(`"not a list"`), &l); err == nil {
			t.Error("expected error")
		}
	})
	t.Run("jsonLen default branch", func(t *testing.T) {
		if got := jsonLen(struct{}{}); got != 0 {
			t.Errorf("jsonLen(unknown) = %d, want 0", got)
		}
	})
}

func TestPaymentFieldMapping(t *testing.T) {
	raw := `{
		"payment_id": "pay_1",
		"order_id": "order_1",
		"status": "processing",
		"token": "tok_1",
		"pay_url": "https://pay.rollypay.io/pay/tok_1",
		"amount": "1500.00",
		"payment_currency": "RUB",
		"qr_content": "ST00012",
		"h2h_enabled": true,
		"qr_activated": true,
		"metadata": {"ref": "INV-1"},
		"created_at": "2026-02-23T12:00:00Z",
		"expires_at": "2026-02-23T12:30:00Z"
	}`
	var p Payment
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.Status != PaymentStatusProcessing || p.QRContent != "ST00012" || !p.H2HEnabled || !p.QRActivated {
		t.Errorf("payment = %+v", p)
	}
	if p.Metadata["ref"] != "INV-1" {
		t.Errorf("metadata = %v", p.Metadata)
	}
	if p.CreatedAt.IsZero() || p.ExpiresAt.IsZero() {
		t.Error("timestamps must be parsed")
	}
	if p.Sandbox() {
		t.Error("Sandbox() must be false when test is absent")
	}
	falseFlag := false
	p.Test = &falseFlag
	if p.Sandbox() {
		t.Error("Sandbox() must be false when test=false")
	}
	trueFlag := true
	p.Test = &trueFlag
	if !p.Sandbox() {
		t.Error("Sandbox() must be true when test=true")
	}
}

func TestSubscriptionConstants(t *testing.T) {
	cases := map[string]struct {
		got, want string
	}{
		"state new":             {SubscriptionStateNew, "new"},
		"state active":          {SubscriptionStateActive, "active"},
		"state stop":            {SubscriptionStateStop, "stop"},
		"billing consent":       {BillingStatusConsentPending, "consent_pending"},
		"billing enabled":       {BillingStatusEnabled, "enabled"},
		"billing review":        {BillingStatusReview, "review"},
		"charge payed spelling": {ChargeStatusPayed, "payed"},
		"payout pending":        {PayoutStatusPending, "pending"},
		"payout completed":      {PayoutStatusCompleted, "completed"},
		"payment method sbp":    {PaymentMethodSBP, "sbp"},
		"payment method crypto": {PaymentMethodCrypto, "crypto"},
		"currency EUR":          {CurrencyEUR, "EUR"},
		"fee payer merchant":    {FeePayerMerchant, "merchant"},
		"conversion instant":    {ConversionInstant, "instant"},
		"environment all":       {EnvironmentAll, "all"},
	}
	for name, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", name, c.got, c.want)
		}
	}
}

func TestTimeParsingInPayout(t *testing.T) {
	raw := `{"id":"po_1","created_at":"2026-02-23T14:00:00Z","updated_at":"2026-02-23T15:30:00Z"}`
	var p Payout
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := time.Date(2026, 2, 23, 14, 0, 0, 0, time.UTC)
	if !p.CreatedAt.Equal(want) {
		t.Errorf("created_at = %v, want %v", p.CreatedAt, want)
	}
	if p.UpdatedAt.Before(p.CreatedAt) {
		t.Error("updated_at before created_at")
	}
}
