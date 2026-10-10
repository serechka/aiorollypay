package rollypay

import (
	"bytes"
	"encoding/json"
	"time"
)

// Currencies accepted by the payment creation endpoint.
const (
	CurrencyRUB = "RUB"
	CurrencyEUR = "EUR" // only valid with PaymentMethodIntlCard
)

// Payment method values for CreatePaymentInput.PaymentMethod.
const (
	PaymentMethodSBP          = "sbp"
	PaymentMethodCard         = "card"
	PaymentMethodIntlCard     = "intl_card"
	PaymentMethodCrypto       = "crypto"
	PaymentMethodSBPRecurrent = "sbp_recurrent"
)

// Payment lifecycle statuses (docs.rollypay.io/api/payments).
const (
	PaymentStatusCreated    = "created"
	PaymentStatusProcessing = "processing"
	PaymentStatusPaid       = "paid"
	PaymentStatusExpired    = "expired"
	PaymentStatusCanceled   = "canceled"
	PaymentStatusChargeback = "chargeback"
	PaymentStatusRefunded   = "refunded"
)

// Environment values for PaymentsFilter.Environment.
const (
	EnvironmentSandbox = "sandbox"
	EnvironmentAll     = "all"
)

// Terminal fee payer values for TerminalUpdate.FeePayer.
const (
	FeePayerClient   = "client"   // the customer pays the fee
	FeePayerMerchant = "merchant" // the merchant pays the fee
)

// Terminal conversion mode values for TerminalUpdate.ConversionMode.
const (
	ConversionInstant   = "instant"
	ConversionScheduled = "scheduled"
)

// Payout lifecycle statuses (docs.rollypay.io/api/payouts).
const (
	PayoutStatusCreated    = "created"
	PayoutStatusPending    = "pending"
	PayoutStatusProcessing = "processing"
	PayoutStatusCompleted  = "completed"
	PayoutStatusRejected   = "rejected"
)

// Subscription states (docs.rollypay.io/api/recurring).
const (
	SubscriptionStateNew    = "new"
	SubscriptionStateActive = "active"
	SubscriptionStateStop   = "stop"
)

// Subscription billing statuses.
const (
	BillingStatusConsentPending = "consent_pending"
	BillingStatusPending        = "pending"
	BillingStatusEnabled        = "enabled"
	BillingStatusReview         = "review"
	BillingStatusStopPending    = "stop_pending"
	BillingStatusStopped        = "stopped"
)

// Subscription charge statuses. Note: "payed" (charge) is intentionally
// different from "paid" (payment); that is how the API spells it.
const (
	ChargeStatusPayed = "payed"
	ChargeStatusFail  = "fail"
	ChargeStatusStop  = "stop"
)

// Payment is a RollyPay payment object: the creation response and the result
// of Get/List. Amount fields are strings exactly as the API returns them.
// Conversion fields (RateFinal, AmountUSDTNet, fees) appear after the payment
// is converted and may be temporarily absent.
type Payment struct {
	PaymentID          string         `json:"payment_id"`
	OrderID            string         `json:"order_id,omitempty"`
	Status             string         `json:"status,omitempty"`
	Token              string         `json:"token,omitempty"`
	PayURL             string         `json:"pay_url,omitempty"`
	Amount             string         `json:"amount,omitempty"`
	PaymentCurrency    string         `json:"payment_currency,omitempty"`
	PaymentMethod      string         `json:"payment_method,omitempty"`
	QRContent          string         `json:"qr_content,omitempty"`
	H2HEnabled         bool           `json:"h2h_enabled,omitempty"`
	QRActivated        bool           `json:"qr_activated,omitempty"`
	RateFinal          string         `json:"rate_final,omitempty"`
	AmountUSDTNet      string         `json:"amount_usdt_net,omitempty"`
	PlatformFeePercent string         `json:"platform_fee_percent,omitempty"`
	PlatformFeeUSDT    string         `json:"platform_fee_usdt,omitempty"`
	SubscriptionID     string         `json:"subscription_id,omitempty"`
	CustomerID         string         `json:"customer_id,omitempty"`
	Description        string         `json:"description,omitempty"`
	Test               *bool          `json:"test,omitempty"`
	Environment        string         `json:"environment,omitempty"`
	Metadata           map[string]any `json:"metadata,omitempty"`
	CreatedAt          time.Time      `json:"created_at,omitempty"`
	ExpiresAt          time.Time      `json:"expires_at,omitempty"`
	PaidAt             time.Time      `json:"paid_at,omitempty"`
}

// Sandbox reports whether the payment was created in test mode.
func (p Payment) Sandbox() bool { return p.Test != nil && *p.Test }

// PaymentList is the paginated result of Payments.List.
type PaymentList struct {
	Items []Payment `json:"items"`
	Total int       `json:"total"`
}

// UnmarshalJSON accepts both the documented {"items": [...], "total": n}
// envelope and a bare JSON array, so minor API shape changes stay harmless.
func (l *PaymentList) UnmarshalJSON(data []byte) error {
	var items []Payment
	if err := unmarshalListOrEnvelope(data, &items, &l.Total); err != nil {
		return err
	}
	l.Items = items
	return nil
}

// Terminal is a касса (cash register) with its settings. The same struct
// serves reads and, via TerminalUpdate, writes.
type Terminal struct {
	ID                   string `json:"id,omitempty"`
	Name                 string `json:"name,omitempty"`
	CallbackURL          string `json:"callback_url,omitempty"`
	SuccessRedirectURL   string `json:"success_redirect_url,omitempty"`
	FailRedirectURL      string `json:"fail_redirect_url,omitempty"`
	RedirectURL          string `json:"redirect_url,omitempty"`
	ShowPlatformBranding *bool  `json:"show_platform_branding,omitempty"`
	FeePayer             string `json:"fee_payer,omitempty"`
	ConversionMode       string `json:"conversion_mode,omitempty"`
	ConversionTime       string `json:"conversion_time,omitempty"`
}

// TerminalUpdate is the payload of Terminals.Update; only non-zero fields are
// sent, so callers change exactly what they set.
type TerminalUpdate struct {
	CallbackURL          string `json:"callback_url,omitempty"`
	SuccessRedirectURL   string `json:"success_redirect_url,omitempty"`
	FailRedirectURL      string `json:"fail_redirect_url,omitempty"`
	RedirectURL          string `json:"redirect_url,omitempty"`
	ShowPlatformBranding *bool  `json:"show_platform_branding,omitempty"`
	FeePayer             string `json:"fee_payer,omitempty"`
	ConversionMode       string `json:"conversion_mode,omitempty"`
	ConversionTime       string `json:"conversion_time,omitempty"`
}

// APIKeyRotation is the response of Terminals.RotateAPIKey: the new key
// replaces the old one immediately.
type APIKeyRotation struct {
	APIKey string `json:"api_key"`
}

// Provider is an entry of the provider list enabled for a terminal. The API
// does not publish a fixed schema for it, so it is kept as a raw object.
type Provider map[string]any

// Balance is a terminal's USDT balance.
type Balance struct {
	TerminalID    string `json:"terminal_id,omitempty"`
	AvailableUSDT string `json:"available_usdt,omitempty"`
	HoldUSDT      string `json:"hold_usdt,omitempty"`
}

// Rate is the current USDT/fiat exchange rate.
type Rate struct {
	Rate      string    `json:"rate"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// Payout is a USDT withdrawal request.
type Payout struct {
	ID             string    `json:"id"`
	TerminalID     string    `json:"terminal_id,omitempty"`
	AmountUSDT     string    `json:"amount_usdt,omitempty"`
	WalletAddress  string    `json:"wallet_address,omitempty"`
	Network        string    `json:"network,omitempty"`
	Status         string    `json:"status,omitempty"`
	IdempotencyKey string    `json:"idempotency_key,omitempty"`
	CreatedAt      time.Time `json:"created_at,omitempty"`
	UpdatedAt      time.Time `json:"updated_at,omitempty"`
}

// PayoutList is the paginated result of Payouts.List.
type PayoutList struct {
	Items []Payout `json:"items"`
	Total int      `json:"total"`
}

// UnmarshalJSON accepts both the envelope form and a bare JSON array.
func (l *PayoutList) UnmarshalJSON(data []byte) error {
	var items []Payout
	if err := unmarshalListOrEnvelope(data, &items, &l.Total); err != nil {
		return err
	}
	l.Items = items
	return nil
}

// SubscriptionPlan describes one recurring SBP tariff of a terminal. A fixed
// plan carries PayerAmountRUB; a cap-limited plan carries CapAmountRUB.
type SubscriptionPlan struct {
	ID             string `json:"id"`
	Interval       string `json:"interval,omitempty"`
	PayerAmountRUB string `json:"payer_amount_rub,omitempty"`
	CapAmountRUB   string `json:"cap_amount_rub,omitempty"`
	Active         *bool  `json:"active,omitempty"`
}

// Subscription is a recurring SBP payment. State and BillingStatus follow the
// API vocabulary; see the constants SubscriptionState* and BillingStatus*.
type Subscription struct {
	ID               string    `json:"id"`
	TerminalID       string    `json:"terminal_id,omitempty"`
	PlanID           string    `json:"plan_id,omitempty"`
	PayerID          string    `json:"payer_id,omitempty"`
	MerchantRef      string    `json:"merchant_subscription_ref,omitempty"`
	State            string    `json:"state,omitempty"`
	BillingStatus    string    `json:"billing_status,omitempty"`
	PayerAmountRUB   string    `json:"payer_amount_rub,omitempty"`
	CapAmountRUB     string    `json:"cap_amount_rub,omitempty"`
	Interval         string    `json:"interval,omitempty"`
	SuccessfulCycles int       `json:"successful_cycles,omitempty"`
	PayURL           string    `json:"pay_url,omitempty"`
	RedirectURL      string    `json:"redirect_url,omitempty"`
	NextChargeAt     time.Time `json:"next_charge_at,omitempty"`
	Description      string    `json:"description,omitempty"`
	CreatedAt        time.Time `json:"created_at,omitempty"`
}

// SubscriptionList is the paginated result of Subscriptions.List.
type SubscriptionList struct {
	Items []Subscription `json:"items"`
	Total int            `json:"total"`
}

// UnmarshalJSON accepts both the envelope form and a bare JSON array.
func (l *SubscriptionList) UnmarshalJSON(data []byte) error {
	var items []Subscription
	if err := unmarshalListOrEnvelope(data, &items, &l.Total); err != nil {
		return err
	}
	l.Items = items
	return nil
}

// SubscriptionCharge is one automatic charge of a subscription. Grant access
// only on the linked payment being paid, never on the charge alone.
type SubscriptionCharge struct {
	PaymentID         string    `json:"payment_id,omitempty"`
	CycleNumber       int       `json:"cycle_number,omitempty"`
	Status            string    `json:"status,omitempty"`
	ValidationStatus  string    `json:"validation_status,omitempty"`
	ValidationReason  string    `json:"validation_reason,omitempty"`
	AmountRUB         string    `json:"amount_rub,omitempty"`
	ExpectedAmountRUB string    `json:"expected_amount_rub,omitempty"`
	ScheduledAt       time.Time `json:"scheduled_at,omitempty"`
	ActualAt          time.Time `json:"actual_at,omitempty"`
	Message           string    `json:"message,omitempty"`
}

// StopResult is the response of Subscriptions.Stop.
type StopResult struct {
	OK bool `json:"ok"`
}

// unmarshalListOrEnvelope decodes data as a bare JSON array into items, or as
// an envelope {"items": [...], "total": n}; the total defaults to the item
// count when the envelope omits it.
func unmarshalListOrEnvelope(data []byte, items any, total *int) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, items); err != nil {
			return err
		}
		*total = jsonLen(items)
		return nil
	}
	var envelope struct {
		Items json.RawMessage `json:"items"`
		Total *int            `json:"total"`
	}
	if err := json.Unmarshal(trimmed, &envelope); err != nil {
		return err
	}
	if err := json.Unmarshal(envelope.Items, items); err != nil {
		return err
	}
	if envelope.Total != nil {
		*total = *envelope.Total
	} else {
		*total = jsonLen(items)
	}
	return nil
}

// jsonLen returns the length of the slice pointed to by items.
func jsonLen(items any) int {
	switch s := items.(type) {
	case *[]Payment:
		return len(*s)
	case *[]Payout:
		return len(*s)
	case *[]Subscription:
		return len(*s)
	default:
		return 0
	}
}
