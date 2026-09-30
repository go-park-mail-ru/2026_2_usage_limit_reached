package validator

import (
	"regexp"
	"strings"
	"unicode"
)

const (
	MinUsernameLen = 3
	MaxUsernameLen = 32
	MaxNicknameLen = 64
	MinPasswordLen = 8
	MaxPasswordLen = 72
	MaxEmailLen    = 254
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

func IsValidEmail(email string) bool {
	if email == "" || len(email) > MaxEmailLen {
		return false
	}
	return emailRegex.MatchString(email)
}

func IsValidUsername(username string) bool {
	length := len([]rune(username))
	if length < MinUsernameLen || length > MaxUsernameLen {
		return false
	}
	for _, r := range username {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_') {
			return false
		}
	}
	return true
}

func IsValidNickname(nickname string) bool {
	length := len([]rune(nickname))
	return length > 0 && length <= MaxNicknameLen
}

func IsValidPassword(password string) bool {
	length := len(password)
	return length >= MinPasswordLen && length <= MaxPasswordLen
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
