package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

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
		mockBehavior   func(m *MockUseCase)
		expectedStatus int
		expectedErrMsg string
		expectedBody   *dto.UserResponse
		expectedToken  string
	}{
		{
			name: "Success",
			body: validBody,
			mockBehavior: func(m *MockUseCase) {
				m.RegisterFunc = func(ctx context.Context, regInput dto.RegistrationRequest) (*dto.UserResponse, string, error) {
					return &dto.UserResponse{Email: "test@mail.ru", Username: "testuser"}, "mock-token", nil
				}
			},
			expectedStatus: http.StatusOK,
			expectedBody:   &dto.UserResponse{Email: "test@mail.ru", Username: "testuser"},
			expectedToken:  "mock-token",
		},
		{
			name:           "Decode Error (Bad JSON)",
			body:           []byte(`{bad-json`),
			mockBehavior:   func(m *MockUseCase) {},
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: "bad request",
		},
		{
			name:           "Validation Error",
			body:           invalidBody,
			mockBehavior:   func(m *MockUseCase) {},
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: "bad request",
		},
		{
			name: "Usecase Error - User Exists (Conflict)",
			body: validBody,
			mockBehavior: func(m *MockUseCase) {
				m.RegisterFunc = func(ctx context.Context, regInput dto.RegistrationRequest) (*dto.UserResponse, string, error) {
					return nil, "", usecase.ErrRegistrationFailed
				}
			},
			expectedStatus: http.StatusConflict,
			expectedErrMsg: "user already exist",
		},
		{
			name: "Internal Server Error",
			body: validBody,
			mockBehavior: func(m *MockUseCase) {
				m.RegisterFunc = func(ctx context.Context, regInput dto.RegistrationRequest) (*dto.UserResponse, string, error) {
					return nil, "", errors.New("internal server error")
				}
			},
			expectedStatus: http.StatusInternalServerError,
			expectedErrMsg: "internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUC := &MockUseCase{}
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

			case tt.expectedBody != nil:
				cookies := w.Result().Cookies()
				require.Len(t, cookies, 1)
				require.Equal(t, "token", cookies[0].Name)
				require.Equal(t, tt.expectedToken, cookies[0].Value)

				var respBody dto.UserResponse
				require.NoError(t, json.NewDecoder(w.Body).Decode(&respBody))
				require.Equal(t, *tt.expectedBody, respBody)
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
		mockBehavior   func(m *MockUseCase)
		expectedStatus int
		expectedErrMsg string
		expectedBody   *dto.UserResponse
		expectedToken  string
	}{
		{
			name: "Success",
			body: validBody,
			mockBehavior: func(m *MockUseCase) {
				m.LoginFunc = func(ctx context.Context, loginInput dto.LoginRequest) (*dto.UserResponse, string, error) {
					return &dto.UserResponse{Email: "test@mail.ru"}, "mock-token", nil
				}
			},
			expectedStatus: http.StatusOK,
			expectedBody:   &dto.UserResponse{Email: "test@mail.ru"},
			expectedToken:  "mock-token",
		},
		{
			name:           "Decode Error (Bad JSON)",
			body:           []byte(`{bad-json`),
			mockBehavior:   func(m *MockUseCase) {},
			expectedStatus: http.StatusUnauthorized,
			expectedErrMsg: "unauthorized",
		},
		{
			name:           "Validation Error",
			body:           invalidBody,
			mockBehavior:   func(m *MockUseCase) {},
			expectedStatus: http.StatusBadRequest,
			expectedErrMsg: "bad request",
		},
		{
			name: "Usecase Error - Unauthorized",
			body: validBody,
			mockBehavior: func(m *MockUseCase) {
				m.LoginFunc = func(ctx context.Context, loginInput dto.LoginRequest) (*dto.UserResponse, string, error) {
					return nil, "", usecase.ErrLoginFailed
				}
			},
			expectedStatus: http.StatusUnauthorized,
			expectedErrMsg: "unauthorized",
		},
		{
			name: "Internal Server Error",
			body: validBody,
			mockBehavior: func(m *MockUseCase) {
				m.LoginFunc = func(ctx context.Context, loginInput dto.LoginRequest) (*dto.UserResponse, string, error) {
					return nil, "", errors.New("db disconnect")
				}
			},
			expectedStatus: http.StatusInternalServerError,
			expectedErrMsg: "internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockUC := &MockUseCase{}
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

			case tt.expectedBody != nil:
				cookies := w.Result().Cookies()
				require.Len(t, cookies, 1)
				require.Equal(t, "token", cookies[0].Name)
				require.Equal(t, tt.expectedToken, cookies[0].Value)

				var respBody dto.UserResponse
				require.NoError(t, json.NewDecoder(w.Body).Decode(&respBody))
				require.Equal(t, *tt.expectedBody, respBody)
			}
		})
	}
}

func TestHandler_Logout(t *testing.T) {
	mockUC := &MockUseCase{}
	h := setupTestHandler(mockUC)

	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	w := httptest.NewRecorder()

	h.Logout(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	cookies := w.Result().Cookies()
	require.Len(t, cookies, 1)

	cookie := cookies[0]
	require.Equal(t, "token", cookie.Name)
	require.Empty(t, cookie.Value)
	require.Equal(t, -1, cookie.MaxAge)
}

func TestHandler_RegisterRoutes(t *testing.T) {
	h := setupTestHandler(&MockUseCase{})

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
