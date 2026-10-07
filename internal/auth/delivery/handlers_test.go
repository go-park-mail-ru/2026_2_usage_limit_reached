package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/validator"
	"github.com/gorilla/mux"
)

type MockUseCase struct {
	RegisterFunc func(ctx context.Context, regInput dto.RegistrationRequest) (*dto.UserResponse, string, error)
	LoginFunc    func(ctx context.Context, loginInput dto.LoginRequest) (*dto.UserResponse, string, error)
	TokenTTLFunc func() time.Duration
}

func (m *MockUseCase) Register(ctx context.Context, regInput dto.RegistrationRequest) (*dto.UserResponse, string, error) {
	if m.RegisterFunc != nil {
		return m.RegisterFunc(ctx, regInput)
	}
	return nil, "", nil
}

func (m *MockUseCase) Login(ctx context.Context, loginInput dto.LoginRequest) (*dto.UserResponse, string, error) {
	if m.LoginFunc != nil {
		return m.LoginFunc(ctx, loginInput)
	}
	return nil, "", nil
}

func (m *MockUseCase) TokenTTL() time.Duration {
	if m.TokenTTLFunc != nil {
		return m.TokenTTLFunc()
	}
	return 24 * time.Hour
}

func setupValidator() *validator.Validator {
	allowedRunes := make(map[rune]struct{})
	for _, r := range "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_" {
		allowedRunes[r] = struct{}{}
	}

	cfg := &validator.ValidationConfig{
		MinUsernameLen:       3,
		MaxUsernameLen:       32,
		MaxNicknameLen:       64,
		MinPasswordLen:       8,
		MaxPasswordLen:       72,
		MaxEmailLen:          254,
		EmailRegexp:          regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`),
		UsernameAllowedRunes: allowedRunes,
	}
	return validator.NewValidator(cfg)
}

func setupTestHandler(uc UseCase) *Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	val := setupValidator()
	return NewHandler(uc, val, logger)
}

func TestHandler_Register(t *testing.T) {
	validReq := dto.RegistrationRequest{
		Email:    "test@mail.ru",
		Username: "testuser",
		Nickname: "Test Nick",
		Password: "password123",
	}
	validBody, _ := json.Marshal(validReq)

	invalidReq := validReq
	invalidReq.Password = "123"
	invalidBody, _ := json.Marshal(invalidReq)

	tests := []struct {
		name           string
		body           []byte
		mockBehavior   func(m *MockUseCase)
		expectedStatus int
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
		},
		{
			name:           "Decode Error (Bad JSON)",
			body:           []byte(`{bad-json`),
			mockBehavior:   func(m *MockUseCase) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Validation Error",
			body:           invalidBody,
			mockBehavior:   func(m *MockUseCase) {},
			expectedStatus: http.StatusBadRequest,
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

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				// Проверка установки куки
				cookies := w.Result().Cookies()
				if len(cookies) == 0 {
					t.Fatalf("expected cookie to be set")
				}
				if cookies[0].Name != "token" || cookies[0].Value != "mock-token" {
					t.Errorf("expected token=mock-token, got %s=%s", cookies[0].Name, cookies[0].Value)
				}
				
				// Проверка тела ответа
				var respBody dto.UserResponse
				err := json.NewDecoder(w.Body).Decode(&respBody)
				if err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if respBody.Email != "test@mail.ru" {
					t.Errorf("expected email test@mail.ru in response, got %s", respBody.Email)
				}
			}
		})
	}
}

func TestHandler_Login(t *testing.T) {
	validReq := dto.LoginRequest{
		Login:    "test@mail.ru",
		Password: "password123",
	}
	validBody, _ := json.Marshal(validReq)

	invalidReq := validReq
	invalidReq.Login = ""
	invalidBody, _ := json.Marshal(invalidReq)

	tests := []struct {
		name           string
		body           []byte
		mockBehavior   func(m *MockUseCase)
		expectedStatus int
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
		},
		{
			name:           "Decode Error (Bad JSON)",
			body:           []byte(`{bad-json`),
			mockBehavior:   func(m *MockUseCase) {},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Validation Error",
			body:           invalidBody,
			mockBehavior:   func(m *MockUseCase) {},
			expectedStatus: http.StatusBadRequest,
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

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				cookies := w.Result().Cookies()
				if len(cookies) == 0 {
					t.Fatalf("expected cookie to be set")
				}
				if cookies[0].Name != "token" || cookies[0].Value != "mock-token" {
					t.Errorf("expected token=mock-token, got %s=%s", cookies[0].Name, cookies[0].Value)
				}
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

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("expected cookie to be modified")
	}

	cookie := cookies[0]
	if cookie.Name != "token" {
		t.Errorf("expected cookie name 'token', got %s", cookie.Name)
	}
	if cookie.Value != "" {
		t.Errorf("expected empty cookie value, got %s", cookie.Value)
	}
	if cookie.MaxAge != -1 {
		t.Errorf("expected MaxAge -1, got %d", cookie.MaxAge)
	}
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
		if !publicRouter.Match(req, &match) {
			t.Errorf("expected %s route to be registered in public router", route)
		}
	}

	reqLogout := httptest.NewRequest(http.MethodPost, "/logout", nil)
	var match mux.RouteMatch
	if !privateRouter.Match(reqLogout, &match) {
		t.Errorf("expected /logout route to be registered in private router")
	}
}