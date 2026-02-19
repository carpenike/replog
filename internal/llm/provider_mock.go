package llm

import (
	"context"
	"time"
)

// MockProvider implements Provider for testing. It returns a fixed response.
type MockProvider struct {
	FixedContent string
	PingErr      error
	GenerateErr  error
}

// NewMockProvider creates a mock provider with a canned CatalogJSON response.
func NewMockProvider(catalogJSON string) *MockProvider {
	return &MockProvider{FixedContent: catalogJSON}
}

func (p *MockProvider) Name() string { return "Mock" }

func (p *MockProvider) Ping(_ context.Context) error {
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
