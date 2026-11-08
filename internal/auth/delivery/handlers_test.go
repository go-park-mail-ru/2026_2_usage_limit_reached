package handlers

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

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/delivery/mock_auth_delivery"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/testutils"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func setupTestHandler(uc UseCase) *Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	val := testutils.SetupValidator()
	return NewHandler(uc, val, logger)
}

func TestHandler_Register(t *testing.T) {
	validReq := dto.RegistrationRequest{
		Email:    "test@mail.ru",
		Username: "testuser",
		Nickname: "Test Nick",
		Password: "password123",
	}
	validBody, err := json.Marshal(validReq)
	require.NoError(t, err)

	invalidReq := validReq
	invalidReq.Password = "123"
	invalidBody, err := json.Marshal(invalidReq)
	require.NoError(t, err)

	tests := []struct {
		name           string
		body           []byte
		mockBehavior   func(m *mock_auth_delivery.MockUseCase)
		expectedStatus int
		expectedErrMsg string
		expectedToken  string
	}{
		{
			name: "Success",
			body: validBody,
			mockBehavior: func(m *mock_auth_delivery.MockUseCase) {
				m.EXPECT().Register(mock.Anything, validReq).Return(&dto.UserResponse{Username: "username", Nickname: "nick"}, "mock-token", nil)
				m.EXPECT().TokenTTL().Return(24 * time.Hour)
			},
			expectedStatus: http.StatusOK,
			expectedToken:  "mock-token",
		},
		{
			name:           "Decode Error (Bad JSON)",
			body:           []byte(`{bad-json`),
			mockBehavior:   func(m *mock_auth_delivery.MockUseCase) {},
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: "bad request",
		},
		{
			name:           "Validation Error",
			body:           invalidBody,
			mockBehavior:   func(m *mock_auth_delivery.MockUseCase) {},
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: "bad request",
		},
		{
			name: "Usecase Error - User Exists (Conflict)",
			body: validBody,
			mockBehavior: func(m *mock_auth_delivery.MockUseCase) {
				m.EXPECT().Register(mock.Anything, validReq).Return(nil, "", usecase.ErrRegistrationFailed)
			},
			expectedStatus: http.StatusConflict,
			expectedErrMsg: "user already exist",
		},
		{
			name: "Internal Server Error",
			body: validBody,
			mockBehavior: func(m *mock_auth_delivery.MockUseCase) {
				m.EXPECT().Register(mock.Anything, validReq).Return(nil, "", errors.New("internal server error"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedErrMsg: "internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUC := mock_auth_delivery.NewMockUseCase(t)
			tt.mockBehavior(mockUC)

			h := setupTestHandler(mockUC)

			req := httptest.NewRequest(http.MethodPost, "/signup", bytes.NewReader(tt.body))
			w := httptest.NewRecorder()

			h.Register(w, req)

			require.Equal(t, tt.expectedStatus, w.Code)

			switch {
			case tt.expectedErrMsg != "":
				var errResp response.ErrorResponse
				require.NoError(t, json.NewDecoder(w.Body).Decode(&errResp))
				require.Equal(t, tt.expectedErrMsg, errResp.Error)

			case tt.expectedToken != "":
				cookies := w.Result().Cookies()
				require.Len(t, cookies, 1)
				require.Equal(t, "__Host-token", cookies[0].Name)
				require.Equal(t, tt.expectedToken, cookies[0].Value)
				require.True(t, cookies[0].Secure)

				require.JSONEq(t, `{"username":"username","nickname":"nick"}`, w.Body.String())
			}
		})
	}
}

func TestHandler_Login(t *testing.T) {
	validReq := dto.LoginRequest{
		Login:    "test@mail.ru",
		Password: "password123",
	}
	validBody, err := json.Marshal(validReq)
	require.NoError(t, err)

	invalidReq := validReq
	invalidReq.Login = ""
	invalidBody, err := json.Marshal(invalidReq)
	require.NoError(t, err)

	tests := []struct {
		name           string
		body           []byte
		mockBehavior   func(m *mock_auth_delivery.MockUseCase)
		expectedStatus int
		expectedErrMsg string
		expectedToken  string
	}{
		{
			name: "Success",
			body: validBody,
			mockBehavior: func(m *mock_auth_delivery.MockUseCase) {
				m.EXPECT().Login(mock.Anything, validReq).Return(&dto.UserResponse{Username: "username", Nickname: "nick"}, "mock-token", nil)
				m.EXPECT().TokenTTL().Return(24 * time.Hour)
			},
			expectedStatus: http.StatusOK,
			expectedToken:  "mock-token",
		},
		{
			name:           "Decode Error (Bad JSON)",
			body:           []byte(`{bad-json`),
			mockBehavior:   func(m *mock_auth_delivery.MockUseCase) {},
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: "bad request",
		},
		{
			name:           "Validation Error",
			body:           invalidBody,
			mockBehavior:   func(m *mock_auth_delivery.MockUseCase) {},
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: "bad request",
		},
		{
			name: "Usecase Error - Unauthorized",
			body: validBody,
			mockBehavior: func(m *mock_auth_delivery.MockUseCase) {
				m.EXPECT().Login(mock.Anything, validReq).Return(nil, "", usecase.ErrLoginFailed)
			},
			expectedStatus: http.StatusUnauthorized,
			expectedErrMsg: "unauthorized",
		},
		{
			name: "Internal Server Error",
			body: validBody,
			mockBehavior: func(m *mock_auth_delivery.MockUseCase) {
				m.EXPECT().Login(mock.Anything, validReq).Return(nil, "", errors.New("db disconnect"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedErrMsg: "internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUC := mock_auth_delivery.NewMockUseCase(t)
			tt.mockBehavior(mockUC)

			h := setupTestHandler(mockUC)

			req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(tt.body))
			w := httptest.NewRecorder()

			h.Login(w, req)

			require.Equal(t, tt.expectedStatus, w.Code)

			switch {
			case tt.expectedErrMsg != "":
				var errResp response.ErrorResponse
				require.NoError(t, json.NewDecoder(w.Body).Decode(&errResp))
				require.Equal(t, tt.expectedErrMsg, errResp.Error)

			case tt.expectedToken != "":
				cookies := w.Result().Cookies()
				require.Len(t, cookies, 1)
				require.Equal(t, "__Host-token", cookies[0].Name)
				require.Equal(t, tt.expectedToken, cookies[0].Value)
				require.True(t, cookies[0].Secure)

				require.JSONEq(t, `{"username":"username","nickname":"nick"}`, w.Body.String())
			}
		})
	}
}

func TestHandler_Logout(t *testing.T) {
	mockUC := mock_auth_delivery.NewMockUseCase(t)
	h := setupTestHandler(mockUC)

	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	w := httptest.NewRecorder()

	h.Logout(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	cookies := w.Result().Cookies()
	require.Len(t, cookies, 1)

	cookie := cookies[0]
	require.Equal(t, "__Host-token", cookie.Name)
	require.Empty(t, cookie.Value)
	require.True(t, cookie.Secure)
	require.Equal(t, -1, cookie.MaxAge)
}

func TestHandler_RegisterRoutes(t *testing.T) {
	h := setupTestHandler(mock_auth_delivery.NewMockUseCase(t))

	publicRouter := mux.NewRouter()
	privateRouter := mux.NewRouter()

	h.RegisterRoutes(publicRouter, privateRouter)

	routes := []string{"/signup", "/login"}
	for _, route := range routes {
		req := httptest.NewRequest(http.MethodPost, route, nil)
		var match mux.RouteMatch
		require.True(t, publicRouter.Match(req, &match), "expected %s route to be registered in public router", route)
	}

	reqLogout := httptest.NewRequest(http.MethodPost, "/logout", nil)
	var match mux.RouteMatch
	require.True(t, privateRouter.Match(reqLogout, &match), "expected /logout route to be registered in private router")
}
