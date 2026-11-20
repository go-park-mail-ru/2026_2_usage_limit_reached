package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	handlers "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/delivery/http"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/delivery/http/mock_delivery_http"
	httprouter "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/delivery/http/router"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/profile/dto"
	profileuc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/profile/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/testutils"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/csrf"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/jwt"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	testUserID = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	testTime   = time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
)

type staticVerifier struct {
	userID uuid.UUID
}

func (v staticVerifier) Verify(context.Context, string) (json.RawMessage, error) {
	return json.Marshal(jwt.UserPayload{UserID: v.userID})
}

func newRouter(t *testing.T, uc handlers.ProfileUsecase, userID uuid.UUID) *mux.Router {
	t.Helper()

	logger := testutils.SetupLogger()
	valid := testutils.SetupValidator()

	authHandler := handlers.NewAuthHandler(mock_delivery_http.NewMockAuthUsecase(t), valid, logger)
	profileHandler := handlers.NewProfileHandler(uc, valid, logger)
	csrfHandler := handlers.NewCSRFHandler(csrf.NewManager(csrf.DefaultConfig()), logger)

	router := mux.NewRouter()
	public := router.NewRoute().Subrouter()
	private := router.NewRoute().Subrouter()
	private.Use(middleware.AuthMiddleware(staticVerifier{userID: userID}))

	httprouter.RegisterRoutes(public, private, httprouter.Handlers{
		Auth:    authHandler,
		Profile: profileHandler,
		CSRF:    csrfHandler,
	})

	return router
}

func doRequest(router http.Handler, req *http.Request, authorized bool) *httptest.ResponseRecorder {
	if authorized {
		req.AddCookie(&http.Cookie{Name: jwt.CookieName, Value: "test-token"})
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func profileResponse() *dto.ProfileResponse {
	avatar := "/media/avatars/ivan.png"
	category := "Книги"

	return &dto.ProfileResponse{
		User: dto.ProfileUserResponse{
			PublicProfileUserResponse: dto.PublicProfileUserResponse{
				ID:        testUserID,
				Username:  "ivan001",
				Nickname:  "Иван",
				AvatarURL: &avatar,
				CreatedAt: testTime,
			},
			Email: "ivan@example.test",
		},
		Author: &dto.AuthorResponse{Bio: "Пишу о книгах", Category: &category},
	}
}

func TestGetMe(t *testing.T) {
	t.Parallel()

	uc := mock_delivery_http.NewMockProfileUsecase(t)
	uc.EXPECT().GetMyProfile(mock.Anything, testUserID).Return(profileResponse(), nil)

	recorder := doRequest(newRouter(t, uc, testUserID), httptest.NewRequest(http.MethodGet, "/profile/me", nil), true)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
	require.JSONEq(t, `{
		"user": {"id":"00000000-0000-4000-8000-000000000001","username":"ivan001","nickname":"Иван",
		         "avatar_url":"/media/avatars/ivan.png","created_at":"2026-09-01T12:00:00Z","email":"ivan@example.test"},
		"author": {"bio":"Пишу о книгах","category":"Книги"}
	}`, recorder.Body.String())
}

func TestGetMeErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "not found", err: profileuc.ErrProfileNotFound, status: http.StatusNotFound},
		{name: "inactive user", err: profileuc.ErrUserInactive, status: http.StatusForbidden},
		{name: "internal", err: io.ErrUnexpectedEOF, status: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc := mock_delivery_http.NewMockProfileUsecase(t)
			uc.EXPECT().GetMyProfile(mock.Anything, testUserID).Return(nil, tt.err)

			recorder := doRequest(newRouter(t, uc, testUserID), httptest.NewRequest(http.MethodGet, "/profile/me", nil), true)
			require.Equal(t, tt.status, recorder.Code)
		})
	}
}

func TestGetMeUnauthorized(t *testing.T) {
	t.Parallel()

	recorder := doRequest(newRouter(t, mock_delivery_http.NewMockProfileUsecase(t), testUserID),
		httptest.NewRequest(http.MethodGet, "/profile/me", nil), false)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestGetPublicProfile(t *testing.T) {
	t.Parallel()

	avatar := "/media/avatars/ivan.png"
	category := "Книги"
	response := &dto.PublicProfileResponse{
		User: dto.PublicProfileUserResponse{
			ID:        testUserID,
			Username:  "ivan001",
			Nickname:  "Иван",
			AvatarURL: &avatar,
			CreatedAt: testTime,
		},
		Author: &dto.AuthorResponse{Bio: "Пишу о книгах", Category: &category},
	}

	uc := mock_delivery_http.NewMockProfileUsecase(t)
	uc.EXPECT().GetPublicProfile(mock.Anything, "ivan001").Return(response, nil)

	recorder := doRequest(newRouter(t, uc, testUserID), httptest.NewRequest(http.MethodGet, "/users/ivan001", nil), false)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{
		"user": {"id":"00000000-0000-4000-8000-000000000001","username":"ivan001","nickname":"Иван",
		         "avatar_url":"/media/avatars/ivan.png","created_at":"2026-09-01T12:00:00Z"},
		"author": {"bio":"Пишу о книгах","category":"Книги"}
	}`, recorder.Body.String())
}

func TestGetPublicProfileErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		username string
		err      error
		status   int
	}{
		{name: "empty username", username: "", status: http.StatusBadRequest},
		{name: "not found", username: "ivan001", err: profileuc.ErrProfileNotFound, status: http.StatusNotFound},
		{name: "internal", username: "ivan001", err: io.ErrUnexpectedEOF, status: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc := mock_delivery_http.NewMockProfileUsecase(t)
			if tt.username != "" {
				uc.EXPECT().GetPublicProfile(mock.Anything, tt.username).Return(nil, tt.err)
			}

			req := httptest.NewRequest(http.MethodGet, "/users/"+tt.username, nil)
			req = mux.SetURLVars(req, map[string]string{"username": tt.username})

			handler := handlers.NewProfileHandler(uc, testutils.SetupValidator(), testutils.SetupLogger())
			recorder := httptest.NewRecorder()
			handler.GetPublicProfile(recorder, req)

			require.Equal(t, tt.status, recorder.Code)
		})
	}
}

func TestUpdateMyProfile(t *testing.T) {
	t.Parallel()

	uc := mock_delivery_http.NewMockProfileUsecase(t)
	uc.EXPECT().UpdateMyProfile(mock.Anything, testUserID, mock.Anything).Return(profileResponse(), nil)

	req := httptest.NewRequest(http.MethodPut, "/profile/me", bytes.NewBufferString(`{"nickname":"Иван"}`))
	recorder := doRequest(newRouter(t, uc, testUserID), req, true)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestUpdateMyProfileErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		err  error
		code int
	}{
		{name: "invalid json", body: `{`, code: http.StatusBadRequest},
		{name: "validation", body: `{"nickname":""}`, code: http.StatusBadRequest},
		{name: "not author", body: `{"nickname":"Иван","author":{"bio":"bio"}}`, err: profileuc.ErrNotAuthor, code: http.StatusForbidden},
		{name: "inactive user", body: `{"nickname":"Иван"}`, err: profileuc.ErrUserInactive, code: http.StatusForbidden},
		{name: "not found", body: `{"nickname":"Иван"}`, err: profileuc.ErrProfileNotFound, code: http.StatusNotFound},
		{name: "internal", body: `{"nickname":"Иван"}`, err: io.ErrUnexpectedEOF, code: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc := mock_delivery_http.NewMockProfileUsecase(t)
			if tt.code >= http.StatusForbidden {
				uc.EXPECT().UpdateMyProfile(mock.Anything, testUserID, mock.Anything).Return(nil, tt.err)
			}

			req := httptest.NewRequest(http.MethodPut, "/profile/me", bytes.NewBufferString(tt.body))
			recorder := doRequest(newRouter(t, uc, testUserID), req, true)

			require.Equal(t, tt.code, recorder.Code)
		})
	}
}

func TestBecomeAuthor(t *testing.T) {
	t.Parallel()

	uc := mock_delivery_http.NewMockProfileUsecase(t)
	uc.EXPECT().BecomeAuthor(mock.Anything, testUserID, mock.MatchedBy(func(req dto.UpdateAuthorRequest) bool {
		return req.Bio == "Пишу о книгах"
	})).Return(profileResponse(), nil)

	req := httptest.NewRequest(http.MethodPost, "/profile/me/author", bytes.NewBufferString(`{"bio":"Пишу о книгах","category":"Книги"}`))
	recorder := doRequest(newRouter(t, uc, testUserID), req, true)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestBecomeAuthorErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		err  error
		code int
	}{
		{name: "invalid json", body: `{`, code: http.StatusBadRequest},
		{name: "empty bio", body: `{"bio":""}`, code: http.StatusBadRequest},
		{name: "inactive user", body: `{"bio":"bio"}`, err: profileuc.ErrUserInactive, code: http.StatusForbidden},
		{name: "internal", body: `{"bio":"bio"}`, err: io.ErrUnexpectedEOF, code: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc := mock_delivery_http.NewMockProfileUsecase(t)
			if tt.code >= http.StatusForbidden {
				uc.EXPECT().BecomeAuthor(mock.Anything, testUserID, mock.Anything).Return(nil, tt.err)
			}

			req := httptest.NewRequest(http.MethodPost, "/profile/me/author", bytes.NewBufferString(tt.body))
			recorder := doRequest(newRouter(t, uc, testUserID), req, true)

			require.Equal(t, tt.code, recorder.Code)
		})
	}
}

func multipartAvatar(t *testing.T, field string) (*bytes.Buffer, string) {
	t.Helper()

	buf := &bytes.Buffer{}
	writer := multipart.NewWriter(buf)

	part, err := writer.CreateFormFile(field, "avatar.png")
	require.NoError(t, err)
	_, err = part.Write([]byte("fake-image-bytes"))
	require.NoError(t, err)

	require.NoError(t, writer.Close())

	return buf, writer.FormDataContentType()
}

func TestUploadAvatar(t *testing.T) {
	t.Parallel()

	uc := mock_delivery_http.NewMockProfileUsecase(t)
	uc.EXPECT().UploadAvatar(mock.Anything, testUserID, mock.Anything, mock.Anything).
		Return("/media/avatars/new.png", nil)

	body, contentType := multipartAvatar(t, "avatar")
	req := httptest.NewRequest(http.MethodPut, "/profile/me/avatar", body)
	req.Header.Set("Content-Type", contentType)

	recorder := doRequest(newRouter(t, uc, testUserID), req, true)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"avatar_url":"/media/avatars/new.png"}`, recorder.Body.String())
}

func TestUploadAvatarErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		code int
	}{
		{name: "too large", err: profileuc.ErrAvatarTooLarge, code: http.StatusRequestEntityTooLarge},
		{name: "unsupported type", err: profileuc.ErrUnsupportedAvatarType, code: http.StatusUnsupportedMediaType},
		{name: "not found", err: profileuc.ErrProfileNotFound, code: http.StatusNotFound},
		{name: "internal", err: io.ErrUnexpectedEOF, code: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc := mock_delivery_http.NewMockProfileUsecase(t)
			uc.EXPECT().UploadAvatar(mock.Anything, testUserID, mock.Anything, mock.Anything).Return("", tt.err)

			body, contentType := multipartAvatar(t, "avatar")
			req := httptest.NewRequest(http.MethodPut, "/profile/me/avatar", body)
			req.Header.Set("Content-Type", contentType)

			recorder := doRequest(newRouter(t, uc, testUserID), req, true)
			require.Equal(t, tt.code, recorder.Code)
		})
	}
}

func TestUploadAvatarWithoutFile(t *testing.T) {
	t.Parallel()

	body, contentType := multipartAvatar(t, "not-avatar")
	req := httptest.NewRequest(http.MethodPut, "/profile/me/avatar", body)
	req.Header.Set("Content-Type", contentType)

	recorder := doRequest(newRouter(t, mock_delivery_http.NewMockProfileUsecase(t), testUserID), req, true)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestDeleteAvatar(t *testing.T) {
	t.Parallel()

	uc := mock_delivery_http.NewMockProfileUsecase(t)
	uc.EXPECT().DeleteAvatar(mock.Anything, testUserID).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/profile/me/avatar", nil)
	recorder := doRequest(newRouter(t, uc, testUserID), req, true)

	require.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestDeleteAvatarErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		code int
	}{
		{name: "not found", err: profileuc.ErrProfileNotFound, code: http.StatusNotFound},
		{name: "internal", err: io.ErrUnexpectedEOF, code: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc := mock_delivery_http.NewMockProfileUsecase(t)
			uc.EXPECT().DeleteAvatar(mock.Anything, testUserID).Return(tt.err)

			req := httptest.NewRequest(http.MethodDelete, "/profile/me/avatar", nil)
			recorder := doRequest(newRouter(t, uc, testUserID), req, true)

			require.Equal(t, tt.code, recorder.Code)
		})
	}
}
