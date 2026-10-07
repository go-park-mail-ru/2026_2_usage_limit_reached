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
	"golang.org/x/crypto/bcrypt"
)

type MockRepository struct {
	CreateUserFunc        func(ctx context.Context, user *models.User) (*models.User, error)
	GetUserByIDFunc       func(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetUserByEmailFunc    func(ctx context.Context, email string) (*models.User, error)
	GetUserByUsernameFunc func(ctx context.Context, username string) (*models.User, error)
}

func (m *MockRepository) CreateUser(ctx context.Context, user *models.User) (*models.User, error) {
	if m.CreateUserFunc != nil {
		return m.CreateUserFunc(ctx, user)
	}
	return nil, nil
}

func (m *MockRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	if m.GetUserByIDFunc != nil {
		return m.GetUserByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *MockRepository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	if m.GetUserByEmailFunc != nil {
		return m.GetUserByEmailFunc(ctx, email)
	}
	return nil, nil
}

func (m *MockRepository) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	if m.GetUserByUsernameFunc != nil {
		return m.GetUserByUsernameFunc(ctx, username)
	}
	return nil, nil
}

type MockTokenManager struct {
	GenerateFunc func(ID uuid.UUID, payload any) (string, error)
	TTLFunc      func() time.Duration
}

func (m *MockTokenManager) Generate(ID uuid.UUID, payload any) (string, error) {
	if m.GenerateFunc != nil {
		return m.GenerateFunc(ID, payload)
	}
	return "", nil
}

func (m *MockTokenManager) TTL() time.Duration {
	if m.TTLFunc != nil {
		return m.TTLFunc()
	}
	return 24 * time.Hour
}

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
		mockRepo      func(r *MockRepository)
		mockToken     func(tm *MockTokenManager)
		expectedErr   error
		errContains   string
		checkResponse bool
	}{
		{
			name:  "Success",
			input: validInput,
			mockRepo: func(r *MockRepository) {
				r.CreateUserFunc = func(ctx context.Context, user *models.User) (*models.User, error) {
					if user.Email != "test@mail.ru" || user.Username != "testuser" {
						t.Errorf("fields were not normalized: %+v", user)
					}
					return user, nil
				}
			},
			mockToken: func(tm *MockTokenManager) {
				tm.GenerateFunc = func(ID uuid.UUID, payload any) (string, error) {
					return "jwt-token", nil
				}
			},
			expectedErr:   nil,
			checkResponse: true,
		},
		{
			name: "Bcrypt error (password > 72 bytes)",
			input: dto.RegistrationRequest{
				Password: strings.Repeat("a", 73), // Bcrypt падает с ошибкой на длине > 72 байт
			},
			mockRepo:    func(r *MockRepository) {},
			mockToken:   func(tm *MockTokenManager) {},
			errContains: "failed to hash password",
		},
		{
			name:  "Repo error: User already exists",
			input: validInput,
			mockRepo: func(r *MockRepository) {
				r.CreateUserFunc = func(ctx context.Context, user *models.User) (*models.User, error) {
					return nil, models.ErrUserExists
				}
			},
			mockToken:   func(tm *MockTokenManager) {},
			expectedErr: usecase.ErrRegistrationFailed,
		},
		{
			name:  "Repo error: DB error",
			input: validInput,
			mockRepo: func(r *MockRepository) {
				r.CreateUserFunc = func(ctx context.Context, user *models.User) (*models.User, error) {
					return nil, errors.New("db connection down")
				}
			},
			mockToken:   func(tm *MockTokenManager) {},
			errContains: "failed to create user",
		},
		{
			name:  "Token generator error",
			input: validInput,
			mockRepo: func(r *MockRepository) {
				r.CreateUserFunc = func(ctx context.Context, user *models.User) (*models.User, error) {
					return user, nil
				}
			},
			mockToken: func(tm *MockTokenManager) {
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
			tt.mockRepo(repo)
			tt.mockToken(tokenMgr)

			uc := usecase.NewUsecase(repo, tokenMgr)
			resp, token, err := uc.Register(ctx, tt.input)

			if tt.expectedErr != nil {
				if !errors.Is(err, tt.expectedErr) {
					t.Fatalf("expected error %v, got %v", tt.expectedErr, err)
				}
				return
			}

			if tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("expected error containing %q, got %v", tt.errContains, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.checkResponse {
				if resp == nil || token != "jwt-token" {
					t.Errorf("invalid response or token: resp=%v, token=%s", resp, token)
				}
			}
		})
	}
}

func TestUsecase_Login(t *testing.T) {
	ctx := context.Background()
	plainPassword := "secret123"
	hash, _ := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)

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
		mockRepo      func(r *MockRepository)
		mockToken     func(tm *MockTokenManager)
		expectedErr   error
		errContains   string
		checkResponse bool
	}{
		{
			name: "Success login by Email",
			input: dto.LoginRequest{
				Login:    "user@mail.ru",
				Password: plainPassword,
			},
			mockRepo: func(r *MockRepository) {
				r.GetUserByEmailFunc = func(ctx context.Context, email string) (*models.User, error) {
					return activeUser, nil
				}
			},
			mockToken: func(tm *MockTokenManager) {
				tm.GenerateFunc = func(ID uuid.UUID, payload any) (string, error) {
					return "jwt-token", nil
				}
			},
			checkResponse: true,
		},
		{
			name: "Success login by Username",
			input: dto.LoginRequest{
				Login:    "username",
				Password: plainPassword,
			},
			mockRepo: func(r *MockRepository) {
				r.GetUserByUsernameFunc = func(ctx context.Context, username string) (*models.User, error) {
					return activeUser, nil
				}
			},
			mockToken: func(tm *MockTokenManager) {
				tm.GenerateFunc = func(ID uuid.UUID, payload any) (string, error) {
					return "jwt-token", nil
				}
			},
			checkResponse: true,
		},
		{
			name: "User not found",
			input: dto.LoginRequest{
				Login:    "unknown@mail.ru",
				Password: plainPassword,
			},
			mockRepo: func(r *MockRepository) {
				r.GetUserByEmailFunc = func(ctx context.Context, email string) (*models.User, error) {
					return nil, models.ErrUserNotFound
				}
			},
			mockToken:   func(tm *MockTokenManager) {},
			expectedErr: usecase.ErrLoginFailed,
		},
		{
			name: "Repo DB error",
			input: dto.LoginRequest{
				Login:    "user@mail.ru",
				Password: plainPassword,
			},
			mockRepo: func(r *MockRepository) {
				r.GetUserByEmailFunc = func(ctx context.Context, email string) (*models.User, error) {
					return nil, errors.New("db network error")
				}
			},
			mockToken:   func(tm *MockTokenManager) {},
			errContains: "failed to find user",
		},
		{
			name: "Wrong password",
			input: dto.LoginRequest{
				Login:    "user@mail.ru",
				Password: "wrong_password",
			},
			mockRepo: func(r *MockRepository) {
				r.GetUserByEmailFunc = func(ctx context.Context, email string) (*models.User, error) {
					return activeUser, nil
				}
			},
			mockToken:   func(tm *MockTokenManager) {},
			expectedErr: usecase.ErrLoginFailed,
		},
		{
			name: "User banned",
			input: dto.LoginRequest{
				Login:    "user@mail.ru",
				Password: plainPassword,
			},
			mockRepo: func(r *MockRepository) {
				r.GetUserByEmailFunc = func(ctx context.Context, email string) (*models.User, error) {
					inactiveUser := *activeUser
					inactiveUser.Status = "banned"
					return &inactiveUser, nil
				}
			},
			mockToken:   func(tm *MockTokenManager) {},
			expectedErr: usecase.ErrLoginFailed,
			errContains: "account disabled",
		},
		{
			name: "Token generation error",
			input: dto.LoginRequest{
				Login:    "user@mail.ru",
				Password: plainPassword,
			},
			mockRepo: func(r *MockRepository) {
				r.GetUserByEmailFunc = func(ctx context.Context, email string) (*models.User, error) {
					return activeUser, nil
				}
			},
			mockToken: func(tm *MockTokenManager) {
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
			tt.mockRepo(repo)
			tt.mockToken(tokenMgr)

			uc := usecase.NewUsecase(repo, tokenMgr)
			resp, token, err := uc.Login(ctx, tt.input)

			if tt.expectedErr != nil {
				if !errors.Is(err, tt.expectedErr) {
					t.Fatalf("expected error %v, got %v", tt.expectedErr, err)
				}
			}

			if tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("expected error message to contain %q, got %v", tt.errContains, err)
				}
				return
			}

			if tt.expectedErr != nil {
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.checkResponse {
				if resp == nil || token != "jwt-token" {
					t.Errorf("invalid response or token: resp=%v, token=%s", resp, token)
				}
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
		mockRepo    func(r *MockRepository)
		expectedErr error
		errContains string
		checkResp   bool
	}{
		{
			name: "Success",
			mockRepo: func(r *MockRepository) {
				r.GetUserByIDFunc = func(ctx context.Context, id uuid.UUID) (*models.User, error) {
					return dummyUser, nil
				}
			},
			checkResp: true,
		},
		{
			name: "User not found",
			mockRepo: func(r *MockRepository) {
				r.GetUserByIDFunc = func(ctx context.Context, id uuid.UUID) (*models.User, error) {
					return nil, models.ErrUserNotFound
				}
			},
			expectedErr: usecase.ErrUserNotFound,
		},
		{
			name: "Repo DB error",
			mockRepo: func(r *MockRepository) {
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
			tt.mockRepo(repo)

			uc := usecase.NewUsecase(repo, &MockTokenManager{})
			user, err := uc.FindUserByID(ctx, testID)

			if tt.expectedErr != nil {
				if !errors.Is(err, tt.expectedErr) {
					t.Fatalf("expected error wrapping %v, got %v", tt.expectedErr, err)
				}
				return
			}

			if tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("expected error to contain %q, got %v", tt.errContains, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.checkResp {
				if user == nil || user.ID != testID {
					t.Fatalf("expected valid user info, got %v", user)
				}
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

	if uc.TokenTTL() != expectedTTL {
		t.Errorf("expected TTL %v, got %v", expectedTTL, uc.TokenTTL())
	}
}