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
	user, err := uc.repo.CreateUser(ctx, models.User{
		Email:        normalizeLogin(req.Email),
		Username:     normalizeLogin(req.Username),
		Nickname:     strings.TrimSpace(req.Nickname),
		PasswordHash: req.PasswordHash,
		Status:       models.StatusActive,
	})
	if err != nil {
		if errors.Is(err, models.ErrUserExists) {
			return nil, fmt.Errorf("create user: %w", ErrUserExists)
		}
		return nil, fmt.Errorf("create user: %w", err)
	}

	return dto.ToUserInfo(user), nil
}

func (uc *Usecase) UpdateNickname(ctx context.Context, userID uuid.UUID, nickname string) (string, error) {
	nickname = strings.TrimSpace(nickname)
	if nickname == "" {
		return "", fmt.Errorf("update nickname: %w", ErrInvalidNickname)
	}

	updated, err := uc.repo.UpdateNickname(ctx, userID, nickname)
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			return "", fmt.Errorf("update nickname: %w", ErrUserNotFound)
		}
		return "", fmt.Errorf("update nickname: %w", err)
	}

	return updated, nil
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

func (uc *Usecase) FindCredentialsByLogin(ctx context.Context, login string) (*dto.UserCredentials, error) {
	user, err := uc.findUserByLogin(ctx, normalizeLogin(login))
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

func normalizeLogin(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
