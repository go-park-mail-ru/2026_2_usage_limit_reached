package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/token"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func (uc *Usecase) Register(ctx context.Context, regInput dto.RegistrationRequest) (*dto.UserResponse, string, error) {
	normalizedEmail := normalizeEmail(regInput.Email)

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(regInput.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrPassHash, err)
	}

	user := &models.User{
		ID:           uuid.New(),
		Email:        normalizedEmail,
		Username:     regInput.Username,
		Nickname:     regInput.Nickname,
		PasswordHash: string(passwordHash),
		Status:       statusActive,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	user, err = uc.repo.CreateUser(ctx, user)
	if err != nil {
		if errors.Is(err, models.ErrUserExists) {
			return nil, "", fmt.Errorf("%w: %w", ErrRegistrationFailed, err)
		}
		return nil, "", fmt.Errorf("%w: %w", ErrDBAccess, err) // типа ошибка похода в базу
	}

	token, err := uc.tokenGenerator.Generate(token.UserPayload{UserID: user.ID, Role: "user"})
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrTokenGenFailed, err)
	}

	response := dto.ToUserResponse(user)
	return response, token, nil
}

func (uc *Usecase) Login(ctx context.Context, logReq dto.LoginRequest) (*dto.UserResponse, string, error) {
	var identifier string
	if strings.TrimSpace(logReq.Username) != "" {
		identifier = strings.TrimSpace(logReq.Username)
	} else {
		identifier = normalizeEmail(logReq.Email)
	}
	user, err := uc.repo.GetUserByIdentifier(ctx, identifier) // логиниться можно по email или username
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			return nil, "", fmt.Errorf("%w: %w", ErrLoginFailed, err)
		}
		return nil, "", fmt.Errorf("%w: %w", ErrDBAccess, err)
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(logReq.Password))
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrLoginFailed, err)
	}

	token, err := uc.tokenGenerator.Generate(token.UserPayload{UserID: user.ID, Role: "user"})
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrTokenGenFailed, err)
	}

	response := dto.ToUserResponse(user)
	return response, token, nil
}

func (uc *Usecase) TokenTTL() time.Duration {
	return uc.tokenGenerator.TTL()
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
