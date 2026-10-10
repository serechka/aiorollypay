package rollypay

import (
	"context"
	"errors"
	"net/url"
)

// TerminalsService manages кассы (cash registers). Access it via
// Client.Terminals.
type TerminalsService struct {
	client *Client
}

// List returns every terminal of the account.
func (s *TerminalsService) List(ctx context.Context) ([]Terminal, error) {
	var out []Terminal
	if err := s.client.get(ctx, "terminals", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Get fetches one terminal by ID.
func (s *TerminalsService) Get(ctx context.Context, terminalID string) (*Terminal, error) {
	if terminalID == "" {
		return nil, errors.New("rollypay: terminal_id is required")
	}
	var out Terminal
	if err := s.client.get(ctx, "terminals/"+url.PathEscape(terminalID), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update applies the non-zero fields of upd to the terminal.
func (s *TerminalsService) Update(ctx context.Context, terminalID string, upd TerminalUpdate) (*Terminal, error) {
	if terminalID == "" {
		return nil, errors.New("rollypay: terminal_id is required")
	}
	var out Terminal
	if err := s.client.put(ctx, "terminals/"+url.PathEscape(terminalID), upd, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RotateAPIKey replaces the terminal's API key. The old key stops working
// immediately; store the returned key durably.
func (s *TerminalsService) RotateAPIKey(ctx context.Context, terminalID string) (*APIKeyRotation, error) {
	if terminalID == "" {
		return nil, errors.New("rollypay: terminal_id is required")
	}
	var out APIKeyRotation
	if err := s.client.post(ctx, "terminals/"+url.PathEscape(terminalID)+"/rotate-api-key", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Providers lists the payment providers enabled for the terminal.
func (s *TerminalsService) Providers(ctx context.Context, terminalID string) ([]Provider, error) {
	if terminalID == "" {
		return nil, errors.New("rollypay: terminal_id is required")
	}
	var out []Provider
	if err := s.client.get(ctx, "terminals/"+url.PathEscape(terminalID)+"/providers", &out); err != nil {
		return nil, err
	}
	return out, nil
}
