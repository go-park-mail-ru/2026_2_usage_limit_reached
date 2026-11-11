package middleware_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/csrf"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/jwt"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type MockCSRFVerifier struct {
	BindingFunc func(in csrf.BindingInput) (string, bool)
	VerifyFunc  func(binding, token string) bool

	BindingCalls []csrf.BindingInput
	VerifyCalls  []struct{ Binding, Token string }
}

func (m *MockCSRFVerifier) Binding(in csrf.BindingInput) (string, bool) {
	m.BindingCalls = append(m.BindingCalls, in)
	if m.BindingFunc != nil {
		return m.BindingFunc(in)
	}
	return "", false
}

func (m *MockCSRFVerifier) Verify(binding, token string) bool {
	m.VerifyCalls = append(m.VerifyCalls, struct{ Binding, Token string }{binding, token})
	if m.VerifyFunc != nil {
		return m.VerifyFunc(binding, token)
	}
	return false
}

func TestCSRFMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		token          string
		binding        func(in csrf.BindingInput) (string, bool)
		verify         func(binding, token string) bool
		expectedStatus int
		expectNextCall bool
		expectVerify   bool
	}{
		{name: "GET is not checked", method: http.MethodGet, expectedStatus: http.StatusOK, expectNextCall: true},
		{name: "HEAD is not checked", method: http.MethodHead, expectedStatus: http.StatusOK, expectNextCall: true},
		{name: "OPTIONS is not checked", method: http.MethodOptions, expectedStatus: http.StatusOK, expectNextCall: true},
		{
			name:           "POST without binding",
			method:         http.MethodPost,
			token:          "token",
			binding:        func(csrf.BindingInput) (string, bool) { return "", false },
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "POST with invalid token",
			method:         http.MethodPost,
			token:          "bad-token",
			binding:        func(csrf.BindingInput) (string, bool) { return "auth:jwt", true },
			verify:         func(string, string) bool { return false },
			expectedStatus: http.StatusForbidden,
			expectVerify:   true,
		},
		{
			name:           "POST without token header",
			method:         http.MethodPost,
			binding:        func(csrf.BindingInput) (string, bool) { return "auth:jwt", true },
			verify:         func(_, token string) bool { return token != "" },
			expectedStatus: http.StatusForbidden,
			expectVerify:   true,
		},
		{
			name:           "POST with valid token",
			method:         http.MethodPost,
			token:          "good-token",
			binding:        func(csrf.BindingInput) (string, bool) { return "auth:jwt", true },
			verify:         func(binding, token string) bool { return binding == "auth:jwt" && token == "good-token" },
			expectedStatus: http.StatusOK,
			expectNextCall: true,
			expectVerify:   true,
		},
		{
			name:           "PUT is checked",
			method:         http.MethodPut,
			token:          "bad-token",
			binding:        func(csrf.BindingInput) (string, bool) { return "auth:jwt", true },
			verify:         func(string, string) bool { return false },
			expectedStatus: http.StatusForbidden,
			expectVerify:   true,
		},
		{
			name:           "PATCH is checked",
			method:         http.MethodPatch,
			token:          "bad-token",
			binding:        func(csrf.BindingInput) (string, bool) { return "auth:jwt", true },
			verify:         func(string, string) bool { return false },
			expectedStatus: http.StatusForbidden,
			expectVerify:   true,
		},
		{
			name:           "DELETE is checked",
			method:         http.MethodDelete,
			token:          "bad-token",
			binding:        func(csrf.BindingInput) (string, bool) { return "auth:jwt", true },
			verify:         func(string, string) bool { return false },
			expectedStatus: http.StatusForbidden,
			expectVerify:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verifier := &MockCSRFVerifier{BindingFunc: tt.binding, VerifyFunc: tt.verify}
			nextCalled := false
			handler := middleware.CSRFMiddleware(verifier)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(tt.method, "/", nil)
			if tt.token != "" {
				req.Header.Set(middleware.CSRFHeaderName, tt.token)
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			require.Equal(t, tt.expectedStatus, w.Code)
			require.Equal(t, tt.expectNextCall, nextCalled)
			require.Equal(t, tt.expectVerify, len(verifier.VerifyCalls) == 1)

			if tt.expectedStatus == http.StatusForbidden {
				var errResp response.ErrorResponse
				require.NoError(t, json.NewDecoder(w.Body).Decode(&errResp))
				require.Equal(t, "csrf_failed", errResp.Error)
			}
			if tt.expectVerify && tt.token != "" {
				require.Equal(t, tt.token, verifier.VerifyCalls[0].Token)
			}
		})
	}
}

func TestCSRFMiddleware_BindingInput(t *testing.T) {
	validPayload, err := json.Marshal(jwt.UserPayload{UserID: uuid.New(), Username: "user", Email: "email@example.com", Role: "user"})
	require.NoError(t, err)

	validVerifier := &MockTokenVerifier{VerifyFunc: func(string) (json.RawMessage, error) {
		return validPayload, nil
	}}
	invalidVerifier := &MockTokenVerifier{VerifyFunc: func(string) (json.RawMessage, error) {
		return nil, errors.New("token verification failed")
	}}

	jwtCookie := &http.Cookie{Name: jwt.CookieName, Value: "jwt-token"}
	ctxCookie := &http.Cookie{Name: csrf.ContextCookieName, Value: "anon-context"}

	tests := []struct {
		name     string
		cookies  []*http.Cookie
		verifier middleware.TokenVerifier
		expected csrf.BindingInput
	}{
		{
			name:     "No cookies",
			verifier: validVerifier,
			expected: csrf.BindingInput{},
		},
		{
			name:     "Anon context only",
			cookies:  []*http.Cookie{ctxCookie},
			verifier: validVerifier,
			expected: csrf.BindingInput{AnonContext: "anon-context"},
		},
		{
			name:     "Verified jwt becomes session token",
			cookies:  []*http.Cookie{jwtCookie},
			verifier: validVerifier,
			expected: csrf.BindingInput{SessionToken: "jwt-token"},
		},
		{
			name:     "Verified jwt and anon context",
			cookies:  []*http.Cookie{jwtCookie, ctxCookie},
			verifier: validVerifier,
			expected: csrf.BindingInput{SessionToken: "jwt-token", AnonContext: "anon-context"},
		},
		{
			name:     "Unverified jwt is ignored",
			cookies:  []*http.Cookie{jwtCookie, ctxCookie},
			verifier: invalidVerifier,
			expected: csrf.BindingInput{AnonContext: "anon-context"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verifier := &MockCSRFVerifier{}
			handler := middleware.OptionalAuthMiddleware(tt.verifier)(
				middleware.CSRFMiddleware(verifier)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				})),
			)

			req := httptest.NewRequest(http.MethodPost, "/", nil)
			for _, c := range tt.cookies {
				req.AddCookie(c)
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			require.Equal(t, http.StatusForbidden, w.Code)
			require.Equal(t, []csrf.BindingInput{tt.expected}, verifier.BindingCalls)
		})
	}
}
