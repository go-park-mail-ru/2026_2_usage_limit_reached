package adapters

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/dto"
	authUc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/models"
	profileUc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/usecase"
	"github.com/google/uuid"
)

type AuthUserFetcher interface {
	FindUserByID(ctx context.Context, userID uuid.UUID) (*dto.UserInfo, error)
}

type UserInfoRepositoryAdapter struct {
	authService AuthUserFetcher
}

func NewUserInfoRepositoryAdapter(fetcher AuthUserFetcher) *UserInfoRepositoryAdapter {
	return &UserInfoRepositoryAdapter{authService: fetcher}
}

func (u *UserInfoRepositoryAdapter) GetProfileUserByID(ctx context.Context, userID uuid.UUID) (*models.ProfileUser, error) {
	user, err := u.authService.FindUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, authUc.ErrUserNotFound) {
			return nil, fmt.Errorf("profile adapter: %w", profileUc.ErrProfileUserNotFound)
		}
		return nil, fmt.Errorf("profile adapter failed: %w", err)
	}

	return &models.ProfileUser{
		ID:        user.ID,
		Username:  user.Username,
		Nickname:  user.Nickname,
		Email:     user.Email,
		AvatarKey: user.AvatarKey,
		CreatedAt: user.CreatedAt,
	}, nil
}
