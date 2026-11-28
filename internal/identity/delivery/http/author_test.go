package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	handlers "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/delivery/http"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/delivery/http/mock_delivery_http"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/dto"
	identityuc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/testutils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newAuthorRequest(method, target, body string, authorized bool) *http.Request {
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	if authorized {
		r = testutils.WithUser(r, testUserID)
	}
	return r
}

func authorWrapped(err error) error {
	return fmt.Errorf("usecase: %w", err)
}

func TestAuthorHandler_SearchAuthors(t *testing.T) {
	t.Parallel()

	avatarURL := "/media/avatars/ivan.png"
	nextCursor := &dto.AuthorSearchCursor{Nickname: "иван петров", ID: uuid.New()}

	page := &dto.AuthorSearchPage{
		Items: []*dto.AuthorSearchItem{{
			Username:  "ivan_petrov",
			Nickname:  "Иван Петров",
			AvatarURL: &avatarURL,
			Bio:       "Пишу о книгах",
		}},
		Next: nextCursor,
	}

	t.Run("returns page with cursor", func(t *testing.T) {
		t.Parallel()

		uc := mock_delivery_http.NewMockAuthorUsecase(t)
		uc.EXPECT().SearchAuthors(mock.Anything, dto.AuthorSearchRequest{Query: "ivan"}).Return(page, nil)

		recorder := httptest.NewRecorder()
		handlers.NewAuthorHandler(uc, testutils.SetupValidator(), nil).SearchAuthors(recorder, httptest.NewRequest(http.MethodGet, "/authors?q=ivan", nil))

		require.Equal(t, http.StatusOK, recorder.Code)

		var resp dto.AuthorSearchResponse
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
		require.Len(t, resp.Items, 1)
		require.NotNil(t, resp.NextCursor)

		decoded, err := handlers.AuthorSearchCursor.Decode(*resp.NextCursor)
		require.NoError(t, err)
		require.Equal(t, nextCursor, decoded)
	})

	t.Run("null avatar and end of list", func(t *testing.T) {
		t.Parallel()

		uc := mock_delivery_http.NewMockAuthorUsecase(t)
		uc.EXPECT().SearchAuthors(mock.Anything, dto.AuthorSearchRequest{Query: "ivan"}).
			Return(&dto.AuthorSearchPage{Items: []*dto.AuthorSearchItem{{Username: "ivan_petrov", Nickname: "Иван"}}}, nil)

		recorder := httptest.NewRecorder()
		handlers.NewAuthorHandler(uc, testutils.SetupValidator(), nil).SearchAuthors(recorder, httptest.NewRequest(http.MethodGet, "/authors?q=ivan", nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		require.Contains(t, recorder.Body.String(), `"avatar_url":null`)
		require.Contains(t, recorder.Body.String(), `"next_cursor":null`)
	})

	t.Run("invalid cursor", func(t *testing.T) {
		t.Parallel()

		uc := mock_delivery_http.NewMockAuthorUsecase(t)

		recorder := httptest.NewRecorder()
		handlers.NewAuthorHandler(uc, testutils.SetupValidator(), nil).SearchAuthors(
			recorder,
			httptest.NewRequest(http.MethodGet, "/authors?q=ivan&cursor=not-a-cursor", nil),
		)

		require.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("empty query and categories", func(t *testing.T) {
		t.Parallel()

		uc := mock_delivery_http.NewMockAuthorUsecase(t)

		recorder := httptest.NewRecorder()
		handlers.NewAuthorHandler(uc, testutils.SetupValidator(), nil).SearchAuthors(recorder, httptest.NewRequest(http.MethodGet, "/authors", nil))

		require.Equal(t, http.StatusBadRequest, recorder.Code)
	})
}

func TestAuthorHandler_BecomeAuthor(t *testing.T) {
	t.Parallel()

	userID := testUserID
	category := "Творчество"
	author := &dto.AuthorMeResponse{ID: userID, Username: "ivan001", Nickname: "Иван", Bio: "Пишу о книгах", Category: &category}

	t.Run("creates author", func(t *testing.T) {
		t.Parallel()

		uc := mock_delivery_http.NewMockAuthorUsecase(t)
		uc.EXPECT().CreateAuthor(mock.Anything, userID, dto.AuthorCreateRequest{Bio: "Пишу о книгах", Category: &category}).Return(author, nil)

		recorder := httptest.NewRecorder()
		handlers.NewAuthorHandler(uc, testutils.SetupValidator(), testutils.SetupLogger()).BecomeAuthor(
			recorder,
			newAuthorRequest(http.MethodPost, "/authors/me", `{"bio":"Пишу о книгах","category":"Творчество"}`, true),
		)

		require.Equal(t, http.StatusOK, recorder.Code)

		var got dto.AuthorMeResponse
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &got))
		require.Equal(t, *author, got)
	})

	t.Run("malformed body", func(t *testing.T) {
		t.Parallel()

		uc := mock_delivery_http.NewMockAuthorUsecase(t)

		recorder := httptest.NewRecorder()
		handlers.NewAuthorHandler(uc, testutils.SetupValidator(), testutils.SetupLogger()).BecomeAuthor(
			recorder,
			newAuthorRequest(http.MethodPost, "/authors/me", `{"bio":`, true),
		)

		require.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("empty bio", func(t *testing.T) {
		t.Parallel()

		uc := mock_delivery_http.NewMockAuthorUsecase(t)

		recorder := httptest.NewRecorder()
		handlers.NewAuthorHandler(uc, testutils.SetupValidator(), testutils.SetupLogger()).BecomeAuthor(
			recorder,
			newAuthorRequest(http.MethodPost, "/authors/me", `{"bio":""}`, true),
		)

		require.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("unknown category", func(t *testing.T) {
		t.Parallel()

		uc := mock_delivery_http.NewMockAuthorUsecase(t)

		recorder := httptest.NewRecorder()
		handlers.NewAuthorHandler(uc, testutils.SetupValidator(), testutils.SetupLogger()).BecomeAuthor(
			recorder,
			newAuthorRequest(http.MethodPost, "/authors/me", `{"bio":"Пишу о книгах","category":"мусор"}`, true),
		)

		require.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("blocked user", func(t *testing.T) {
		t.Parallel()

		uc := mock_delivery_http.NewMockAuthorUsecase(t)
		uc.EXPECT().CreateAuthor(mock.Anything, userID, dto.AuthorCreateRequest{Bio: "Пишу о книгах"}).
			Return(nil, authorWrapped(identityuc.ErrUserInactive))

		recorder := httptest.NewRecorder()
		handlers.NewAuthorHandler(uc, testutils.SetupValidator(), testutils.SetupLogger()).BecomeAuthor(
			recorder,
			newAuthorRequest(http.MethodPost, "/authors/me", `{"bio":"Пишу о книгах"}`, true),
		)

		require.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("unauthorized", func(t *testing.T) {
		t.Parallel()

		uc := mock_delivery_http.NewMockAuthorUsecase(t)

		recorder := httptest.NewRecorder()
		handlers.NewAuthorHandler(uc, testutils.SetupValidator(), testutils.SetupLogger()).BecomeAuthor(
			recorder,
			newAuthorRequest(http.MethodPost, "/authors/me", `{"bio":"Пишу о книгах"}`, false),
		)

		require.Equal(t, http.StatusUnauthorized, recorder.Code)
	})
}

func TestAuthorHandler_GetMeAuthor(t *testing.T) {
	t.Parallel()

	userID := testUserID
	category := "Игры"
	author := &dto.AuthorMeResponse{ID: userID, Username: "ivan001", Nickname: "Иван", Bio: "Пишу об играх", Category: &category}

	t.Run("returns author block", func(t *testing.T) {
		t.Parallel()

		uc := mock_delivery_http.NewMockAuthorUsecase(t)
		uc.EXPECT().GetAuthor(mock.Anything, userID).Return(author, nil)

		recorder := httptest.NewRecorder()
		handlers.NewAuthorHandler(uc, testutils.SetupValidator(), nil).GetMeAuthor(
			recorder,
			newAuthorRequest(http.MethodGet, "/authors/me", "", true),
		)

		require.Equal(t, http.StatusOK, recorder.Code)

		var got dto.AuthorMeResponse
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &got))
		require.Equal(t, *author, got)
	})

	t.Run("not an author", func(t *testing.T) {
		t.Parallel()

		uc := mock_delivery_http.NewMockAuthorUsecase(t)
		uc.EXPECT().GetAuthor(mock.Anything, userID).Return(nil, authorWrapped(identityuc.ErrAuthorNotFound))

		recorder := httptest.NewRecorder()
		handlers.NewAuthorHandler(uc, testutils.SetupValidator(), nil).GetMeAuthor(
			recorder,
			newAuthorRequest(http.MethodGet, "/authors/me", "", true),
		)

		require.Equal(t, http.StatusNotFound, recorder.Code)
	})

	t.Run("unauthorized", func(t *testing.T) {
		t.Parallel()

		uc := mock_delivery_http.NewMockAuthorUsecase(t)

		recorder := httptest.NewRecorder()
		handlers.NewAuthorHandler(uc, testutils.SetupValidator(), nil).GetMeAuthor(
			recorder,
			newAuthorRequest(http.MethodGet, "/authors/me", "", false),
		)

		require.Equal(t, http.StatusUnauthorized, recorder.Code)
	})
}

func TestAuthorHandler_EditAuthor(t *testing.T) {
	t.Parallel()

	userID := testUserID
	category := "Образование"
	author := &dto.AuthorMeResponse{ID: userID, Username: "ivan001", Nickname: "Иван", Bio: "Новое био", Category: &category}

	t.Run("replaces bio and category", func(t *testing.T) {
		t.Parallel()

		uc := mock_delivery_http.NewMockAuthorUsecase(t)
		uc.EXPECT().UpdateAuthor(mock.Anything, userID, dto.AuthorCreateRequest{Bio: "Новое био", Category: &category}).Return(author, nil)

		recorder := httptest.NewRecorder()
		handlers.NewAuthorHandler(uc, testutils.SetupValidator(), testutils.SetupLogger()).EditAuthor(
			recorder,
			newAuthorRequest(http.MethodPut, "/authors/me", `{"bio":"Новое био","category":"Образование"}`, true),
		)

		require.Equal(t, http.StatusOK, recorder.Code)

		var got dto.AuthorMeResponse
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &got))
		require.Equal(t, *author, got)
	})

	t.Run("empty bio", func(t *testing.T) {
		t.Parallel()

		uc := mock_delivery_http.NewMockAuthorUsecase(t)

		recorder := httptest.NewRecorder()
		handlers.NewAuthorHandler(uc, testutils.SetupValidator(), testutils.SetupLogger()).EditAuthor(
			recorder,
			newAuthorRequest(http.MethodPut, "/authors/me", `{"bio":""}`, true),
		)

		require.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("not an author", func(t *testing.T) {
		t.Parallel()

		uc := mock_delivery_http.NewMockAuthorUsecase(t)
		uc.EXPECT().UpdateAuthor(mock.Anything, userID, dto.AuthorCreateRequest{Bio: "Новое био"}).
			Return(nil, authorWrapped(identityuc.ErrAuthorNotFound))

		recorder := httptest.NewRecorder()
		handlers.NewAuthorHandler(uc, testutils.SetupValidator(), testutils.SetupLogger()).EditAuthor(
			recorder,
			newAuthorRequest(http.MethodPut, "/authors/me", `{"bio":"Новое био"}`, true),
		)

		require.Equal(t, http.StatusNotFound, recorder.Code)
	})
}
