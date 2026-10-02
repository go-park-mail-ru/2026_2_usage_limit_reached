package validator

import (
	"regexp"
)

type ValidationConfig struct {
	MinUsernameLen       int
	MaxUsernameLen       int
	MaxNicknameLen       int
	MinPasswordLen       int
	MaxPasswordLen       int
	MaxEmailLen          int
	EmailRegexp          *regexp.Regexp
	UsernameAllowedRunes map[rune]struct{}
}

type Validator struct {
	ValidationConfig
}

func NewValidator(valCfg ValidationConfig) Validator {
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
