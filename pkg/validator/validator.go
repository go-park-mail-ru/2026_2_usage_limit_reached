package validator

import (
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/config"
)

type Validator struct {
	config.ValidationConfig
}

func NewValidator(valCfg config.ValidationConfig) Validator {
	return Validator{valCfg}
}

func (v Validator) IsValidEmail(email string) bool {
	if len(email) > v.MaxEmailLen {
		return false
	}
	return v.EmailRegexp.MatchString(email)
}

func (v Validator) IsValidUsername(username string) bool {
	length := len([]rune(username))
	if length < v.MinUsernameLen || length > v.MaxUsernameLen {
		return false
	}
	for _, r := range []rune(username) {
		if _, ok := v.UsernameAllowedRunes[r]; !ok {
			return false
		}
	}
	return true
}

func (v Validator) IsValidNickname(nickname string) bool {
	length := len([]rune(nickname))
	return length > 0 && length <= v.MaxNicknameLen
}

func (v Validator) IsValidPassword(password string) bool {
	length := len(password)
	return length >= v.MinPasswordLen && length <= v.MaxPasswordLen
}
