package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/authn"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type tokenClaims struct {
	UserID   uuid.UUID `json:"user_id"`
	Email    string    `json:"email"`
	Username string    `json:"username"`
	Role     string    `json:"role"`
}

func (c tokenClaims) Encode() ([]byte, error) {
	return json.Marshal(c)
}

func (uc *Usecase) Register(ctx context.Context, regInput dto.RegistrationRequest) (*dto.UserResponse, string, error) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(regInput.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", fmt.Errorf("failed to hash password: %w", err)
	}

	user, err := uc.createUser(ctx, dto.CreateUserRequest{
		Email:        regInput.Email,
		Username:     regInput.Username,
		Nickname:     regInput.Nickname,
		PasswordHash: string(passwordHash),
	})
	if err != nil {
		if errors.Is(err, ErrUserExists) {
			return nil, "", fmt.Errorf("register: %w", ErrRegistrationFailed)
		}

		return nil, "", fmt.Errorf("failed to create user: %w", err)
	}

	token, err := uc.tokens.Generate(user.ID, tokenClaims{UserID: user.ID, Email: user.Email, Username: user.Username, Role: models.RoleUser})
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate token: %w", err)
	}

	return dto.ToUserResponse(user), token, nil
}

func (uc *Usecase) Login(ctx context.Context, logReq dto.LoginRequest) (*dto.UserResponse, string, error) {
	user, err := uc.findCredentialsByLogin(ctx, normalizeLogin(logReq.Login))
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, "", fmt.Errorf("%w: user not found", ErrLoginFailed)
		}

		return nil, "", fmt.Errorf("failed to find user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(logReq.Password)); err != nil {
		return nil, "", fmt.Errorf("%w: invalid password", ErrLoginFailed)
	}

	if !user.IsActive() {
		return nil, "", fmt.Errorf("%w: account disabled", ErrLoginFailed)
	}

	token, err := uc.tokens.Generate(user.ID, tokenClaims{UserID: user.ID, Email: user.Email, Username: user.Username, Role: user.Role})
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate token: %w", err)
	}

	return dto.ToUserResponse(&user.UserInfo), token, nil
}

func (uc *Usecase) VerifyToken(_ context.Context, token string) (*authn.User, error) {
	if strings.TrimSpace(token) == "" {
		return nil, ErrInvalidToken
	}

	raw, err := uc.tokens.Verify(token)
	if err != nil {
		return nil, fmt.Errorf("verify token: %w: %w", ErrInvalidToken, err)
	}

	var payload tokenClaims
	if err := json.Unmarshal(raw, &payload); err != nil || payload.UserID == uuid.Nil {
		return nil, fmt.Errorf("verify token: %w", ErrInvalidToken)
	}

	return &authn.User{ID: payload.UserID, Role: payload.Role}, nil
}

func (uc *Usecase) TokenTTL() time.Duration {
	return uc.tokens.TTL()
}
