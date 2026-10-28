package usecase_test

import (
	"context"
	"time"

	userdto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/dto"
	"github.com/google/uuid"
)

type MockUserProvider struct {
	CreateUserFunc      func(ctx context.Context, req userdto.CreateUserRequest) (*userdto.UserInfo, error)
	FindUserByLoginFunc func(ctx context.Context, login string) (*userdto.UserCredentials, error)
}

func (m *MockUserProvider) CreateUser(ctx context.Context, req userdto.CreateUserRequest) (*userdto.UserInfo, error) {
	if m.CreateUserFunc != nil {
		return m.CreateUserFunc(ctx, req)
	}
	return nil, nil
}

func (m *MockUserProvider) FindUserByLogin(ctx context.Context, login string) (*userdto.UserCredentials, error) {
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
