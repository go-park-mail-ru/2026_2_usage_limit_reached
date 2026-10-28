package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/dto"
	userdto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/models"
	userusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/jwt"
	"golang.org/x/crypto/bcrypt"
)

func (uc *Usecase) Register(ctx context.Context, regInput dto.RegistrationRequest) (*dto.UserResponse, string, error) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(regInput.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", fmt.Errorf("failed to hash password: %w", err)
	}

	user, err := uc.users.CreateUser(ctx, userdto.CreateUserRequest{
		Email:        regInput.Email,
		Username:     regInput.Username,
		Nickname:     regInput.Nickname,
		PasswordHash: string(passwordHash),
	})
	if err != nil {
		if errors.Is(err, userusecase.ErrUserExists) {
			return nil, "", fmt.Errorf("%w: %w", ErrRegistrationFailed, err)
		}
		return nil, "", fmt.Errorf("failed to create user: %w", err)
	}

	token, err := uc.tokenGenerator.Generate(user.ID, jwt.UserPayload{UserID: user.ID, Role: "user"})
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate token: %w", err)
	}

	response := dto.ToUserResponse(user)
	return response, token, nil
}

func (uc *Usecase) Login(ctx context.Context, logReq dto.LoginRequest) (*dto.UserResponse, string, error) {
	user, err := uc.users.FindUserByLogin(ctx, logReq.Login)

	if err != nil {
		if errors.Is(err, userusecase.ErrUserNotFound) {
			return nil, "", fmt.Errorf("%w: %w", ErrLoginFailed, err)
		}
		return nil, "", fmt.Errorf("failed to find user: %w", err)
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(logReq.Password))
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrLoginFailed, err)
	}

	if user.Status != models.StatusActive {
		return nil, "", fmt.Errorf("%w: account disabled", ErrLoginFailed)
	}

	token, err := uc.tokenGenerator.Generate(user.ID, jwt.UserPayload{UserID: user.ID, Role: "user"})
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate token: %w", err)
	}

	response := dto.ToUserResponse(&user.UserInfo)
	return response, token, nil
}

func (uc *Usecase) TokenTTL() time.Duration {
	return uc.tokenGenerator.TTL()
}
