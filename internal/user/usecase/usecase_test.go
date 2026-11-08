package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/usecase/mock_user_usecase"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const testAvatarBasePath = "/media"

func TestUsecase_CreateUser(t *testing.T) {
	repo := mock_user_usecase.NewMockRepository(t)
	repo.EXPECT().CreateUser(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, user models.User) (*models.User, error) {
		require.Equal(t, "test@mail.ru", user.Email, "email must ne normalized")
		require.Equal(t, "testuser", user.Username, "username must ne normalized")
		require.Equal(t, "Test Nick", user.Nickname, "nickname must be trimmed")

		return &user, nil
	})

	uc := usecase.NewUsecase(repo, testAvatarBasePath)
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
			userID := uuid.New()

			repo := mock_user_usecase.NewMockRepository(t)
			if tt.wantSent != "" {
				repo.EXPECT().UpdateNickname(mock.Anything, userID, tt.wantSent).Return(tt.wantSent, tt.repoErr)
			}

			uc := usecase.NewUsecase(repo, testAvatarBasePath)
			got, err := uc.UpdateNickname(context.Background(), userID, tt.nickname)

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

	byEmailRepo := mock_user_usecase.NewMockRepository(t)
	byEmailRepo.EXPECT().GetUserByEmail(mock.Anything, "user@mail.ru").Return(user, nil)

	byEmail := usecase.NewUsecase(byEmailRepo, testAvatarBasePath)
	got, err := byEmail.FindCredentialsByLogin(context.Background(), "user@mail.ru")
	require.NoError(t, err)
	require.Equal(t, user.ID, got.ID)

	byUsernameRepo := mock_user_usecase.NewMockRepository(t)
	byUsernameRepo.EXPECT().GetUserByUsername(mock.Anything, "username").Return(user, nil)

	byUsername := usecase.NewUsecase(byUsernameRepo, testAvatarBasePath)
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
		setupMocks  func(r *mock_user_usecase.MockRepository)
		expectedErr error
		errContains string
		checkResp   bool
	}{
		{
			name: "Success",
			setupMocks: func(r *mock_user_usecase.MockRepository) {
				r.EXPECT().GetUserByID(mock.Anything, testID).Return(dummyUser, nil)
			},
			checkResp: true,
		},
		{
			name: "User not found",
			setupMocks: func(r *mock_user_usecase.MockRepository) {
				r.EXPECT().GetUserByID(mock.Anything, testID).Return(nil, models.ErrUserNotFound)
			},
			expectedErr: usecase.ErrUserNotFound,
		},
		{
			name: "Repo DB error",
			setupMocks: func(r *mock_user_usecase.MockRepository) {
				r.EXPECT().GetUserByID(mock.Anything, testID).Return(nil, errors.New("db network error"))
			},
			errContains: "find user: db network error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mock_user_usecase.NewMockRepository(t)
			if tt.setupMocks != nil {
				tt.setupMocks(repo)
			}

			uc := usecase.NewUsecase(repo, testAvatarBasePath)
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
