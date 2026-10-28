package usecase_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/usecase"
	userdto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/dto"
	userusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/usecase"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestUsecase_Register(t *testing.T) {
	ctx := context.Background()
	validInput := dto.RegistrationRequest{
		Email:    "Test@Mail.ru ",
		Username: " TestUser ",
		Nickname: "Nickname",
		Password: "password123",
	}

	tests := []struct {
		name          string
		input         dto.RegistrationRequest
		setupMocks    func(r *MockUserProvider, tm *MockTokenManager)
		expectedErr   error
		errContains   string
		checkResponse bool
	}{
		{
			name:  "Success",
			input: validInput,
			setupMocks: func(r *MockUserProvider, tm *MockTokenManager) {
				r.CreateUserFunc = func(ctx context.Context, req userdto.CreateUserRequest) (*userdto.UserInfo, error) {
					return &userdto.UserInfo{Email: req.Email, Username: req.Username}, nil
				}
				tm.GenerateFunc = func(ID uuid.UUID, payload any) (string, error) {
					return "jwt-token", nil
				}
			},
			checkResponse: true,
		},
		{
			name: "Bcrypt error (password > 72 bytes)",
			input: dto.RegistrationRequest{
				Password: strings.Repeat("a", 73), // Bcrypt падает с ошибкой на длине > 72 байт
			},
			errContains: "failed to hash password",
		},
		{
			name:  "Repo error: User already exists",
			input: validInput,
			setupMocks: func(r *MockUserProvider, tm *MockTokenManager) {
				r.CreateUserFunc = func(ctx context.Context, req userdto.CreateUserRequest) (*userdto.UserInfo, error) {
					return nil, userusecase.ErrUserExists
				}
			},
			expectedErr: usecase.ErrRegistrationFailed,
		},
		{
			name:  "Repo error: DB error",
			input: validInput,
			setupMocks: func(r *MockUserProvider, tm *MockTokenManager) {
				r.CreateUserFunc = func(ctx context.Context, req userdto.CreateUserRequest) (*userdto.UserInfo, error) {
					return nil, errors.New("db connection down")
				}
			},
			errContains: "failed to create user",
		},
		{
			name:  "Token generator error",
			input: validInput,
			setupMocks: func(r *MockUserProvider, tm *MockTokenManager) {
				r.CreateUserFunc = func(ctx context.Context, req userdto.CreateUserRequest) (*userdto.UserInfo, error) {
					return &userdto.UserInfo{Email: req.Email, Username: req.Username}, nil
				}
				tm.GenerateFunc = func(ID uuid.UUID, payload any) (string, error) {
					return "", errors.New("token failure")
				}
			},
			errContains: "failed to generate token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &MockUserProvider{}
			tokenMgr := &MockTokenManager{}
			if tt.setupMocks != nil {
				tt.setupMocks(repo, tokenMgr)
			}

			uc := usecase.NewUsecase(repo, tokenMgr)
			resp, token, err := uc.Register(ctx, tt.input)

			if tt.expectedErr != nil {
				require.ErrorIs(t, err, tt.expectedErr)
				return
			}
			if tt.errContains != "" {
				require.ErrorContains(t, err, tt.errContains)
				return
			}

			require.NoError(t, err)
			if tt.checkResponse {
				require.NotNil(t, resp)
				require.Equal(t, "jwt-token", token)
			}
		})
	}
}

func TestUsecase_Login(t *testing.T) {
	ctx := context.Background()
	plainPassword := "secret123"
	hash, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)
	require.NoError(t, err)

	activeUser := &userdto.UserCredentials{
		UserInfo: userdto.UserInfo{
			ID:       uuid.New(),
			Email:    "user@mail.ru",
			Username: "username",
			Status:   "active",
		},
		PasswordHash: string(hash),
	}

	tests := []struct {
		name          string
		input         dto.LoginRequest
		setupMocks    func(r *MockUserProvider, tm *MockTokenManager)
		expectedErr   error
		errContains   string
		checkResponse bool
	}{
		{
			name:  "Success login by Email",
			input: dto.LoginRequest{Login: "user@mail.ru", Password: plainPassword},
			setupMocks: func(r *MockUserProvider, tm *MockTokenManager) {
				r.FindUserByLoginFunc = func(ctx context.Context, login string) (*userdto.UserCredentials, error) {
					return activeUser, nil
				}
				tm.GenerateFunc = func(ID uuid.UUID, payload any) (string, error) {
					return "jwt-token", nil
				}
			},
			checkResponse: true,
		},
		{
			name:  "User not found",
			input: dto.LoginRequest{Login: "unknown@mail.ru", Password: plainPassword},
			setupMocks: func(r *MockUserProvider, tm *MockTokenManager) {
				r.FindUserByLoginFunc = func(ctx context.Context, login string) (*userdto.UserCredentials, error) {
					return nil, userusecase.ErrUserNotFound
				}
			},
			expectedErr: usecase.ErrLoginFailed,
		},
		{
			name:  "Repo DB error",
			input: dto.LoginRequest{Login: "user@mail.ru", Password: plainPassword},
			setupMocks: func(r *MockUserProvider, tm *MockTokenManager) {
				r.FindUserByLoginFunc = func(ctx context.Context, login string) (*userdto.UserCredentials, error) {
					return nil, errors.New("db network error")
				}
			},
			errContains: "failed to find user",
		},
		{
			name:  "Wrong password",
			input: dto.LoginRequest{Login: "user@mail.ru", Password: "wrong_password"},
			setupMocks: func(r *MockUserProvider, tm *MockTokenManager) {
				r.FindUserByLoginFunc = func(ctx context.Context, login string) (*userdto.UserCredentials, error) {
					return activeUser, nil
				}
			},
			expectedErr: usecase.ErrLoginFailed,
		},
		{
			name:  "User banned",
			input: dto.LoginRequest{Login: "user@mail.ru", Password: plainPassword},
			setupMocks: func(r *MockUserProvider, tm *MockTokenManager) {
				r.FindUserByLoginFunc = func(ctx context.Context, login string) (*userdto.UserCredentials, error) {
					inactiveUser := *activeUser
					inactiveUser.Status = "banned"
					return &inactiveUser, nil
				}
			},
			expectedErr: usecase.ErrLoginFailed,
			errContains: "account disabled",
		},
		{
			name:  "Token generation error",
			input: dto.LoginRequest{Login: "user@mail.ru", Password: plainPassword},
			setupMocks: func(r *MockUserProvider, tm *MockTokenManager) {
				r.FindUserByLoginFunc = func(ctx context.Context, login string) (*userdto.UserCredentials, error) {
					return activeUser, nil
				}
				tm.GenerateFunc = func(ID uuid.UUID, payload any) (string, error) {
					return "", errors.New("token failed")
				}
			},
			errContains: "failed to generate token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &MockUserProvider{}
			tokenMgr := &MockTokenManager{}
			if tt.setupMocks != nil {
				tt.setupMocks(repo, tokenMgr)
			}

			uc := usecase.NewUsecase(repo, tokenMgr)
			resp, token, err := uc.Login(ctx, tt.input)

			if tt.expectedErr != nil {
				require.ErrorIs(t, err, tt.expectedErr)
			}
			if tt.errContains != "" {
				require.ErrorContains(t, err, tt.errContains)
				return
			}
			if tt.expectedErr != nil {
				return
			}

			require.NoError(t, err)
			if tt.checkResponse {
				require.NotNil(t, resp)
				require.Equal(t, "jwt-token", token)
			}
		})
	}
}

func TestUsecase_TokenTTL(t *testing.T) {
	expectedTTL := 24 * time.Hour
	tm := &MockTokenManager{
		TTLFunc: func() time.Duration {
			return expectedTTL
		},
	}

	uc := usecase.NewUsecase(&MockUserProvider{}, tm)
	require.Equal(t, expectedTTL, uc.TokenTTL())
}
