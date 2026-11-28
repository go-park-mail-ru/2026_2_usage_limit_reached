package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/models"
	identityuc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/usecase/mock_identity_usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newAuthorUsecase(repo *mock_identity_usecase.MockRepository) *identityuc.Usecase {
	return identityuc.New(repo, nil, nil, "/media", nil)
}

func newSearchAuthor(i int) *models.Author {
	avatarKey := "avatars/author-" + string(rune('0'+i%10)) + ".png"

	return &models.Author{
		ID:        uuid.New(),
		Username:  "ivan" + string(rune('0'+i%10)),
		Nickname:  "Иван " + string(rune('0'+i%10)),
		AvatarKey: &avatarKey,
		Bio:       "Био",
	}
}

func TestUsecase_SearchAuthors(t *testing.T) {
	t.Run("no query and no categories", func(t *testing.T) {
		uc := newAuthorUsecase(mock_identity_usecase.NewMockRepository(t))

		got, err := uc.SearchAuthors(context.Background(), dto.AuthorSearchRequest{})

		require.ErrorIs(t, err, validator.ErrValidation)
		require.Nil(t, got)
	})

	t.Run("passes request fields to repository", func(t *testing.T) {
		request := dto.AuthorSearchRequest{
			Query:      "ivan",
			Categories: []string{"Творчество"},
			Cursor:     &dto.AuthorSearchCursor{Nickname: "иван", ID: uuid.New()},
		}

		repo := mock_identity_usecase.NewMockRepository(t)
		repo.EXPECT().SearchAuthors(mock.Anything, request.Query, request.Categories, request.Cursor, identityuc.AuthorSearchPageSize+1).
			Return([]*models.Author{}, nil)

		got, err := newAuthorUsecase(repo).SearchAuthors(context.Background(), request)

		require.NoError(t, err)
		require.Empty(t, got.Items)
	})

	t.Run("builds avatar urls from author avatar key", func(t *testing.T) {
		author := newSearchAuthor(1)
		avatarKey := *author.AvatarKey

		repo := mock_identity_usecase.NewMockRepository(t)
		repo.EXPECT().SearchAuthors(mock.Anything, "ivan", []string(nil), (*dto.AuthorSearchCursor)(nil), identityuc.AuthorSearchPageSize+1).
			Return([]*models.Author{author}, nil)

		got, err := newAuthorUsecase(repo).SearchAuthors(context.Background(), dto.AuthorSearchRequest{Query: "ivan"})

		require.NoError(t, err)
		require.Len(t, got.Items, 1)
		require.Equal(t, author.Username, got.Items[0].Username)
		require.Equal(t, author.Nickname, got.Items[0].Nickname)
		require.Equal(t, author.Bio, got.Items[0].Bio)
		require.NotNil(t, got.Items[0].AvatarURL)
		require.Equal(t, "/media/"+avatarKey, *got.Items[0].AvatarURL)
	})

	t.Run("author without avatar gets nil url", func(t *testing.T) {
		author := newSearchAuthor(2)
		author.AvatarKey = nil

		repo := mock_identity_usecase.NewMockRepository(t)
		repo.EXPECT().SearchAuthors(mock.Anything, "ivan", []string(nil), (*dto.AuthorSearchCursor)(nil), identityuc.AuthorSearchPageSize+1).
			Return([]*models.Author{author}, nil)

		got, err := newAuthorUsecase(repo).SearchAuthors(context.Background(), dto.AuthorSearchRequest{Query: "ivan"})

		require.NoError(t, err)
		require.Nil(t, got.Items[0].AvatarURL)
	})

	t.Run("paginates and passes next cursor to repository", func(t *testing.T) {
		repo := mock_identity_usecase.NewMockRepository(t)

		authors := make([]*models.Author, 0, identityuc.AuthorSearchPageSize+1)
		for i := 0; i <= identityuc.AuthorSearchPageSize; i++ {
			authors = append(authors, newSearchAuthor(i))
		}

		repo.EXPECT().SearchAuthors(mock.Anything, "ivan", []string(nil), (*dto.AuthorSearchCursor)(nil), identityuc.AuthorSearchPageSize+1).
			Return(authors, nil)

		uc := newAuthorUsecase(repo)

		first, err := uc.SearchAuthors(context.Background(), dto.AuthorSearchRequest{Query: "ivan"})
		require.NoError(t, err)
		require.Len(t, first.Items, identityuc.AuthorSearchPageSize)
		require.NotNil(t, first.Next)

		repo.EXPECT().SearchAuthors(mock.Anything, "ivan", []string(nil), first.Next, identityuc.AuthorSearchPageSize+1).
			Return([]*models.Author{}, nil)

		second, err := uc.SearchAuthors(context.Background(), dto.AuthorSearchRequest{Query: "ivan", Cursor: first.Next})
		require.NoError(t, err)
		require.Empty(t, second.Items)
		require.Nil(t, second.Next)
	})
}

func TestUsecase_CreateAuthor(t *testing.T) {
	authorID := testUserID
	category := "Творчество"

	t.Run("creates author for active user", func(t *testing.T) {
		repo := mock_identity_usecase.NewMockRepository(t)
		repo.EXPECT().GetUserByID(mock.Anything, authorID).Return(testUser(), nil)
		repo.EXPECT().CreateAuthor(mock.Anything, authorID, "Пишу о книгах", &category).
			Return(&models.Author{ID: authorID, Username: "ivan001", Nickname: "Иван", Bio: "Пишу о книгах", Category: &category}, nil)

		got, err := newAuthorUsecase(repo).CreateAuthor(context.Background(), authorID, dto.AuthorCreateRequest{Bio: "Пишу о книгах", Category: &category})

		require.NoError(t, err)
		require.Equal(t, dto.AuthorMeResponse{ID: authorID, Username: "ivan001", Nickname: "Иван", Bio: "Пишу о книгах", Category: &category}, *got)
	})

	t.Run("user not found", func(t *testing.T) {
		repo := mock_identity_usecase.NewMockRepository(t)
		repo.EXPECT().GetUserByID(mock.Anything, authorID).Return(nil, models.ErrUserNotFound)

		got, err := newAuthorUsecase(repo).CreateAuthor(context.Background(), authorID, dto.AuthorCreateRequest{Bio: "Пишу о книгах"})

		require.ErrorIs(t, err, identityuc.ErrProfileNotFound)
		require.Nil(t, got)
	})

	t.Run("inactive user is forbidden", func(t *testing.T) {
		user := testUser()
		user.Status = models.StatusBlocked

		repo := mock_identity_usecase.NewMockRepository(t)
		repo.EXPECT().GetUserByID(mock.Anything, authorID).Return(user, nil)

		got, err := newAuthorUsecase(repo).CreateAuthor(context.Background(), authorID, dto.AuthorCreateRequest{Bio: "Пишу о книгах"})

		require.ErrorIs(t, err, identityuc.ErrUserInactive)
		require.Nil(t, got)
	})

	t.Run("repository error is wrapped", func(t *testing.T) {
		repo := mock_identity_usecase.NewMockRepository(t)
		repo.EXPECT().GetUserByID(mock.Anything, authorID).Return(testUser(), nil)
		repo.EXPECT().CreateAuthor(mock.Anything, authorID, "Пишу о книгах", (*string)(nil)).
			Return(nil, errors.New("db down"))

		got, err := newAuthorUsecase(repo).CreateAuthor(context.Background(), authorID, dto.AuthorCreateRequest{Bio: "Пишу о книгах"})

		require.ErrorContains(t, err, "create author")
		require.Nil(t, got)
	})
}

func TestUsecase_GetAuthor(t *testing.T) {
	authorID := testUserID

	t.Run("returns author", func(t *testing.T) {
		repo := mock_identity_usecase.NewMockRepository(t)
		repo.EXPECT().GetUserByID(mock.Anything, authorID).Return(testUser(), nil)
		repo.EXPECT().GetAuthorByID(mock.Anything, authorID).
			Return(&models.Author{ID: authorID, Username: "ivan001", Nickname: "Иван", Bio: "Пишу о книгах"}, nil)

		got, err := newAuthorUsecase(repo).GetAuthor(context.Background(), authorID)

		require.NoError(t, err)
		require.Equal(t, authorID, got.ID)
	})

	t.Run("blocked user is forbidden", func(t *testing.T) {
		user := testUser()
		user.Status = models.StatusDeleted

		repo := mock_identity_usecase.NewMockRepository(t)
		repo.EXPECT().GetUserByID(mock.Anything, authorID).Return(user, nil)

		got, err := newAuthorUsecase(repo).GetAuthor(context.Background(), authorID)

		require.ErrorIs(t, err, identityuc.ErrUserInactive)
		require.Nil(t, got)
	})

	t.Run("not an author", func(t *testing.T) {
		repo := mock_identity_usecase.NewMockRepository(t)
		repo.EXPECT().GetUserByID(mock.Anything, authorID).Return(testUser(), nil)
		repo.EXPECT().GetAuthorByID(mock.Anything, authorID).Return(nil, models.ErrAuthorNotFound)

		got, err := newAuthorUsecase(repo).GetAuthor(context.Background(), authorID)

		require.ErrorIs(t, err, identityuc.ErrAuthorNotFound)
		require.Nil(t, got)
	})
}

func TestUsecase_UpdateAuthor(t *testing.T) {
	authorID := testUserID
	category := "Образование"

	t.Run("replaces bio and category", func(t *testing.T) {
		repo := mock_identity_usecase.NewMockRepository(t)
		repo.EXPECT().GetUserByID(mock.Anything, authorID).Return(testUser(), nil)
		repo.EXPECT().UpdateAuthorDetails(mock.Anything, authorID, "Новое био", &category).
			Return(&models.Author{ID: authorID, Username: "ivan001", Nickname: "Иван", Bio: "Новое био", Category: &category}, nil)

		got, err := newAuthorUsecase(repo).UpdateAuthor(context.Background(), authorID, dto.AuthorCreateRequest{Bio: "Новое био", Category: &category})

		require.NoError(t, err)
		require.Equal(t, dto.AuthorMeResponse{ID: authorID, Username: "ivan001", Nickname: "Иван", Bio: "Новое био", Category: &category}, *got)
	})

	t.Run("not an author", func(t *testing.T) {
		repo := mock_identity_usecase.NewMockRepository(t)
		repo.EXPECT().GetUserByID(mock.Anything, authorID).Return(testUser(), nil)
		repo.EXPECT().UpdateAuthorDetails(mock.Anything, authorID, "Новое био", (*string)(nil)).
			Return(nil, models.ErrAuthorNotFound)

		got, err := newAuthorUsecase(repo).UpdateAuthor(context.Background(), authorID, dto.AuthorCreateRequest{Bio: "Новое био"})

		require.ErrorIs(t, err, identityuc.ErrAuthorNotFound)
		require.Nil(t, got)
	})
}
