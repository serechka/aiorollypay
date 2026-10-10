package rollypay

import (
	"context"
	"errors"
	"net/url"
)

// StatsService exposes merchant statistics. Access it via Client.Stats.
// The API does not publish fixed schemas for these payloads, so they are
// returned as decoded JSON objects.
type StatsService struct {
	client *Client
}

// Overview returns summary statistics for a terminal.
func (s *StatsService) Overview(ctx context.Context, terminalID string) (map[string]any, error) {
	if terminalID == "" {
		return nil, errors.New("rollypay: terminal_id is required")
	}
	q := url.Values{"terminal_id": {terminalID}}
	var out map[string]any
	if err := s.client.getQuery(ctx, "merchant/stats/overview", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// FinanceSummary returns the merchant finance summary, across all terminals
// when terminalID is empty.
func (s *StatsService) FinanceSummary(ctx context.Context, terminalID string) (map[string]any, error) {
	q := url.Values{}
	setIfNotEmpty(q, "terminal_id", terminalID)
	var out map[string]any
	if err := s.client.getQuery(ctx, "merchant/finance/summary", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Timeseries returns statistics over time, e.g. metric "volume" or "count"
// with interval "day" or "hour".
func (s *StatsService) Timeseries(ctx context.Context, metric, interval, terminalID string) (map[string]any, error) {
	if metric == "" {
		return nil, errors.New("rollypay: metric is required")
	}
	if interval == "" {
		return nil, errors.New("rollypay: interval is required")
	}
	q := url.Values{"metric": {metric}, "interval": {interval}}
	setIfNotEmpty(q, "terminal_id", terminalID)
	var out map[string]any
	if err := s.client.getQuery(ctx, "merchant/stats/timeseries", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}
