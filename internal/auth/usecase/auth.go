package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/jwt"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func (uc *Usecase) Register(ctx context.Context, regInput dto.RegistrationRequest) (*dto.UserResponse, string, error) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(regInput.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", fmt.Errorf("failed to hash password: %w", err)
	}

	user := &models.User{
		ID:           uuid.New(),
		Email:        normalize(regInput.Email),
		Username:     normalize(regInput.Username),
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
		return nil, "", fmt.Errorf("failed to create user: %w", err) // типа ошибка похода в базу
	}

	token, err := uc.tokenGenerator.Generate(user.ID, jwt.UserPayload{UserID: user.ID, Role: "user"})
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate token: %w", err)
	}

	response := dto.ToUserResponse(user)
	return response, token, nil
}

func (uc *Usecase) Login(ctx context.Context, logReq dto.LoginRequest) (*dto.UserResponse, string, error) {
	user, err := uc.findUserByLogin(ctx, normalize(logReq.Login))

	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			return nil, "", fmt.Errorf("%w: %w", ErrLoginFailed, err)
		}
		return nil, "", fmt.Errorf("failed to find user: %w", err)
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(logReq.Password))
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrLoginFailed, err)
	}

	if user.Status != statusActive {
		return nil, "", fmt.Errorf("%w: account disabled", ErrLoginFailed)
	}

	token, err := uc.tokenGenerator.Generate(user.ID, jwt.UserPayload{UserID: user.ID, Role: "user"})
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate token: %w", err)
	}

	response := dto.ToUserResponse(user)
	return response, token, nil
}

func (uc *Usecase) findUserByLogin(ctx context.Context, login string) (*models.User, error) {
	if strings.ContainsRune(login, '@') {
		return uc.repo.GetUserByEmail(ctx, login)
	}

	return uc.repo.GetUserByUsername(ctx, login)
}

func (uc *Usecase) TokenTTL() time.Duration {
	return uc.tokenGenerator.TTL()
}

func normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
