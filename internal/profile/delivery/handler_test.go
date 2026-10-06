package profilehandlers_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	authdto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/dto"
	authmodels "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/models"
	authmemory "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/repository/memory"
	authuc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/usecase"
	profilehandlers "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/delivery"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/models"
	profilememory "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/repository/memory"
	profileusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/jwt"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

var (
	deliveryUnknownID = uuid.MustParse("00000000-0000-4000-8000-000000000099")
	deliveryPostID    = uuid.MustParse("00000000-0000-4000-8000-000000000101")
	deliveryTime      = time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
)

type errorUserProvider struct{ err error }

func (s errorUserProvider) FindUserByID(context.Context, uuid.UUID) (*authdto.UserInfo, error) {
	return nil, s.err
}

type errorProfileRepository struct {
	authorErr error
	postsErr  error
}

func (s errorProfileRepository) GetAuthorByUserID(context.Context, uuid.UUID) (*models.Author, error) {
	return nil, s.authorErr
}

func (s errorProfileRepository) ListPostsByAuthorID(context.Context, uuid.UUID) ([]models.Post, error) {
	return nil, s.postsErr
}

func newDeliveryRepositories(t *testing.T, withAuthor, withPost bool) (profileusecase.UserInfoProvider, profileusecase.ProfileRepository, uuid.UUID) {
	t.Helper()

	users := authmemory.NewUserRepository()
	user, err := users.CreateUser(context.Background(), &authmodels.User{
		Username: "ivan001", Nickname: "Иван", Email: "ivan@example.test",
		AvatarKey: "avatars/ivan.png", Status: "active", CreatedAt: deliveryTime, UpdatedAt: deliveryTime,
	})
	require.NoError(t, err)

	authors := map[uuid.UUID]models.Author{}
	if withAuthor {
		authors[user.ID] = models.Author{Bio: "Пишу о книгах", Category: "Книги"}
	}
	var posts []models.Post
	if withPost {
		posts = []models.Post{{
			ID: deliveryPostID, AuthorID: user.ID, Title: "Первый пост", Body: "Текст",
			Status: models.PostStatusPublished, PublishedAt: &deliveryTime, CreatedAt: deliveryTime,
		}}
	}

	profiles, err := profilememory.NewProfileRepositoryWithMockData(authors, posts)
	require.NoError(t, err)
	return authuc.NewUsecase(users, nil), profiles, user.ID
}

func newDeliveryRouter(t *testing.T, users profileusecase.UserInfoProvider, profiles profileusecase.ProfileRepository) (http.Handler, *jwt.JWTManager) {
	t.Helper()

	tokens, err := jwt.NewJWTManager(&jwt.JWTConfig{Secret: "0123456789abcdef0123456789abcdef", TokenTTL: time.Hour})
	require.NoError(t, err)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := profilehandlers.NewProfileHandler(profileusecase.NewProfileUsecase(users, profiles), logger)
	router := mux.NewRouter()
	private := router.NewRoute().Subrouter()
	private.Use(middleware.AuthMiddleware(tokens))
	handler.RegisterRoutes(router, private)

	return router, tokens
}

func signedDeliveryToken(t *testing.T, tokens *jwt.JWTManager, userID, payloadID uuid.UUID) string {
	t.Helper()
	token, err := tokens.Generate(userID, jwt.UserPayload{UserID: payloadID, Role: "user"})
	require.NoError(t, err)
	return token
}

func requestProfile(t *testing.T, router http.Handler, path, token string) *httptest.ResponseRecorder {
	t.Helper()
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

	tests := []struct {
		name       string
		withAuthor bool
		want       string
	}{
		{
			name:       "author profile",
			withAuthor: true,
			want:       `{"user":{"id":"%s","username":"ivan001","nickname":"Иван","email":"ivan@example.test","avatar_key":"avatars/ivan.png","created_at":"2026-09-01T12:00:00Z"},"author":{"bio":"Пишу о книгах","category":"Книги"}}`,
		},
		{
			name: "reader profile",
			want: `{"user":{"id":"%s","username":"ivan001","nickname":"Иван","email":"ivan@example.test","avatar_key":"avatars/ivan.png","created_at":"2026-09-01T12:00:00Z"}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			users, profiles, userID := newDeliveryRepositories(t, tt.withAuthor, false)
			router, tokens := newDeliveryRouter(t, users, profiles)
			recorder := requestProfile(t, router, "/profile/me", signedDeliveryToken(t, tokens, userID, userID))
			require.Equal(t, http.StatusOK, recorder.Code)
			require.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
			require.JSONEq(t, fmt.Sprintf(tt.want, userID), recorder.Body.String())
		})
	}
}

func TestProfileHandler_GetMe_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		credential string
		userErr    error
		authorErr  error
		wantStatus int
		wantErr    string
	}{
		{name: "missing token", credential: "missing", wantStatus: http.StatusUnauthorized, wantErr: `{"error":"unauthorized"}`},
		{name: "invalid token", credential: "invalid", wantStatus: http.StatusUnauthorized, wantErr: `{"error":"unauthorized"}`},
		{name: "nil user ID in token", credential: "nil ID", wantStatus: http.StatusUnauthorized, wantErr: `{"error":"unauthorized"}`},
		{name: "unknown user", credential: "unknown", wantStatus: http.StatusNotFound, wantErr: `{"error":"not found"}`},
		{name: "user storage failure", credential: "valid", userErr: errors.New("user storage unavailable"), wantStatus: http.StatusInternalServerError, wantErr: `{"error":"internal server error"}`},
		{name: "author storage failure", credential: "valid", authorErr: errors.New("author storage unavailable"), wantStatus: http.StatusInternalServerError, wantErr: `{"error":"internal server error"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			users, profiles, userID := newDeliveryRepositories(t, false, false)
			if tt.userErr != nil {
				users = errorUserProvider{err: tt.userErr}
			}
			if tt.authorErr != nil {
				profiles = errorProfileRepository{authorErr: tt.authorErr}
			}

			router, tokens := newDeliveryRouter(t, users, profiles)

			var token string
			switch tt.credential {
			case "invalid":
				token = "not-a-token"
			case "nil ID":
				token = signedDeliveryToken(t, tokens, userID, uuid.Nil)
			case "unknown":
				token = signedDeliveryToken(t, tokens, deliveryUnknownID, deliveryUnknownID)
			case "valid":
				token = signedDeliveryToken(t, tokens, userID, userID)
			}

			recorder := requestProfile(t, router, "/profile/me", token)
			require.Equal(t, tt.wantStatus, recorder.Code)
			require.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
			require.JSONEq(t, tt.wantErr, recorder.Body.String())
		})
	}
}

func TestProfileHandler_GetMePosts_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		withPost bool
		want     string
	}{
		{
			name:     "author has a post",
			withPost: true,
			want:     `{"user_id":"%s","posts":[{"id":"00000000-0000-4000-8000-000000000101","title":"Первый пост","body":"Текст","status":"published","published_at":"2026-09-01T12:00:00Z"}]}`,
		},
		{name: "author has no posts", want: `{"user_id":"%s","posts":[]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			users, profiles, userID := newDeliveryRepositories(t, true, tt.withPost)
			router, tokens := newDeliveryRouter(t, users, profiles)
			recorder := requestProfile(t, router, "/profile/me/posts", signedDeliveryToken(t, tokens, userID, userID))

			require.Equal(t, http.StatusOK, recorder.Code)
			require.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
			require.JSONEq(t, fmt.Sprintf(tt.want, userID), recorder.Body.String())
		})
	}
}

func TestProfileHandler_GetMePosts_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		credential string
		postsErr   error
		wantStatus int
		wantErr    string
	}{
		{name: "missing token", credential: "missing", wantStatus: http.StatusUnauthorized, wantErr: `{"error":"unauthorized"}`},
		{name: "invalid token", credential: "invalid", wantStatus: http.StatusUnauthorized, wantErr: `{"error":"unauthorized"}`},
		{name: "nil user ID in token", credential: "nil ID", wantStatus: http.StatusUnauthorized, wantErr: `{"error":"unauthorized"}`},
		{name: "posts storage failure", credential: "valid", postsErr: errors.New("posts storage unavailable"), wantStatus: http.StatusInternalServerError, wantErr: `{"error":"internal server error"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			users, profiles, userID := newDeliveryRepositories(t, true, false)
			if tt.postsErr != nil {
				profiles = errorProfileRepository{postsErr: tt.postsErr}
			}

			router, tokens := newDeliveryRouter(t, users, profiles)

			var token string
			switch tt.credential {
			case "invalid":
				token = "not-a-token"
			case "nil ID":
				token = signedDeliveryToken(t, tokens, userID, uuid.Nil)
			case "valid":
				token = signedDeliveryToken(t, tokens, userID, userID)
			}
			
			recorder := requestProfile(t, router, "/profile/me/posts", token)
			require.Equal(t, tt.wantStatus, recorder.Code)
			require.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
			require.JSONEq(t, tt.wantErr, recorder.Body.String())
		})
	}
}
