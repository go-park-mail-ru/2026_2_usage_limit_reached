package handlers

import (
	"context"
	"io"
	"log/slog"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/testutils"
)

type MockUseCase struct {
	RegisterFunc func(ctx context.Context, regInput dto.RegistrationRequest) (*dto.UserResponse, string, error)
	LoginFunc    func(ctx context.Context, loginInput dto.LoginRequest) (*dto.UserResponse, string, error)
	TokenTTLFunc func() time.Duration
}

func (m *MockUseCase) Register(ctx context.Context, regInput dto.RegistrationRequest) (*dto.UserResponse, string, error) {
	if m.RegisterFunc != nil {
		return m.RegisterFunc(ctx, regInput)
	}
	return nil, "", nil
}

func (m *MockUseCase) Login(ctx context.Context, loginInput dto.LoginRequest) (*dto.UserResponse, string, error) {
	if m.LoginFunc != nil {
		return m.LoginFunc(ctx, loginInput)
	}
	return nil, "", nil
}

func (m *MockUseCase) TokenTTL() time.Duration {
	if m.TokenTTLFunc != nil {
		return m.TokenTTLFunc()
	}
	return 24 * time.Hour
}

func setupTestHandler(uc UseCase) *Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	val := testutils.SetupValidator()
	return NewHandler(uc, val, logger)
}
