package profilehandlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	profilehandlers "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/delivery"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/dto"
	profileusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/jwt"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

var (
	testUserID = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	testPostID = uuid.MustParse("00000000-0000-4000-8000-000000000101")
	testTime   = time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
)

type profileUsecaseMock struct {
	profiles   map[uuid.UUID]*dto.ProfileResponse
	posts      map[uuid.UUID]*dto.ProfilePostsResponse
	profileErr error
	postsErr   error
}

func (s profileUsecaseMock) GetMyProfile(_ context.Context, id uuid.UUID) (*dto.ProfileResponse, error) {
	if s.profileErr != nil {
		return nil, s.profileErr
	}
	return s.profiles[id], nil
}

func (s profileUsecaseMock) GetMyPosts(_ context.Context, id uuid.UUID) (*dto.ProfilePostsResponse, error) {
	if s.postsErr != nil {
		return nil, s.postsErr
	}
	return s.posts[id], nil
}

type tokenVerifierMock struct {
	userID uuid.UUID
}

func (v tokenVerifierMock) Verify(token string) (json.RawMessage, error) {
	if token != "test-token" {
		return nil, errors.New("invalid test token")
	}
	return json.Marshal(jwt.UserPayload{UserID: v.userID, Role: "user"})
}

func newTestRouter(uc profilehandlers.ProfileUsecase, userID uuid.UUID) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := profilehandlers.NewProfileHandler(uc, logger)
	router := mux.NewRouter()
	private := router.NewRoute().Subrouter()
	private.Use(middleware.AuthMiddleware(tokenVerifierMock{userID: userID}))
	handler.RegisterRoutes(router, private)
	return router
}

func requestProfile(router http.Handler, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.AddCookie(&http.Cookie{Name: jwt.CookieName, Value: token})
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestProfileHandler_GetMe_Success(t *testing.T) {
	t.Parallel()

	user := dto.ProfileUserResponse{
		ID: testUserID, Username: "ivan001", Nickname: "Иван", Email: "ivan@example.test",
		AvatarKey: "avatars/ivan.png", CreatedAt: testTime,
	}
	tests := []struct {
		name     string
		response *dto.ProfileResponse
		wantJSON string
	}{
		{
			name:     "author profile",
			response: &dto.ProfileResponse{User: user, Author: &dto.AuthorResponse{Bio: "Пишу о книгах", Category: "Книги"}},
			wantJSON: `{"user":{"id":"00000000-0000-4000-8000-000000000001","username":"ivan001","nickname":"Иван","email":"ivan@example.test","avatar_key":"avatars/ivan.png","created_at":"2026-09-01T12:00:00Z"},"author":{"bio":"Пишу о книгах","category":"Книги"}}`,
		},
		{
			name:     "reader profile",
			response: &dto.ProfileResponse{User: user},
			wantJSON: `{"user":{"id":"00000000-0000-4000-8000-000000000001","username":"ivan001","nickname":"Иван","email":"ivan@example.test","avatar_key":"avatars/ivan.png","created_at":"2026-09-01T12:00:00Z"}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			uc := profileUsecaseMock{profiles: map[uuid.UUID]*dto.ProfileResponse{testUserID: tt.response}}
			recorder := requestProfile(newTestRouter(uc, testUserID), "/profile/me", "test-token")
			require.Equal(t, http.StatusOK, recorder.Code)
			require.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
			require.JSONEq(t, tt.wantJSON, recorder.Body.String())
		})
	}
}

func TestProfileHandler_GetMe_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ucErr    error
		wantCode int
		wantJSON string
	}{
		{name: "profile not found", ucErr: profileusecase.ErrProfileNotFound, wantCode: http.StatusNotFound, wantJSON: `{"error":"not found"}`},
		{name: "usecase failure", ucErr: errors.New("user storage unavailable"), wantCode: http.StatusInternalServerError, wantJSON: `{"error":"internal server error"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			uc := profileUsecaseMock{profileErr: tt.ucErr}
			recorder := requestProfile(newTestRouter(uc, testUserID), "/profile/me", "test-token")
			require.Equal(t, tt.wantCode, recorder.Code)
			require.JSONEq(t, tt.wantJSON, recorder.Body.String())
		})
	}
}

func TestProfileHandler_GetMePosts_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		response *dto.ProfilePostsResponse
		wantJSON string
	}{
		{
			name: "author has a post",
			response: &dto.ProfilePostsResponse{UserID: testUserID, Posts: []dto.PostResponse{{
				ID: testPostID, Title: "Первый пост", Body: "Текст", Status: "published", PublishedAt: &testTime,
			}}},
			wantJSON: `{"user_id":"00000000-0000-4000-8000-000000000001","posts":[{"id":"00000000-0000-4000-8000-000000000101","title":"Первый пост","body":"Текст","status":"published","published_at":"2026-09-01T12:00:00Z"}]}`,
		},
		{
			name:     "author has no posts",
			response: &dto.ProfilePostsResponse{UserID: testUserID, Posts: []dto.PostResponse{}},
			wantJSON: `{"user_id":"00000000-0000-4000-8000-000000000001","posts":[]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			uc := profileUsecaseMock{posts: map[uuid.UUID]*dto.ProfilePostsResponse{testUserID: tt.response}}
			recorder := requestProfile(newTestRouter(uc, testUserID), "/profile/me/posts", "test-token")
			require.Equal(t, http.StatusOK, recorder.Code)
			require.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
			require.JSONEq(t, tt.wantJSON, recorder.Body.String())
		})
	}
}

func TestProfileHandler_GetMePosts_Error(t *testing.T) {
	t.Parallel()

	uc := profileUsecaseMock{postsErr: errors.New("posts storage unavailable")}
	recorder := requestProfile(newTestRouter(uc, testUserID), "/profile/me/posts", "test-token")
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.JSONEq(t, `{"error":"internal server error"}`, recorder.Body.String())
}

func TestProfileHandler_Unauthorized(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		path   string
		token  string
		userID uuid.UUID
	}{
		{name: "profile without token", path: "/profile/me", userID: testUserID},
		{name: "posts without token", path: "/profile/me/posts", userID: testUserID},
		{name: "profile with invalid token", path: "/profile/me", token: "invalid", userID: testUserID},
		{name: "profile with nil user ID", path: "/profile/me", token: "test-token"},
		{name: "posts with nil user ID", path: "/profile/me/posts", token: "test-token"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := requestProfile(newTestRouter(profileUsecaseMock{}, tt.userID), tt.path, tt.token)
			require.Equal(t, http.StatusUnauthorized, recorder.Code)
			require.JSONEq(t, `{"error":"unauthorized"}`, recorder.Body.String())
		})
	}
}

func TestProfileHandler_RegisterRoutes(t *testing.T) {
	t.Parallel()

	router := newTestRouter(profileUsecaseMock{}, testUserID)
	for _, path := range []string{"/profile/me", "/profile/me/posts"} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusMethodNotAllowed, recorder.Code, path)
	}
}
