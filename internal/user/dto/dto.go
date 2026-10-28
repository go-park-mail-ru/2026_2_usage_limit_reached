package dto

import (
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/models"
	"github.com/google/uuid"
)

type CreateUserRequest struct {
	Email        string
	Username     string
	Nickname     string
	PasswordHash string
}

type UserInfo struct {
	ID        uuid.UUID
	Email     string
	Username  string
	Nickname  string
	AvatarKey *string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type UserCredentials struct {
	UserInfo
	PasswordHash string
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
		Status:    user.Status,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

func ToUserCredentials(user *models.User) *UserCredentials {
	if user == nil {
		return nil
	}
	return &UserCredentials{
		UserInfo:     *ToUserInfo(user),
		PasswordHash: user.PasswordHash,
	}
}
