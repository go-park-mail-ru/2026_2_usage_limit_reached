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

func (uc *Usecase) CreateUser(ctx context.Context, user *models.User) (*models.User, error) {
	user = &models.User{
		ID:           uuid.New(),
		Email:        normalize(user.Email),
		Username:     normalize(user.Username),
		Nickname:     user.Nickname,
		PasswordHash: user.PasswordHash,
		Status:       models.StatusActive,
	}

	return uc.repo.CreateUser(ctx, user)
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

func (uc *Usecase) FindUserByLogin(ctx context.Context, login string) (*models.User, error) {
	login = normalize(login)
	if strings.ContainsRune(login, '@') {
		return uc.repo.GetUserByEmail(ctx, login)
	}

	return uc.repo.GetUserByUsername(ctx, login)
}

func normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
