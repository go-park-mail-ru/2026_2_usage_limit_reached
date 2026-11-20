package middleware_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/jwt"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type MockTokenVerifier struct {
	VerifyFunc func(ctx context.Context, token string) (json.RawMessage, error)
}

func (m *MockTokenVerifier) Verify(ctx context.Context, token string) (json.RawMessage, error) {
	if m.VerifyFunc != nil {
		return m.VerifyFunc(ctx, token)
	}

	return nil, middleware.ErrInvalidToken
}

func TestAuthMiddleware(t *testing.T) {
	userID := uuid.New()
	validPayload := json.RawMessage(fmt.Sprintf(`{"user_id":%q,"role":"user"}`, userID.String()))

	tests := []struct {
		name           string
		cookie         *http.Cookie
		mockVerify     func(ctx context.Context, token string) (json.RawMessage, error)
		expectedStatus int
		expectNextCall bool
	}{
		{
			name:   "valid token",
			cookie: &http.Cookie{Name: jwt.CookieName, Value: "valid-jwt-token"},
			mockVerify: func(context.Context, string) (json.RawMessage, error) {
				return validPayload, nil
			},
			expectedStatus: http.StatusOK,
			expectNextCall: true,
		},
		{
			name:           "no cookie",
			cookie:         nil,
			mockVerify:     nil,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:   "invalid token",
			cookie: &http.Cookie{Name: jwt.CookieName, Value: "bad-token"},
			mockVerify: func(context.Context, string) (json.RawMessage, error) {
				return nil, middleware.ErrInvalidToken
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:   "empty payload",
			cookie: &http.Cookie{Name: jwt.CookieName, Value: "empty-token"},
			mockVerify: func(context.Context, string) (json.RawMessage, error) {
				return nil, nil
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:   "payload without user id",
			cookie: &http.Cookie{Name: jwt.CookieName, Value: "no-id-token"},
			mockVerify: func(context.Context, string) (json.RawMessage, error) {
				return json.RawMessage(`{"role":"user"}`), nil
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:   "auth service unavailable",
			cookie: &http.Cookie{Name: jwt.CookieName, Value: "valid-jwt-token"},
			mockVerify: func(context.Context, string) (json.RawMessage, error) {
				return nil, middleware.ErrAuthUnavailable
			},
			expectedStatus: http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw := middleware.AuthMiddleware(&MockTokenVerifier{VerifyFunc: tt.mockVerify})

			nextCalled := false
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true

				payload, ok := middleware.UserInfoFromContext(r.Context())
				require.True(t, ok)
				require.Equal(t, userID, payload.UserID)

				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if tt.cookie != nil {
				req.AddCookie(tt.cookie)
			}
			recorder := httptest.NewRecorder()

			mw(nextHandler).ServeHTTP(recorder, req)

			require.Equal(t, tt.expectedStatus, recorder.Code)
			require.Equal(t, tt.expectNextCall, nextCalled)
		})
	}
}

func TestOptionalAuthMiddleware(t *testing.T) {
	userID := uuid.New()
	validPayload := json.RawMessage(fmt.Sprintf(`{"user_id":%q,"role":"user"}`, userID.String()))

	tests := []struct {
		name       string
		cookie     *http.Cookie
		mockVerify func(ctx context.Context, token string) (json.RawMessage, error)
		expectUser bool
	}{
		{
			name:   "valid token",
			cookie: &http.Cookie{Name: jwt.CookieName, Value: "valid-jwt-token"},
			mockVerify: func(context.Context, string) (json.RawMessage, error) {
				return validPayload, nil
			},
			expectUser: true,
		},
		{
			name:   "invalid token",
			cookie: &http.Cookie{Name: jwt.CookieName, Value: "bad-token"},
			mockVerify: func(context.Context, string) (json.RawMessage, error) {
				return nil, middleware.ErrInvalidToken
			},
		},
		{
			name:       "no cookie",
			cookie:     nil,
			mockVerify: nil,
		},
		{
			name:   "auth service unavailable",
			cookie: &http.Cookie{Name: jwt.CookieName, Value: "valid-jwt-token"},
			mockVerify: func(context.Context, string) (json.RawMessage, error) {
				return nil, middleware.ErrAuthUnavailable
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw := middleware.OptionalAuthMiddleware(&MockTokenVerifier{VerifyFunc: tt.mockVerify})

			var (
				nextCalled bool
				gotUser    *jwt.UserPayload
			)
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				gotUser, _ = middleware.UserInfoFromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/public", nil)
			if tt.cookie != nil {
				req.AddCookie(tt.cookie)
			}
			recorder := httptest.NewRecorder()

			mw(nextHandler).ServeHTTP(recorder, req)

			require.Equal(t, http.StatusOK, recorder.Code)
			require.True(t, nextCalled)

			if tt.expectUser {
				require.NotNil(t, gotUser)
				require.Equal(t, userID, gotUser.UserID)
				return
			}

			require.Nil(t, gotUser)
		})
	}
}

func TestUserFromContext_NotFound(t *testing.T) {
	user, ok := middleware.UserInfoFromContext(context.Background())

	var nilPayload *jwt.UserPayload
	require.False(t, ok)
	require.Equal(t, nilPayload, user, "expected nil, got %v", user)
}

func TestCORSMiddleware(t *testing.T) {
	cfg := middleware.CORSConfig{
		AllowedOrigins: []string{"http://localhost:3000", "https://example.com"},
		AllowedMethods: "GET, POST, OPTIONS",
		AllowedHeaders: "Content-Type, Authorization",
	}

	mw := middleware.CORSMiddleware(cfg)

	tests := []struct {
		name              string
		method            string
		originHeader      string
		expectedStatus    int
		expectCORSHeaders bool
		expectNextCalled  bool
	}{
		{
			name:              "Allowed origin - Standard GET request",
			method:            http.MethodGet,
			originHeader:      "http://localhost:3000",
			expectedStatus:    http.StatusOK,
			expectCORSHeaders: true,
			expectNextCalled:  true,
		},
		{
			name:              "Allowed origin - Preflight OPTIONS request",
			method:            http.MethodOptions,
			originHeader:      "https://example.com",
			expectedStatus:    http.StatusOK,
			expectCORSHeaders: true,
			expectNextCalled:  false,
		},
		{
			name:              "Disallowed origin - Standard POST request",
			method:            http.MethodPost,
			originHeader:      "http://evil-site.com",
			expectedStatus:    http.StatusOK,
			expectCORSHeaders: false,
			expectNextCalled:  true,
		},
		{
			name:              "Disallowed origin - Preflight OPTIONS request",
			method:            http.MethodOptions,
			originHeader:      "http://evil-site.com",
			expectedStatus:    http.StatusOK,
			expectCORSHeaders: false,
			expectNextCalled:  false,
		},
		{
			name:              "No Origin header provided",
			method:            http.MethodGet,
			originHeader:      "",
			expectedStatus:    http.StatusOK,
			expectCORSHeaders: false,
			expectNextCalled:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextCalled := false
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(tt.method, "/", nil)
			if tt.originHeader != "" {
				req.Header.Set("Origin", tt.originHeader)
			}
			w := httptest.NewRecorder()

			mw(nextHandler).ServeHTTP(w, req)

			require.Equal(t, tt.expectedStatus, w.Code)
			require.Equal(t, tt.expectNextCalled, nextCalled)

			corsOrigin := w.Header().Get("Access-Control-Allow-Origin")
			if tt.expectCORSHeaders {
				require.Equal(t, tt.originHeader, corsOrigin, "expected Access-Control-Allow-Origin %q, got %q", tt.originHeader, corsOrigin)
				require.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"), "expected Access-Control-Allow-Credentials to be 'true'")
				require.Equal(t, cfg.AllowedMethods, w.Header().Get("Access-Control-Allow-Methods"), "expected methods %q, got %q", cfg.AllowedMethods, w.Header().Get("Access-Control-Allow-Methods"))
				require.Equal(t, cfg.AllowedHeaders, w.Header().Get("Access-Control-Allow-Headers"), "expected headers %q, got %q", cfg.AllowedHeaders, w.Header().Get("Access-Control-Allow-Headers"))
			} else {
				require.Empty(t, corsOrigin, "expected no CORS headers, but got origin %q", corsOrigin)
			}
		})
	}
}
