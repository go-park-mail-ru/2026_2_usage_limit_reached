package dto

import (
	"errors"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/validator"
)

var ErrValidation = errors.New("validation error")

type RegistrationRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Password string `json:"password"`
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
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r *LoginRequest) Validate(v validator.Validator) error {
	if !v.IsValidEmail(r.Email) || r.Password == "" {
		return ErrValidation
	}

	return nil
}

type UserResponse struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
}

func ToUserResponse(user models.User) UserResponse {
	return UserResponse{
		Email:    user.Email,
		Username: user.Username,
		Nickname: user.Nickname,
	}
}
