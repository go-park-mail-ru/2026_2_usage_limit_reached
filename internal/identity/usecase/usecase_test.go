package usecase_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/models"
	identityuc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/usecase/mock_identity_usecase"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

var (
	testUserID = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	testTime   = time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
)

func testUser() *models.User {
	return &models.User{
		ID:        testUserID,
		Username:  "ivan001",
		Nickname:  "Иван",
		Email:     "ivan@example.test",
		Status:    models.StatusActive,
		CreatedAt: testTime,
		UpdatedAt: testTime,
	}
}

func newUsecase(repo *mock_identity_usecase.MockRepository, tokens *mock_identity_usecase.MockTokenManager, avatars *mock_identity_usecase.MockAvatarStorage) *identityuc.Usecase {
	return identityuc.New(repo, tokens, avatars, "", nil)
}

func TestRegister(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := mock_identity_usecase.NewMockRepository(t)
		repo.EXPECT().CreateUser(mock.Anything, mock.MatchedBy(func(u models.User) bool {
			return u.Email == "ivan@example.test" && u.Username == "ivan001" && u.Status == models.StatusActive
		})).Return(testUser(), nil)

		tokens := mock_identity_usecase.NewMockTokenManager(t)
		tokens.EXPECT().Generate(testUserID, mock.Anything).Return("token", nil)

		got, token, err := newUsecase(repo, tokens, nil).Register(context.Background(), dto.RegistrationRequest{
			Email: "ivan@example.test", Username: "ivan001", Nickname: "Иван", Password: "password123",
		})

		require.NoError(t, err)
		require.Equal(t, "token", token)
		require.Equal(t, "ivan001", got.Username)
	})

	t.Run("existing user", func(t *testing.T) {
		repo := mock_identity_usecase.NewMockRepository(t)
		repo.EXPECT().CreateUser(mock.Anything, mock.Anything).Return(nil, models.ErrUserExists)

		got, _, err := newUsecase(repo, mock_identity_usecase.NewMockTokenManager(t), nil).Register(context.Background(), dto.RegistrationRequest{
			Email: "ivan@example.test", Username: "ivan001", Nickname: "Иван", Password: "password123",
		})

		require.ErrorIs(t, err, identityuc.ErrRegistrationFailed)
		require.Nil(t, got)
	})
}

func TestLogin(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	require.NoError(t, err)

	userWithHash := func() *models.User {
		user := testUser()
		user.PasswordHash = string(hash)

		return user
	}

	t.Run("success", func(t *testing.T) {
		repo := mock_identity_usecase.NewMockRepository(t)
		repo.EXPECT().GetUserByUsername(mock.Anything, "ivan001").Return(userWithHash(), nil)

		tokens := mock_identity_usecase.NewMockTokenManager(t)
		tokens.EXPECT().Generate(testUserID, mock.Anything).Return("token", nil)

		got, token, err := newUsecase(repo, tokens, nil).Login(context.Background(), dto.LoginRequest{Login: "ivan001", Password: "password123"})

		require.NoError(t, err)
		require.Equal(t, "token", token)
		require.Equal(t, "ivan001", got.Username)
	})

	t.Run("wrong password", func(t *testing.T) {
		repo := mock_identity_usecase.NewMockRepository(t)
		repo.EXPECT().GetUserByUsername(mock.Anything, "ivan001").Return(userWithHash(), nil)

		got, _, err := newUsecase(repo, mock_identity_usecase.NewMockTokenManager(t), nil).Login(context.Background(), dto.LoginRequest{Login: "ivan001", Password: "wrong"})

		require.ErrorIs(t, err, identityuc.ErrLoginFailed)
		require.Nil(t, got)
	})

	t.Run("unknown user", func(t *testing.T) {
		repo := mock_identity_usecase.NewMockRepository(t)
		repo.EXPECT().GetUserByUsername(mock.Anything, "ghost").Return(nil, models.ErrUserNotFound)

		got, _, err := newUsecase(repo, mock_identity_usecase.NewMockTokenManager(t), nil).Login(context.Background(), dto.LoginRequest{Login: "ghost", Password: "password123"})

		require.ErrorIs(t, err, identityuc.ErrLoginFailed)
		require.Nil(t, got)
	})
}

func TestVerifyToken(t *testing.T) {
	raw, err := json.Marshal(map[string]any{"user_id": testUserID.String(), "role": "user"})
	require.NoError(t, err)

	t.Run("valid token", func(t *testing.T) {
		tokens := mock_identity_usecase.NewMockTokenManager(t)
		tokens.EXPECT().Verify("token").Return(json.RawMessage(raw), nil)

		got, err := newUsecase(nil, tokens, nil).VerifyToken(context.Background(), "token")

		require.NoError(t, err)
		require.Equal(t, testUserID, got.ID)
		require.Equal(t, "user", got.Role)
	})

	t.Run("invalid token", func(t *testing.T) {
		tokens := mock_identity_usecase.NewMockTokenManager(t)
		tokens.EXPECT().Verify("bad").Return(nil, errors.New("bad signature"))

		got, err := newUsecase(nil, tokens, nil).VerifyToken(context.Background(), "bad")

		require.ErrorIs(t, err, identityuc.ErrInvalidToken)
		require.Nil(t, got)
	})

	t.Run("empty token", func(t *testing.T) {
		got, err := newUsecase(nil, mock_identity_usecase.NewMockTokenManager(t), nil).VerifyToken(context.Background(), " ")

		require.ErrorIs(t, err, identityuc.ErrInvalidToken)
		require.Nil(t, got)
	})
}

func TestFindUsersByIDs(t *testing.T) {
	t.Run("empty ids", func(t *testing.T) {
		got, err := newUsecase(mock_identity_usecase.NewMockRepository(t), nil, nil).FindUsersByIDs(context.Background(), nil)

		require.NoError(t, err)
		require.Empty(t, got)
	})

	t.Run("found", func(t *testing.T) {
		repo := mock_identity_usecase.NewMockRepository(t)
		repo.EXPECT().GetUsersByIDs(mock.Anything, []uuid.UUID{testUserID}).Return([]models.User{*testUser()}, nil)

		got, err := newUsecase(repo, nil, nil).FindUsersByIDs(context.Background(), []uuid.UUID{testUserID})

		require.NoError(t, err)
		require.Len(t, got, 1)
		require.Equal(t, testUserID, got[0].ID)
	})
}

func TestFindUserByUsername(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		repo := mock_identity_usecase.NewMockRepository(t)
		repo.EXPECT().GetUserByUsername(mock.Anything, "ivan001").Return(testUser(), nil)

		got, err := newUsecase(repo, nil, nil).FindUserByUsername(context.Background(), " Ivan001 ")

		require.NoError(t, err)
		require.Equal(t, "ivan001", got.Username)
	})

	t.Run("not found", func(t *testing.T) {
		repo := mock_identity_usecase.NewMockRepository(t)
		repo.EXPECT().GetUserByUsername(mock.Anything, "ghost").Return(nil, models.ErrUserNotFound)

		got, err := newUsecase(repo, nil, nil).FindUserByUsername(context.Background(), "ghost")

		require.ErrorIs(t, err, identityuc.ErrUserNotFound)
		require.Nil(t, got)
	})
}

func TestGetPublicProfile(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		repo := mock_identity_usecase.NewMockRepository(t)
		repo.EXPECT().GetUserByUsername(mock.Anything, "ivan001").Return(testUser(), nil)
		repo.EXPECT().GetAuthorByID(mock.Anything, testUserID).Return(nil, models.ErrAuthorNotFound)

		got, err := newUsecase(repo, nil, nil).GetPublicProfile(context.Background(), "ivan001")

		require.NoError(t, err)
		require.Equal(t, testUserID, got.User.ID)
		require.Nil(t, got.Author)
	})

	t.Run("includes author block", func(t *testing.T) {
		author := &models.Author{ID: testUserID, Username: "ivan001", Nickname: "Иван", Bio: "Пишу о книгах"}

		repo := mock_identity_usecase.NewMockRepository(t)
		repo.EXPECT().GetUserByUsername(mock.Anything, "ivan001").Return(testUser(), nil)
		repo.EXPECT().GetAuthorByID(mock.Anything, testUserID).Return(author, nil)

		got, err := newUsecase(repo, nil, nil).GetPublicProfile(context.Background(), "ivan001")

		require.NoError(t, err)
		require.NotNil(t, got.Author)
		require.Equal(t, "Пишу о книгах", got.Author.Bio)
	})

	t.Run("inactive user hidden", func(t *testing.T) {
		user := testUser()
		user.Status = models.StatusBlocked

		repo := mock_identity_usecase.NewMockRepository(t)
		repo.EXPECT().GetUserByUsername(mock.Anything, "ivan001").Return(user, nil)

		got, err := newUsecase(repo, nil, nil).GetPublicProfile(context.Background(), "ivan001")

		require.ErrorIs(t, err, identityuc.ErrProfileNotFound)
		require.Nil(t, got)
	})
}

func TestUpdateMyProfile(t *testing.T) {
	repo := mock_identity_usecase.NewMockRepository(t)
	repo.EXPECT().GetUserByID(mock.Anything, testUserID).Return(testUser(), nil)
	repo.EXPECT().UpdateNickname(mock.Anything, testUserID, "Новый").Return("Новый", nil)
	repo.EXPECT().GetAuthorByID(mock.Anything, testUserID).Return(nil, models.ErrAuthorNotFound)

	got, err := newUsecase(repo, nil, nil).UpdateMyProfile(context.Background(), testUserID, dto.UpdateProfileRequest{Nickname: "Новый"})

	require.NoError(t, err)
	require.Equal(t, "Новый", got.User.Nickname)
	require.Nil(t, got.Author)
}

func TestUploadAvatarTooLarge(t *testing.T) {
	got, err := newUsecase(nil, nil, nil).UploadAvatar(context.Background(), testUserID, nil, identityuc.MaxAvatarSize+1)

	require.ErrorIs(t, err, identityuc.ErrAvatarTooLarge)
	require.Empty(t, got)
}

func TestDeleteAvatar(t *testing.T) {
	oldKey := "avatars/old.png"

	repo := mock_identity_usecase.NewMockRepository(t)
	repo.EXPECT().UpdateAvatarKey(mock.Anything, testUserID, (*string)(nil)).Return(&oldKey, nil)

	avatars := mock_identity_usecase.NewMockAvatarStorage(t)
	avatars.EXPECT().Delete(mock.Anything, oldKey).Return(nil)

	err := newUsecase(repo, nil, avatars).DeleteAvatar(context.Background(), testUserID)

	require.NoError(t, err)
}

func TestDeleteAvatarNotFound(t *testing.T) {
	repo := mock_identity_usecase.NewMockRepository(t)
	repo.EXPECT().UpdateAvatarKey(mock.Anything, testUserID, (*string)(nil)).Return(nil, models.ErrUserNotFound)

	err := newUsecase(repo, nil, nil).DeleteAvatar(context.Background(), testUserID)

	require.ErrorIs(t, err, identityuc.ErrProfileNotFound)
}
