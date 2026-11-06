package providermock

import (
	"context"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/provider"
)

type MockProvider struct {
	name string
}

func NewMockProvider() *MockProvider {
	return &MockProvider{name: "mock"}
}

func (m *MockProvider) Charge(_ context.Context, req provider.ChargeRequest) (provider.ChargeResult, error) {
	return provider.ChargeResult{
		Status:      provider.ChargeSucceeded,
		ExternalID:  "mock_" + req.AttemptID.String(),
		AmountMinor: req.AmountMinor,
		Currency:    req.Currency,
	}, nil
}

func (m *MockProvider) Name() string {
	return m.name
}
