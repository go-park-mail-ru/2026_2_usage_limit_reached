package dto

import (
	"strings"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	userdto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/dto"
)

type UserResponse struct {
	Username  string  `json:"username" example:"username123"`
	Nickname  string  `json:"nickname" example:"nickname_123"`
	AvatarURL *string `json:"avatar_url,omitempty" example:"/media/avatars/00000000-0000-4000-8000-000000000001/00000000-0000-4000-8000-000000000201.png"`
}

func ToUserResponse(user *userdto.UserInfo) *UserResponse {
	if user == nil {
		return nil
	}

	return &UserResponse{
		Username: user.Username,
		Nickname: user.Nickname,
		// AvatarURL добавится после мерджа с веткой profile-avatar-upload
	}
}

type RegistrationRequest struct {
	Email    string `json:"email" example:"user@example.com"`
	Username string `json:"username" example:"username123"`
	Nickname string `json:"nickname" example:"nickname_123"`
	Password string `json:"password" example:"secret_password_123"`
}

func (r RegistrationRequest) Validate(v *validator.Validator) error {
	if !v.IsValidEmail(r.Email) ||
		!v.IsValidUsername(r.Username) ||
		!v.IsValidNickname(r.Nickname) ||
		!v.IsValidPassword(r.Password) {
		return validator.ErrValidation
	}

	return nil
}

type LoginRequest struct {
	Login    string `json:"login" example:"username123"` // Email или username пользователя
	Password string `json:"password" example:"secret_password_123"`
}

func (r LoginRequest) Validate() error {
	if strings.TrimSpace(r.Login) == "" || strings.TrimSpace(r.Password) == "" {
		return validator.ErrValidation
	}

	return nil
}

type CSRFTokenResponse struct {
	CSRFToken string `json:"csrf_token" example:"q3VtX2Jhc2U2NHVybF9zYWx0X2FuZF9obWFjX3NpZ25hdHVyZV9leGFtcGxlXzAx"`
}
