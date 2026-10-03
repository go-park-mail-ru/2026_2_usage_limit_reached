package dto

import (
	"errors"
	"strings"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/validator"
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
	Email    string `json:"email" example:"user@example.com"`
	Username string `json:"username" example:"username123"`
	Nickname string `json:"nickname" example:"nickname_123"`
}

func ToUserResponse(user *models.User) *UserResponse {
	if user == nil {
		return nil
	}
	return &UserResponse{
		Email:    user.Email,
		Username: user.Username,
		Nickname: user.Nickname,
	}
}
