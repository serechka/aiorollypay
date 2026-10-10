package rollypay

import (
	"context"
	"net/url"
	"strconv"
)

// MiscService groups the small standalone endpoints: account info, terminal
// balance and the exchange rate. Access it via Client.Misc.
type MiscService struct {
	client *Client
}

// Me returns information about the API key's account.
func (s *MiscService) Me(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := s.client.get(ctx, "me", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Balance returns the USDT balance of a terminal. With an API key bound to a
// single terminal, terminalID may be empty and the key's terminal is used.
func (s *MiscService) Balance(ctx context.Context, terminalID string) (*Balance, error) {
	q := url.Values{}
	setIfNotEmpty(q, "terminal_id", terminalID)
	var out Balance
	if err := s.client.getQuery(ctx, "balance", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Rate returns the current USDT/RUB rate; with terminalID the rate includes
// that terminal's markup.
func (s *MiscService) Rate(ctx context.Context, terminalID string) (*Rate, error) {
	q := url.Values{}
	setIfNotEmpty(q, "terminal_id", terminalID)
	var out Rate
	if err := s.client.getQuery(ctx, "rate", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// setIfNotEmpty adds key=value to q only when value is non-empty.
func setIfNotEmpty(q url.Values, key, value string) {
	if value != "" {
		q.Set(key, value)
	}
}

// itoa formats n for query strings.
func itoa(n int) string { return strconv.Itoa(n) }
