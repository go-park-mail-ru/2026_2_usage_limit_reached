package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/usecase"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestUsecase_CreateUser(t *testing.T) {
	repo := &MockRepository{
		CreateUserFunc: func(ctx context.Context, user models.User) (*models.User, error) {
			require.Equal(t, "test@mail.ru", user.Email, "email must ne normalized")
			require.Equal(t, "testuser", user.Username, "username must ne normalized")
			require.Equal(t, "Test Nick", user.Nickname, "nickname must be trimmed")

			return &user, nil
		},
	}

	uc := usecase.NewUsecase(repo)
	_, err := uc.CreateUser(context.Background(), dto.CreateUserRequest{Email: "Test@Mail.ru ", Username: " TestUser ", Nickname: " Test Nick "})
	require.NoError(t, err)
}

func TestUsecase_UpdateNickname(t *testing.T) {
	technicalErr := errors.New("db network error")
	tests := []struct {
		name     string
		nickname string
		repoErr  error
		wantErr  error
		wantSent string
	}{
		{name: "success", nickname: "nick", wantSent: "nick"},
		{name: "trims spaces", nickname: "  nick  ", wantSent: "nick"},
		{name: "empty nickname", nickname: "", wantErr: usecase.ErrInvalidNickname},
		{name: "whitespace nickname", nickname: "   ", wantErr: usecase.ErrInvalidNickname},
		{name: "user not found", nickname: "nick", repoErr: models.ErrUserNotFound, wantSent: "nick", wantErr: usecase.ErrUserNotFound},
		{name: "repo error", nickname: "nick", repoErr: technicalErr, wantSent: "nick", wantErr: technicalErr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sent string

			uc := usecase.NewUsecase(&MockRepository{
				UpdateNicknameFunc: func(ctx context.Context, id uuid.UUID, nickname string) (string, error) {
					sent = nickname
					return nickname, tt.repoErr
				},
			})

			got, err := uc.UpdateNickname(context.Background(), uuid.New(), tt.nickname)

			require.Equal(t, tt.wantSent, sent)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantSent, got)
		})
	}
}

func TestUsecase_FindCredentialsByLogin(t *testing.T) {
	user := &models.User{ID: uuid.New()}

	byEmail := usecase.NewUsecase(&MockRepository{
		GetUserByEmailFunc: func(ctx context.Context, email string) (*models.User, error) {
			return user, nil
		},
	})
	got, err := byEmail.FindCredentialsByLogin(context.Background(), "user@mail.ru")
	require.NoError(t, err)
	require.Equal(t, user.ID, got.ID)

	byUsername := usecase.NewUsecase(&MockRepository{
		GetUserByUsernameFunc: func(ctx context.Context, username string) (*models.User, error) {
			return user, nil
		},
	})
	got, err = byUsername.FindCredentialsByLogin(context.Background(), "username")
	require.NoError(t, err)
	require.Equal(t, user.ID, got.ID)
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
