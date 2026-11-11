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

var testUserID = uuid.MustParse("00000000-0000-4000-8000-000000000011")

func ptr(s string) *string { return &s }

func TestUsecase_CreateUser_Errors(t *testing.T) {
	repoErr := errors.New("db down")

	tests := []struct {
		name        string
		repoErr     error
		expectedErr error
	}{
		{name: "User exists", repoErr: models.ErrUserExists, expectedErr: usecase.ErrUserExists},
		{name: "Repository error", repoErr: repoErr, expectedErr: repoErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mock_user_usecase.NewMockRepository(t)
			repo.EXPECT().CreateUser(mock.Anything, mock.Anything).Return(nil, tt.repoErr)
			uc := usecase.NewUsecase(repo, testAvatarBasePath)

			got, err := uc.CreateUser(context.Background(), newCreateUserRequest())

			require.ErrorIs(t, err, tt.expectedErr)
			require.Nil(t, got)
		})
	}
}

func TestUsecase_UpdateAvatarKey(t *testing.T) {
	repoErr := errors.New("db down")

	tests := []struct {
		name        string
		oldKey      *string
		repoErr     error
		expectedErr error
	}{
		{name: "Success returns old key", oldKey: ptr("avatars/old.png")},
		{name: "Success without old key"},
		{name: "User not found", repoErr: models.ErrUserNotFound, expectedErr: usecase.ErrUserNotFound},
		{name: "Repository error", repoErr: repoErr, expectedErr: repoErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mock_user_usecase.NewMockRepository(t)
			newKey := ptr("avatars/new.png")
			repo.EXPECT().UpdateAvatarKey(mock.Anything, testUserID, newKey).Return(tt.oldKey, tt.repoErr)
			uc := usecase.NewUsecase(repo, testAvatarBasePath)

			got, err := uc.UpdateAvatarKey(context.Background(), testUserID, newKey)

			if tt.expectedErr != nil {
				require.ErrorIs(t, err, tt.expectedErr)
				require.Nil(t, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.oldKey, got)
		})
	}
}

func TestUsecase_FindCredentialsByLogin_Lookup(t *testing.T) {
	repoErr := errors.New("db down")
	user := &models.User{
		ID:           testUserID,
		Username:     "ivan001",
		Email:        "ivan@gmail.com",
		PasswordHash: "hash",
		AvatarKey:    ptr("avatars/ivan.png"),
		Status:       models.StatusActive,
	}

	tests := []struct {
		name         string
		login        string
		mockBehavior func(m *mock_user_usecase.MockRepository)
		expectedErr  error
	}{
		{
			name:  "Email login is normalized",
			login: "  Ivan@Gmail.com ",
			mockBehavior: func(m *mock_user_usecase.MockRepository) {
				m.EXPECT().GetUserByEmail(mock.Anything, "ivan@gmail.com").Return(user, nil)
			},
		},
		{
			name:  "Username login is normalized",
			login: " IVAN001 ",
			mockBehavior: func(m *mock_user_usecase.MockRepository) {
				m.EXPECT().GetUserByUsername(mock.Anything, "ivan001").Return(user, nil)
			},
		},
		{
			name:  "User not found",
			login: "ghost",
			mockBehavior: func(m *mock_user_usecase.MockRepository) {
				m.EXPECT().GetUserByUsername(mock.Anything, "ghost").Return(nil, models.ErrUserNotFound)
			},
			expectedErr: usecase.ErrUserNotFound,
		},
		{
			name:  "Repository error",
			login: "ghost@mail.ru",
			mockBehavior: func(m *mock_user_usecase.MockRepository) {
				m.EXPECT().GetUserByEmail(mock.Anything, "ghost@mail.ru").Return(nil, repoErr)
			},
			expectedErr: repoErr,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := mock_user_usecase.NewMockRepository(t)
			tt.mockBehavior(repo)
			uc := usecase.NewUsecase(repo, testAvatarBasePath)

			got, err := uc.FindCredentialsByLogin(context.Background(), tt.login)

			if tt.expectedErr != nil {
				require.ErrorIs(t, err, tt.expectedErr)
				require.Nil(t, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, "hash", got.PasswordHash)
			require.Equal(t, "ivan001", got.Username)
			require.NotNil(t, got.AvatarURL)
			require.Equal(t, "/media/avatars/ivan.png", *got.AvatarURL)
		})
	}
}

func newCreateUserRequest() dto.CreateUserRequest {
	return dto.CreateUserRequest{Email: "a@b.c", Username: "user", Nickname: "nick", PasswordHash: "hash"}
}
