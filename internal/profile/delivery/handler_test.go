package profilehandlers_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/testutils"
	profilehandlers "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/delivery"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/delivery/mock_profile_delivery"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/dto"
	profileusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/usecase"
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

type tokenVerifierMock struct {
	userID   uuid.UUID
	email    string
	username string
	role     string
}

func (v tokenVerifierMock) Verify(token string) (json.RawMessage, error) {
	if token != "test-token" {
		return nil, errors.New("invalid test token")
	}
	if v.userID == uuid.Nil {
		return nil, errors.New("nil user id")
	}
	return json.Marshal(jwt.UserPayload{
		UserID:   v.userID,
		Email:    v.email,
		Username: v.username,
		Role:     v.role,
	})
}

func newTestRouter(uc profilehandlers.ProfileUsecase, userID uuid.UUID) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := profilehandlers.NewProfileHandler(uc, testutils.SetupValidator(), logger)
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
	url := "http://localhost:9000/media/avatars/ivan.png"
	avatar := &url
	user := dto.ProfileUserResponse{
		PublicProfileUserResponse: dto.PublicProfileUserResponse{
			ID: testUserID, Username: "ivan001", Nickname: "Иван",
			AvatarURL: avatar, CreatedAt: testTime,
		},
		Email: "ivan@example.test",
	}
	category := "Книги"
	tests := []struct {
		name     string
		response *dto.ProfileResponse
		wantJSON string
	}{
		{
			name:     "author profile",
			response: &dto.ProfileResponse{User: user, Author: &dto.AuthorResponse{Bio: "Пишу о книгах", Category: &category}},
			wantJSON: `{"user":{"id":"00000000-0000-4000-8000-000000000001","username":"ivan001","nickname":"Иван","email":"ivan@example.test","avatar_url":"http://localhost:9000/media/avatars/ivan.png","created_at":"2026-09-01T12:00:00Z"},"author":{"bio":"Пишу о книгах","category":"Книги"}}`,
		},
		{
			name:     "reader profile",
			response: &dto.ProfileResponse{User: user},
			wantJSON: `{"user":{"id":"00000000-0000-4000-8000-000000000001","username":"ivan001","nickname":"Иван","email":"ivan@example.test","avatar_url":"http://localhost:9000/media/avatars/ivan.png","created_at":"2026-09-01T12:00:00Z"}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			uc := mock_profile_delivery.NewMockProfileUsecase(t)
			uc.EXPECT().GetMyProfile(mock.Anything, testUserID).Return(tt.response, nil)
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
		{name: "inactive user", ucErr: profileusecase.ErrUserInactive, wantCode: http.StatusForbidden, wantJSON: `{"error":"forbidden"}`},
		{name: "usecase failure", ucErr: errors.New("user storage unavailable"), wantCode: http.StatusInternalServerError, wantJSON: `{"error":"internal server error"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			uc := mock_profile_delivery.NewMockProfileUsecase(t)
			uc.EXPECT().GetMyProfile(mock.Anything, testUserID).Return(nil, tt.ucErr)
			recorder := requestProfile(newTestRouter(uc, testUserID), "/profile/me", "test-token")
			require.Equal(t, tt.wantCode, recorder.Code)
			require.JSONEq(t, tt.wantJSON, recorder.Body.String())
		})
	}
}

func TestProfileHandler_ErrorLogLevel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		err       error
		wantLevel string
	}{
		{name: "expected error", err: profileusecase.ErrProfileNotFound, wantLevel: "level=INFO"},
		{name: "unexpected error", err: errors.New("storage unavailable"), wantLevel: "level=ERROR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var logs bytes.Buffer

			logger := slog.New(slog.NewTextHandler(&logs, nil))
			uc := mock_profile_delivery.NewMockProfileUsecase(t)
			uc.EXPECT().GetMyProfile(mock.Anything, testUserID).Return(nil, tt.err)

			handler := profilehandlers.NewProfileHandler(uc, testutils.SetupValidator(), logger)
			router := mux.NewRouter()
			private := router.NewRoute().Subrouter()
			private.Use(middleware.AuthMiddleware(tokenVerifierMock{userID: testUserID}))
			handler.RegisterRoutes(router, private)

			requestProfile(router, "/profile/me", "test-token")

			require.Contains(t, logs.String(), tt.wantLevel)
		})
	}
}

func TestProfileHandler_GetByUsername(t *testing.T) {
	t.Parallel()

	public := &dto.PublicProfileResponse{User: dto.PublicProfileUserResponse{ID: testUserID, Username: "ivan001", Nickname: "Иван", CreatedAt: testTime}}
	tests := []struct {
		name       string
		setupMocks func(m *mock_profile_delivery.MockProfileUsecase)
		wantCode   int
		wantJSON   string
	}{
		{
			name: "public profile without email and token",
			setupMocks: func(m *mock_profile_delivery.MockProfileUsecase) {
				m.EXPECT().GetPublicProfile(mock.Anything, "ivan001").Return(public, nil)
			},
			wantCode: http.StatusOK,
			wantJSON: `{"user":{"id":"00000000-0000-4000-8000-000000000001","username":"ivan001","nickname":"Иван","created_at":"2026-09-01T12:00:00Z"}}`,
		},
		{
			name: "not found",
			setupMocks: func(m *mock_profile_delivery.MockProfileUsecase) {
				m.EXPECT().GetPublicProfile(mock.Anything, "ivan001").Return(nil, profileusecase.ErrProfileNotFound)
			},
			wantCode: http.StatusNotFound,
			wantJSON: `{"error":"not found"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc := mock_profile_delivery.NewMockProfileUsecase(t)
			tt.setupMocks(uc)

			recorder := requestProfile(newTestRouter(uc, testUserID), "/users/ivan001", "")

			require.Equal(t, tt.wantCode, recorder.Code)
			require.JSONEq(t, tt.wantJSON, recorder.Body.String())
		})
	}
}

func TestProfileHandler_MeRouteIsNotPublicProfile(t *testing.T) {
	t.Parallel()
	uc := mock_profile_delivery.NewMockProfileUsecase(t)
	recorder := requestProfile(newTestRouter(uc, testUserID), "/profile/me", "")
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
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
		{name: "profile with invalid token", path: "/profile/me", token: "invalid", userID: testUserID},
		{name: "profile with nil user ID", path: "/profile/me", token: "test-token"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := requestProfile(newTestRouter(mock_profile_delivery.NewMockProfileUsecase(t), tt.userID), tt.path, tt.token)
			require.Equal(t, http.StatusUnauthorized, recorder.Code)
			require.JSONEq(t, `{"error":"unauthorized"}`, recorder.Body.String())
		})
	}
}

func TestProfileHandler_RegisterRoutes(t *testing.T) {
	t.Parallel()
	router := newTestRouter(mock_profile_delivery.NewMockProfileUsecase(t), testUserID)

	for _, path := range []string{"/profile/me"} {
		req := httptest.NewRequest(http.MethodPost, path, nil)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusMethodNotAllowed, recorder.Code, path)
	}
}
