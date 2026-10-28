package usecase_test

import (
	"context"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/models"
	"github.com/google/uuid"
)

type MockUserProvider struct {
	CreateUserFunc      func(ctx context.Context, user *models.User) (*models.User, error)
	FindUserByLoginFunc func(ctx context.Context, login string) (*models.User, error)
}

func (m *MockUserProvider) CreateUser(ctx context.Context, user *models.User) (*models.User, error) {
	if m.CreateUserFunc != nil {
		return m.CreateUserFunc(ctx, user)
	}
	return nil, nil
}

func (m *MockUserProvider) FindUserByLogin(ctx context.Context, login string) (*models.User, error) {
	if m.FindUserByLoginFunc != nil {
		return m.FindUserByLoginFunc(ctx, login)
	}
	return nil, nil
}

type MockTokenManager struct {
	GenerateFunc func(ID uuid.UUID, payload any) (string, error)
	TTLFunc      func() time.Duration
}

func (m *MockTokenManager) Generate(ID uuid.UUID, payload any) (string, error) {
	if m.GenerateFunc != nil {
		return m.GenerateFunc(ID, payload)
	}
	return "", nil
}

func (m *MockTokenManager) TTL() time.Duration {
	if m.TTLFunc != nil {
		return m.TTLFunc()
	}
	return 24 * time.Hour
}
