package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/delivery/mock_auth_delivery"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/csrf"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

func setupTestCSRFHandler(m CSRFManager) *CSRFHandler {
	return NewCSRFHandler(m, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestCSRFHandler_GetToken(t *testing.T) {
	const contextTTL = time.Hour

	tests := []struct {
		name            string
		cookie          *http.Cookie
		mockBehavior    func(m *mock_auth_delivery.MockCSRFManager)
		expectedStatus  int
		expectedToken   string
		expectedErrMsg  string
		expectedContext string
	}{
		{
			name: "New anonymous context sets cookie",
			mockBehavior: func(m *mock_auth_delivery.MockCSRFManager) {
				m.EXPECT().Resolve(csrf.BindingInput{}).Return("anon:new-context", "new-context", nil)
				m.EXPECT().Token("anon:new-context").Return("token", nil)
				m.EXPECT().ContextTTL().Return(contextTTL)
			},
			expectedStatus:  http.StatusOK,
			expectedToken:   "token",
			expectedContext: "new-context",
		},
		{
			name:   "Existing context from cookie is passed to manager and not reissued",
			cookie: &http.Cookie{Name: csrf.ContextCookieName, Value: "old-context"},
			mockBehavior: func(m *mock_auth_delivery.MockCSRFManager) {
				m.EXPECT().Resolve(csrf.BindingInput{AnonContext: "old-context"}).Return("anon:old-context", "", nil)
				m.EXPECT().Token("anon:old-context").Return("token", nil)
			},
			expectedStatus: http.StatusOK,
			expectedToken:  "token",
		},
		{
			name: "Resolve error",
			mockBehavior: func(m *mock_auth_delivery.MockCSRFManager) {
				m.EXPECT().Resolve(csrf.BindingInput{}).Return("", "", errors.New("rand failed"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedErrMsg: "internal server error",
		},
		{
			name:   "Token error",
			cookie: &http.Cookie{Name: csrf.ContextCookieName, Value: "old-context"},
			mockBehavior: func(m *mock_auth_delivery.MockCSRFManager) {
				m.EXPECT().Resolve(csrf.BindingInput{AnonContext: "old-context"}).Return("anon:old-context", "", nil)
				m.EXPECT().Token("anon:old-context").Return("", errors.New("rand failed"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedErrMsg: "internal server error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := mock_auth_delivery.NewMockCSRFManager(t)
			tt.mockBehavior(m)
			h := setupTestCSRFHandler(m)

			req := httptest.NewRequest(http.MethodGet, "/csrf", nil)
			if tt.cookie != nil {
				req.AddCookie(tt.cookie)
			}
			w := httptest.NewRecorder()

			h.GetToken(w, req)

			require.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedErrMsg != "" {
				var errResp response.ErrorResponse
				require.NoError(t, json.NewDecoder(w.Body).Decode(&errResp))
				require.Equal(t, tt.expectedErrMsg, errResp.Error)
				return
			}

			require.Equal(t, "no-store", w.Header().Get("Cache-Control"))

			var resp dto.CSRFTokenResponse
			require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
			require.Equal(t, tt.expectedToken, resp.CSRFToken)

			cookies := w.Result().Cookies()
			if tt.expectedContext == "" {
				require.Empty(t, cookies)
				return
			}

			require.Len(t, cookies, 1)
			cookie := cookies[0]
			require.Equal(t, csrf.ContextCookieName, cookie.Name)
			require.Equal(t, tt.expectedContext, cookie.Value)
			require.True(t, cookie.HttpOnly)
			require.True(t, cookie.Secure)
			require.Equal(t, "/", cookie.Path)
			require.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
			require.Equal(t, int(contextTTL.Seconds()), cookie.MaxAge)
		})
	}
}

func TestCSRFHandler_RegisterRoutes(t *testing.T) {
	h := setupTestCSRFHandler(mock_auth_delivery.NewMockCSRFManager(t))
	router := mux.NewRouter()
	h.RegisterRoutes(router)

	tests := []struct {
		method   string
		expected bool
	}{
		{method: http.MethodGet, expected: true},
		{method: http.MethodOptions, expected: true},
		{method: http.MethodPost, expected: false},
		{method: http.MethodPut, expected: false},
		{method: http.MethodDelete, expected: false},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			var match mux.RouteMatch

			require.Equal(t, tt.expected, router.Match(httptest.NewRequest(tt.method, "/csrf", nil), &match))
		})
	}
}
