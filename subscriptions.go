package rollypay

import (
	"context"
	"errors"
	"net/url"

	"github.com/serechka/aiorollypay/internal/uuid"
)

// SubscriptionsService manages recurring SBP payments. Access it via
// Client.Subscriptions.
type SubscriptionsService struct {
	client *Client
}

// CreateSubscriptionInput is the payload of Subscriptions.Create. Two plan
// modes exist: fixed-tariff plans (plan has PayerAmountRUB) take PayerID and
// must not set Amount; cap-limited plans (plan has CapAmountRUB) take Amount
// and must not set PayerID.
type CreateSubscriptionInput struct {
	// TerminalID owning the plan. Required.
	TerminalID string `json:"terminal_id"`
	// PlanID is the subscription plan UUID from Subscriptions.Plans. Required.
	PlanID string `json:"plan_id"`
	// PayerID identifies the payer, e.g. "tg_123456789" (max 32 bytes).
	// Required for fixed-tariff plans.
	PayerID string `json:"payer_id,omitempty"`
	// Amount fixes the charged amount ("990.00") for cap-limited plans; it
	// applies to every cycle.
	Amount string `json:"amount,omitempty"`
	// MerchantRef is the merchant's own subscription reference (max 128 bytes).
	MerchantRef string `json:"merchant_subscription_ref,omitempty"`
	// Description is optional text shown to the payer.
	Description string `json:"description,omitempty"`
	// IdempotencyKey makes creation safe to retry: the same key with the
	// same body returns the existing subscription. When empty a random UUID
	// is generated; supply a stable key if you may retry across processes.
	IdempotencyKey string `json:"-"`
}

// Plans lists the recurring plans available on a terminal.
func (s *SubscriptionsService) Plans(ctx context.Context, terminalID string) ([]SubscriptionPlan, error) {
	if terminalID == "" {
		return nil, errors.New("rollypay: terminal_id is required")
	}
	q := url.Values{"terminal_id": {terminalID}}
	var out []SubscriptionPlan
	if err := s.client.getQuery(ctx, "subscription-plans", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Create opens a subscription and returns it with the consent PayURL to send
// the customer to. An active link is not proof of payment; grant access only
// after the linked payment becomes paid.
func (s *SubscriptionsService) Create(ctx context.Context, in CreateSubscriptionInput) (*Subscription, error) {
	if in.TerminalID == "" {
		return nil, errors.New("rollypay: terminal_id is required")
	}
	if in.PlanID == "" {
		return nil, errors.New("rollypay: plan_id is required")
	}
	if len(in.PayerID) > 32 {
		return nil, errors.New("rollypay: payer_id must be at most 32 bytes")
	}
	if len(in.MerchantRef) > 128 {
		return nil, errors.New("rollypay: merchant_subscription_ref must be at most 128 bytes")
	}
	if in.Amount != "" && in.PayerID != "" {
		return nil, errors.New("rollypay: amount and payer_id are mutually exclusive (cap-limited vs fixed-tariff plan)")
	}
	idempotencyKey := in.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = uuid.NewV4()
	}
	var out Subscription
	if err := s.client.postIdempotent(ctx, "subscriptions", in, &out, idempotencyKey); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get fetches one subscription by ID.
func (s *SubscriptionsService) Get(ctx context.Context, subscriptionID string) (*Subscription, error) {
	if subscriptionID == "" {
		return nil, errors.New("rollypay: subscription_id is required")
	}
	var out Subscription
	if err := s.client.get(ctx, "subscriptions/"+url.PathEscape(subscriptionID), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// List returns the terminal's subscriptions, 50 per page. page <= 0 means 1.
func (s *SubscriptionsService) List(ctx context.Context, terminalID string, page int) (*SubscriptionList, error) {
	if terminalID == "" {
		return nil, errors.New("rollypay: terminal_id is required")
	}
	if page <= 0 {
		page = 1
	}
	q := url.Values{"terminal_id": {terminalID}, "page": {itoa(page)}}
	var out SubscriptionList
	if err := s.client.getQuery(ctx, "subscriptions", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Charges lists the automatic charge history of a subscription (up to 100
// records).
func (s *SubscriptionsService) Charges(ctx context.Context, subscriptionID string) ([]SubscriptionCharge, error) {
	if subscriptionID == "" {
		return nil, errors.New("rollypay: subscription_id is required")
	}
	var out []SubscriptionCharge
	if err := s.client.get(ctx, "subscriptions/"+url.PathEscape(subscriptionID)+"/charges", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Stop cancels auto-renewal of a subscription. Verify the result by re-reading
// the subscription (state "stop", billing_status "stopped"); stopping does not
// refund money or revoke already-paid access.
func (s *SubscriptionsService) Stop(ctx context.Context, subscriptionID string) (*StopResult, error) {
	if subscriptionID == "" {
		return nil, errors.New("rollypay: subscription_id is required")
	}
	var out StopResult
	if err := s.client.post(ctx, "subscriptions/"+url.PathEscape(subscriptionID)+"/stop", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
