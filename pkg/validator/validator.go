package validator

import (
	"regexp"
)

type Validator struct {
	MinUsernameLen       int
	MaxUsernameLen       int
	MaxNicknameLen       int
	MinPasswordLen       int
	MaxPasswordLen       int
	MaxEmailLen          int
	EmailRegexp          *regexp.Regexp
	UsernameAllowedRunes map[rune]struct{}
}

func NewValidator(
	minUsernameLen int,
	maxUsernameLen int,
	maxNicknameLen int,
	minPasswordLen int,
	maxPasswordLen int,
	maxEmailLen int,
	emailRegexp *regexp.Regexp,
	usernameAllowedRunes map[rune]struct{}) Validator {
	return Validator{
		MinUsernameLen:       minUsernameLen,
		MaxUsernameLen:       maxUsernameLen,
		MaxNicknameLen:       maxNicknameLen,
		MinPasswordLen:       minPasswordLen,
		MaxPasswordLen:       maxPasswordLen,
		MaxEmailLen:          maxEmailLen,
		EmailRegexp:          emailRegexp,
		UsernameAllowedRunes: usernameAllowedRunes,
	}
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
