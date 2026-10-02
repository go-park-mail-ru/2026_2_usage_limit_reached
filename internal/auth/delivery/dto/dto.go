package dto

import (
	"errors"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/validator"
)

var ErrValidation = errors.New("validation error")

type RegistrationRequest struct {
	Email    string `json:"email" example:"user@example.com"`
	Username string `json:"username" example:"username123"`
	Nickname string `json:"nickname" example:"nickname_123"`
	Password string `json:"password" example:"secret_password_123"`
}

func (r *RegistrationRequest) Validate(v validator.Validator) error {
	if !v.IsValidEmail(r.Email) ||
		!v.IsValidUsername(r.Username) ||
		!v.IsValidNickname(r.Nickname) ||
		!v.IsValidPassword(r.Password) {
		return ErrValidation
	}

	return nil
}

type LoginRequest struct {
	Username string `json:"username" example:"username123"`
	Email    string `json:"email" example:"user@example.com"`
	Password string `json:"password" example:"secret_password_123"`
}

func (r *LoginRequest) Validate(v validator.Validator) error {
	if r.Password == "" {
		return ErrValidation
	}

	return nil
}

type UserResponse struct {
	Email    string `json:"email" example:"user@example.com"`
	Username string `json:"username" example:"username123"`
	Nickname string `json:"nickname" example:"nickname_123"`
}

func ToUserResponse(user models.User) UserResponse {
	return UserResponse{
		Email:    user.Email,
		Username: user.Username,
		Nickname: user.Nickname,
	}
}
