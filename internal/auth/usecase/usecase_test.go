package usecase_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/usecase"
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
		setupMocks    func(r *MockRepository, tm *MockTokenManager)
		expectedErr   error
		errContains   string
		checkResponse bool
	}{
		{
			name:  "Success",
			input: validInput,
			setupMocks: func(r *MockRepository, tm *MockTokenManager) {
				r.CreateUserFunc = func(ctx context.Context, user *models.User) (*models.User, error) {
					require.Equal(t, "test@mail.ru", user.Email, "email must ne normalized")
					require.Equal(t, "testuser", user.Username, "username must ne normalized")
					return user, nil
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
			setupMocks: func(r *MockRepository, tm *MockTokenManager) {
				r.CreateUserFunc = func(ctx context.Context, user *models.User) (*models.User, error) {
					return nil, models.ErrUserExists
				}
			},
			expectedErr: usecase.ErrRegistrationFailed,
		},
		{
			name:  "Repo error: DB error",
			input: validInput,
			setupMocks: func(r *MockRepository, tm *MockTokenManager) {
				r.CreateUserFunc = func(ctx context.Context, user *models.User) (*models.User, error) {
					return nil, errors.New("db connection down")
				}
			},
			errContains: "failed to create user",
		},
		{
			name:  "Token generator error",
			input: validInput,
			setupMocks: func(r *MockRepository, tm *MockTokenManager) {
				r.CreateUserFunc = func(ctx context.Context, user *models.User) (*models.User, error) {
					return user, nil
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
			repo := &MockRepository{}
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

	activeUser := &models.User{
		ID:           uuid.New(),
		Email:        "user@mail.ru",
		Username:     "username",
		PasswordHash: string(hash),
		Status:       "active",
	}

	tests := []struct {
		name          string
		input         dto.LoginRequest
		setupMocks    func(r *MockRepository, tm *MockTokenManager)
		expectedErr   error
		errContains   string
		checkResponse bool
	}{
		{
			name:  "Success login by Email",
			input: dto.LoginRequest{Login: "user@mail.ru", Password: plainPassword},
			setupMocks: func(r *MockRepository, tm *MockTokenManager) {
				r.GetUserByEmailFunc = func(ctx context.Context, email string) (*models.User, error) {
					return activeUser, nil
				}
				tm.GenerateFunc = func(ID uuid.UUID, payload any) (string, error) {
					return "jwt-token", nil
				}
			},
			checkResponse: true,
		},
		{
			name:  "Success login by Username",
			input: dto.LoginRequest{Login: "username", Password: plainPassword},
			setupMocks: func(r *MockRepository, tm *MockTokenManager) {
				r.GetUserByUsernameFunc = func(ctx context.Context, username string) (*models.User, error) {
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
			setupMocks: func(r *MockRepository, tm *MockTokenManager) {
				r.GetUserByEmailFunc = func(ctx context.Context, email string) (*models.User, error) {
					return nil, models.ErrUserNotFound
				}
			},
			expectedErr: usecase.ErrLoginFailed,
		},
		{
			name:  "Repo DB error",
			input: dto.LoginRequest{Login: "user@mail.ru", Password: plainPassword},
			setupMocks: func(r *MockRepository, tm *MockTokenManager) {
				r.GetUserByEmailFunc = func(ctx context.Context, email string) (*models.User, error) {
					return nil, errors.New("db network error")
				}
			},
			errContains: "failed to find user",
		},
		{
			name:  "Wrong password",
			input: dto.LoginRequest{Login: "user@mail.ru", Password: "wrong_password"},
			setupMocks: func(r *MockRepository, tm *MockTokenManager) {
				r.GetUserByEmailFunc = func(ctx context.Context, email string) (*models.User, error) {
					return activeUser, nil
				}
			},
			expectedErr: usecase.ErrLoginFailed,
		},
		{
			name:  "User banned",
			input: dto.LoginRequest{Login: "user@mail.ru", Password: plainPassword},
			setupMocks: func(r *MockRepository, tm *MockTokenManager) {
				r.GetUserByEmailFunc = func(ctx context.Context, email string) (*models.User, error) {
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
			setupMocks: func(r *MockRepository, tm *MockTokenManager) {
				r.GetUserByEmailFunc = func(ctx context.Context, email string) (*models.User, error) {
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
			repo := &MockRepository{}
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

func TestUsecase_FindUserByID(t *testing.T) {
	ctx := context.Background()
	testID := uuid.New()
	dummyUser := &models.User{
		ID:       testID,
		Email:    "test@mail.ru",
		Username: "testuser",
		Nickname: "testnick",
	}

	tests := []struct {
		name        string
		setupMocks  func(r *MockRepository)
		expectedErr error
		errContains string
		checkResp   bool
	}{
		{
			name: "Success",
			setupMocks: func(r *MockRepository) {
				r.GetUserByIDFunc = func(ctx context.Context, id uuid.UUID) (*models.User, error) {
					return dummyUser, nil
				}
			},
			checkResp: true,
		},
		{
			name: "User not found",
			setupMocks: func(r *MockRepository) {
				r.GetUserByIDFunc = func(ctx context.Context, id uuid.UUID) (*models.User, error) {
					return nil, models.ErrUserNotFound
				}
			},
			expectedErr: usecase.ErrUserNotFound,
		},
		{
			name: "Repo DB error",
			setupMocks: func(r *MockRepository) {
				r.GetUserByIDFunc = func(ctx context.Context, id uuid.UUID) (*models.User, error) {
					return nil, errors.New("db network error")
				}
			},
			errContains: "find user: db network error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &MockRepository{}
			if tt.setupMocks != nil {
				tt.setupMocks(repo)
			}

			uc := usecase.NewUsecase(repo, &MockTokenManager{})
			user, err := uc.FindUserByID(ctx, testID)

			if tt.expectedErr != nil {
				require.ErrorIs(t, err, tt.expectedErr)
				return
			}
			if tt.errContains != "" {
				require.ErrorContains(t, err, tt.errContains)
				return
			}

			require.NoError(t, err)
			if tt.checkResp {
				require.NotNil(t, user)
				require.Equal(t, testID, user.ID)
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

	uc := usecase.NewUsecase(&MockRepository{}, tm)
	require.Equal(t, expectedTTL, uc.TokenTTL())
}
