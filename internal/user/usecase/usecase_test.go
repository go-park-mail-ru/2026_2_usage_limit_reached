package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/usecase"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestUsecase_CreateUser(t *testing.T) {
	repo := &MockRepository{
		CreateUserFunc: func(ctx context.Context, user *models.User) (*models.User, error) {
			require.Equal(t, "test@mail.ru", user.Email, "email must ne normalized")
			require.Equal(t, "testuser", user.Username, "username must ne normalized")
			return user, nil
		},
	}

	uc := usecase.NewUsecase(repo)
	_, err := uc.CreateUser(context.Background(), &models.User{Email: "Test@Mail.ru ", Username: " TestUser "})
	require.NoError(t, err)
}

func TestUsecase_FindUserByLogin(t *testing.T) {
	user := &models.User{ID: uuid.New()}

	byEmail := usecase.NewUsecase(&MockRepository{
		GetUserByEmailFunc: func(ctx context.Context, email string) (*models.User, error) {
			return user, nil
		},
	})
	got, err := byEmail.FindUserByLogin(context.Background(), "user@mail.ru")
	require.NoError(t, err)
	require.Same(t, user, got)

	byUsername := usecase.NewUsecase(&MockRepository{
		GetUserByUsernameFunc: func(ctx context.Context, username string) (*models.User, error) {
			return user, nil
		},
	})
	got, err = byUsername.FindUserByLogin(context.Background(), "username")
	require.NoError(t, err)
	require.Same(t, user, got)
}

func TestUsecase_FindUserByID(t *testing.T) {
	ctx := context.Background()
	testID := uuid.New()
	dummyUser := &models.User{
		ID:       testID,
		Email:    "test@mail.ru",
		Username: "testuser",
		Nickname: "testnick",
	}

	tests := []struct {
		name        string
		setupMocks  func(r *MockRepository)
		expectedErr error
		errContains string
		checkResp   bool
	}{
		{
			name: "Success",
			setupMocks: func(r *MockRepository) {
				r.GetUserByIDFunc = func(ctx context.Context, id uuid.UUID) (*models.User, error) {
					return dummyUser, nil
				}
			},
			checkResp: true,
		},
		{
			name: "User not found",
			setupMocks: func(r *MockRepository) {
				r.GetUserByIDFunc = func(ctx context.Context, id uuid.UUID) (*models.User, error) {
					return nil, models.ErrUserNotFound
				}
			},
			expectedErr: usecase.ErrUserNotFound,
		},
		{
			name: "Repo DB error",
			setupMocks: func(r *MockRepository) {
				r.GetUserByIDFunc = func(ctx context.Context, id uuid.UUID) (*models.User, error) {
					return nil, errors.New("db network error")
				}
			},
			errContains: "find user: db network error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &MockRepository{}
			if tt.setupMocks != nil {
				tt.setupMocks(repo)
			}

			uc := usecase.NewUsecase(repo)
			user, err := uc.FindUserByID(ctx, testID)

			if tt.expectedErr != nil {
				require.ErrorIs(t, err, tt.expectedErr)
				return
			}
			if tt.errContains != "" {
				require.ErrorContains(t, err, tt.errContains)
				return
			}

			require.NoError(t, err)
			if tt.checkResp {
				require.NotNil(t, user)
				require.Equal(t, testID, user.ID)
			}
		})
	}
}
