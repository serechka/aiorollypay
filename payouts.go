package rollypay

import (
	"context"
	"errors"
	"net/url"
)

// PayoutsService manages USDT withdrawals. Access it via Client.Payouts.
type PayoutsService struct {
	client *Client
}

// CreatePayoutInput is the payload of Payouts.Create.
type CreatePayoutInput struct {
	// AmountUSDT is the withdrawal amount as a string, e.g. "50.00".
	AmountUSDT string `json:"amount_usdt"`
	// WalletAddress is the recipient's crypto wallet address.
	WalletAddress string `json:"wallet_address"`
	// Network is the transfer network, e.g. "TRC-20".
	Network string `json:"network"`
	// IdempotencyKey prevents duplicate payouts when a request is retried.
	// Strongly recommended.
	IdempotencyKey string `json:"idempotency_key,omitempty"`
	// TerminalID selects the source terminal; optional when the API key is
	// bound to one.
	TerminalID string `json:"-"`
}

// Create starts a withdrawal of terminal USDT to an external wallet.
func (s *PayoutsService) Create(ctx context.Context, in CreatePayoutInput) (*Payout, error) {
	if in.AmountUSDT == "" {
		return nil, errors.New("rollypay: amount_usdt is required")
	}
	if in.WalletAddress == "" {
		return nil, errors.New("rollypay: wallet_address is required")
	}
	if in.Network == "" {
		return nil, errors.New("rollypay: network is required")
	}
	q := url.Values{}
	setIfNotEmpty(q, "terminal_id", in.TerminalID)
	var out Payout
	if err := s.client.do(ctx, "POST", "payouts", requestOptions{query: q, body: in, out: &out}); err != nil {
		return nil, err
	}
	return &out, nil
}

// List returns the terminal's payouts; with an empty terminalID the terminal
// of the API key is used.
func (s *PayoutsService) List(ctx context.Context, terminalID string) (*PayoutList, error) {
	q := url.Values{}
	setIfNotEmpty(q, "terminal_id", terminalID)
	var out PayoutList
	if err := s.client.getQuery(ctx, "payouts", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
