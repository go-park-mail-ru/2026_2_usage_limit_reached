package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/models"
	"github.com/google/uuid"
)

func (uc *Usecase) CreateUser(ctx context.Context, req dto.CreateUserRequest) (*dto.UserInfo, error) {
	user := &models.User{
		ID:           uuid.New(),
		Email:        normalize(req.Email),
		Username:     normalize(req.Username),
		Nickname:     req.Nickname,
		PasswordHash: req.PasswordHash,
		Status:       models.StatusActive,
	}

	user, err := uc.repo.CreateUser(ctx, user)
	if err != nil {
		if errors.Is(err, models.ErrUserExists) {
			return nil, fmt.Errorf("create user: %w", ErrUserExists)
		}
		return nil, fmt.Errorf("create user: %w", err)
	}

	return dto.ToUserInfo(user), nil
}

func (uc *Usecase) FindUserByID(ctx context.Context, userID uuid.UUID) (*dto.UserInfo, error) {
	user, err := uc.repo.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			return nil, fmt.Errorf("find user: %w", ErrUserNotFound)
		}
		return nil, fmt.Errorf("find user: %w", err)
	}

	return dto.ToUserInfo(user), nil
}

func (uc *Usecase) FindUserByLogin(ctx context.Context, login string) (*dto.UserCredentials, error) {
	user, err := uc.findUserByLogin(ctx, normalize(login))
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			return nil, fmt.Errorf("find user: %w", ErrUserNotFound)
		}
		return nil, fmt.Errorf("find user: %w", err)
	}

	return dto.ToUserCredentials(user), nil
}

func (uc *Usecase) findUserByLogin(ctx context.Context, login string) (*models.User, error) {
	if strings.ContainsRune(login, '@') {
		return uc.repo.GetUserByEmail(ctx, login)
	}

	return uc.repo.GetUserByUsername(ctx, login)
}

func normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
