package usecase_test

import (
	"context"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/models"
	"github.com/google/uuid"
)

type MockRepository struct {
	CreateUserFunc        func(ctx context.Context, user *models.User) (*models.User, error)
	GetUserByIDFunc       func(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetUserByEmailFunc    func(ctx context.Context, email string) (*models.User, error)
	GetUserByUsernameFunc func(ctx context.Context, username string) (*models.User, error)
}

func (m *MockRepository) CreateUser(ctx context.Context, user *models.User) (*models.User, error) {
	if m.CreateUserFunc != nil {
		return m.CreateUserFunc(ctx, user)
	}
	return nil, nil
}

func (m *MockRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	if m.GetUserByIDFunc != nil {
		return m.GetUserByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *MockRepository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	if m.GetUserByEmailFunc != nil {
		return m.GetUserByEmailFunc(ctx, email)
	}
	return nil, nil
}

func (m *MockRepository) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	if m.GetUserByUsernameFunc != nil {
		return m.GetUserByUsernameFunc(ctx, username)
	}
	return nil, nil
}
