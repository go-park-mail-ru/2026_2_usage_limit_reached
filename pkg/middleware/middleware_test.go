package middleware_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/jwt"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/google/uuid"
)

type MockTokenVerifier struct {
	VerifyFunc func(tokenString string) (json.RawMessage, error)
}

func (m *MockTokenVerifier) Verify(tokenString string) (json.RawMessage, error) {
	if m.VerifyFunc != nil {
		return m.VerifyFunc(tokenString)
	}
	return nil, nil
}

// Tests for auth mw
func TestAuthMiddleware(t *testing.T) {
	expectedUserID := uuid.New()
	validPayload, _ := json.Marshal(jwt.UserPayload{UserID: expectedUserID})

	tests := []struct {
		name           string
		cookie         *http.Cookie
		mockVerify     func(tokenString string) (json.RawMessage, error)
		expectedStatus int
		expectNextCall bool
	}{
		{
			name: "Success - valid token and payload",
			cookie: &http.Cookie{
				Name:  jwt.CookieName,
				Value: "valid-jwt-token",
			},
			mockVerify: func(tokenString string) (json.RawMessage, error) {
				return json.RawMessage(validPayload), nil
			},
			expectedStatus: http.StatusOK,
			expectNextCall: true,
		},
		{
			name:           "No Cookie in request",
			cookie:         nil,
			mockVerify:     nil,
			expectedStatus: http.StatusUnauthorized,
			expectNextCall: false,
		},
		{
			name: "Verifier returns error (invalid/expired token)",
			cookie: &http.Cookie{
				Name:  jwt.CookieName,
				Value: "bad-token",
			},
			mockVerify: func(tokenString string) (json.RawMessage, error) {
				return nil, errors.New("token verification failed")
			},
			expectedStatus: http.StatusUnauthorized,
			expectNextCall: false,
		},
		{
			name: "Corrupted/Invalid JSON payload",
			cookie: &http.Cookie{
				Name:  jwt.CookieName,
				Value: "token-with-bad-payload",
			},
			mockVerify: func(tokenString string) (json.RawMessage, error) {
				return json.RawMessage(`{not-a-valid-json`), nil
			},
			expectedStatus: http.StatusUnauthorized,
			expectNextCall: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verifier := &MockTokenVerifier{VerifyFunc: tt.mockVerify}
			mw := middleware.AuthMiddleware(verifier)

			nextCalled := false
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true

				extractedID, ok := middleware.UserIDFromContext(r.Context())
				if !ok {
					t.Errorf("expected UserID in context")
				}
				if extractedID != expectedUserID {
					t.Errorf("expected UserID %v, got %v", expectedUserID, extractedID)
				}

				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if tt.cookie != nil {
				req.AddCookie(tt.cookie)
			}
			w := httptest.NewRecorder()

			mw(nextHandler).ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
			if nextCalled != tt.expectNextCall {
				t.Errorf("expected nextHandler called=%v, got %v", tt.expectNextCall, nextCalled)
			}
		})
	}
}

func TestUserIDFromContext_NotFound(t *testing.T) {
	ctx := context.Background()
	id, ok := middleware.UserIDFromContext(ctx)

	if ok {
		t.Errorf("expected ok=false for empty context, got true")
	}
	if id != uuid.Nil {
		t.Errorf("expected uuid.Nil, got %v", id)
	}
}

// Tests for CORS
func TestCORSMiddleware(t *testing.T) {
	cfg := &middleware.CORSConfig{
		AllowedOrigins: map[string]struct{}{
			"http://localhost:3000": {},
			"https://example.com":   {},
		},
		AllowedMethods: "GET, POST, OPTIONS",
		AllowedHeaders: "Content-Type, Authorization",
	}

	mw := middleware.CORSMiddleware(cfg)

	tests := []struct {
		name               string
		method             string
		originHeader       string
		expectedStatus     int
		expectCORSHeaders  bool
		expectNextCalled   bool
	}{
		{
			name:               "Allowed origin - Standard GET request",
			method:             http.MethodGet,
			originHeader:       "http://localhost:3000",
			expectedStatus:     http.StatusOK,
			expectCORSHeaders:  true,
			expectNextCalled:   true,
		},
		{
			name:               "Allowed origin - Preflight OPTIONS request",
			method:             http.MethodOptions,
			originHeader:       "https://example.com",
			expectedStatus:     http.StatusOK,
			expectCORSHeaders:  true,
			expectNextCalled:   false,
		},
		{
			name:               "Disallowed origin - Standard POST request",
			method:             http.MethodPost,
			originHeader:       "http://evil-site.com",
			expectedStatus:     http.StatusOK,
			expectCORSHeaders:  false,
			expectNextCalled:   true,
		},
		{
			name:               "Disallowed origin - Preflight OPTIONS request",
			method:             http.MethodOptions,
			originHeader:       "http://evil-site.com",
			expectedStatus:     http.StatusOK,
			expectCORSHeaders:  false,
			expectNextCalled:   false,
		},
		{
			name:               "No Origin header provided",
			method:             http.MethodGet,
			originHeader:       "",
			expectedStatus:     http.StatusOK,
			expectCORSHeaders:  false,
			expectNextCalled:   true,
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

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if nextCalled != tt.expectNextCalled {
				t.Errorf("expected nextCalled=%v, got %v", tt.expectNextCalled, nextCalled)
			}

			corsOrigin := w.Header().Get("Access-Control-Allow-Origin")
			if tt.expectCORSHeaders {
				if corsOrigin != tt.originHeader {
					t.Errorf("expected Access-Control-Allow-Origin %q, got %q", tt.originHeader, corsOrigin)
				}
				if w.Header().Get("Access-Control-Allow-Credentials") != "true" {
					t.Errorf("expected Access-Control-Allow-Credentials to be 'true'")
				}
				if w.Header().Get("Access-Control-Allow-Methods") != cfg.AllowedMethods {
					t.Errorf("expected methods %q, got %q", cfg.AllowedMethods, w.Header().Get("Access-Control-Allow-Methods"))
				}
				if w.Header().Get("Access-Control-Allow-Headers") != cfg.AllowedHeaders {
					t.Errorf("expected headers %q, got %q", cfg.AllowedHeaders, w.Header().Get("Access-Control-Allow-Headers"))
				}
			} else {
				if corsOrigin != "" {
					t.Errorf("expected no CORS headers, but got origin %q", corsOrigin)
				}
			}
		})
	}
}