package dto

import (
	"fmt"
	"strings"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/validator"
)

type RegistrationRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Password string `json:"password"`
}

func (r *RegistrationRequest) Validate() error {
	var problems []string

	r.Email = validator.NormalizeEmail(r.Email)
	if !validator.IsValidEmail(r.Email) {
		problems = append(problems, "email: invalid format")
	}
	if !validator.IsValidUsername(r.Username) {
		problems = append(problems, fmt.Sprintf("username: %d-%d characters, letters/digits/underscore only", validator.MinUsernameLen, validator.MaxUsernameLen))
	}
	if !validator.IsValidNickname(r.Nickname) {
		problems = append(problems, fmt.Sprintf("nickname: up to %d characters", validator.MaxNicknameLen))
	}
	if !validator.IsValidPassword(r.Password) {
		problems = append(problems, fmt.Sprintf("password: %d-%d characters", validator.MinPasswordLen, validator.MaxPasswordLen))
	}

	if len(problems) > 0 {
		return fmt.Errorf("%w: %s", models.ErrValidation, strings.Join(problems, "; "))
	}

	return nil
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r *LoginRequest) Validate() error {
	r.Email = validator.NormalizeEmail(r.Email)
	if !validator.IsValidEmail(r.Email) {
		return fmt.Errorf("%w: email is invalid", models.ErrValidation)
	}
	if r.Password == "" {
		return fmt.Errorf("%w: password is required", models.ErrValidation)
	}

	return nil
}

type UserResponse struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
}
