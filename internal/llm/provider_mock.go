package llm

import (
	"context"
	"time"
)

// MockProvider implements Provider for testing. It returns a fixed response.
//
// PingDelay simulates a slow/hung provider: Ping blocks for up to PingDelay
// or until the caller's context fires (whichever comes first). Used by the
// handler-level tests that exercise the request-scoped timeout we wrap
// around the admin "Test Connection" endpoints.
type MockProvider struct {
	FixedContent string
	PingErr      error
	PingDelay    time.Duration
	GenerateErr  error
}

func (p *MockProvider) Name() string { return "Mock" }

func (p *MockProvider) Ping(ctx context.Context) error {
	if p.PingDelay > 0 {
		select {
		case <-time.After(p.PingDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return p.PingErr
}

func (p *MockProvider) Generate(_ context.Context, _, _ string, _ Options) (*Response, error) {
	if p.GenerateErr != nil {
		return nil, p.GenerateErr
	}
	return &Response{
		Content:    p.FixedContent,
		Model:      "mock",
		TokensUsed: 100,
		Duration:   time.Millisecond,
	}, nil
}
