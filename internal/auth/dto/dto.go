package dto

import (
	"errors"
	"strings"

	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/validator"
	"github.com/google/uuid"
)

var errValidation = errors.New("validation error")

type RegistrationRequest struct {
	Email    string `json:"email" example:"user@example.com"`
	Username string `json:"username" example:"username123"`
	Nickname string `json:"nickname" example:"nickname_123"`
	Password string `json:"password" example:"secret_password_123"`
}

func (r *RegistrationRequest) Validate(v *validator.Validator) error {
	if !v.IsValidEmail(r.Email) ||
		!v.IsValidUsername(r.Username) ||
		!v.IsValidNickname(r.Nickname) ||
		!v.IsValidPassword(r.Password) {
		return errValidation
	}

	return nil
}

type LoginRequest struct {
	// Email или username пользователя
	Login    string `json:"login" example:"username123"`
	Password string `json:"password" example:"secret_password_123"`
}

func (r *LoginRequest) Validate(v *validator.Validator) error {
	if strings.TrimSpace(r.Login) == "" || strings.TrimSpace(r.Password) == "" {
		return errValidation
	}

	return nil
}

type UserResponse struct {
	Email     string    `json:"email" example:"user@example.com"`
	Username  string    `json:"username" example:"username123"`
	Nickname  string    `json:"nickname" example:"nickname_123"`
	AvatarKey string    `json:"avatar_key" example:"AKIAIOSFODNN7EXAMPLE"`
	Status    string    `json:"status" example:"active"`
	CreatedAt time.Time `json:"created_at" example:"2025-06-15T14:30:45.123456789Z"`
	UpdatedAt time.Time `json:"updated_at" example:"2025-06-15T14:30:45.123456789Z"`
}

type UserInfo struct {
	ID        uuid.UUID
	Email     string
	Username  string
	Nickname  string
	AvatarKey string
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
		AvatarKey: processEmptyAvatar(user.AvatarKey),
		CreatedAt: user.CreatedAt,
	}
}

func ToUserResponse(user *models.User) *UserResponse {
	if user == nil {
		return nil
	}
	return &UserResponse{
		Email:     user.Email,
		Username:  user.Username,
		Nickname:  user.Nickname,
		AvatarKey: processEmptyAvatar(user.AvatarKey),
		Status:    user.Status,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

func processEmptyAvatar(userAvatar *string) string {
	var avatarKey string
	if userAvatar == nil {
		avatarKey = ""
	} else {
		avatarKey = *userAvatar
	}
	return avatarKey
}
