package dto

import (
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/models"
	"github.com/google/uuid"
)

type UserInfo struct {
	ID        uuid.UUID
	Email     string
	Username  string
	Nickname  string
	AvatarKey *string
	CreatedAt time.Time
}

func ToUserInfo(user *models.User) *UserInfo {
	if user == nil {
		return nil
	}
	return &UserInfo{
		ID:        user.ID,
		Email:     user.Email,
		Username:  user.Username,
		Nickname:  user.Nickname,
		AvatarKey: user.AvatarKey,
		CreatedAt: user.CreatedAt,
	}
}
