package testutils

import (
	"regexp"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/validator"
)

func SetupValidator() *validator.Validator {
	allowedRunes := make(map[rune]struct{})
	for _, r := range "abcdefghijklmnopqrstuvwxyz0123456789_" {
		allowedRunes[r] = struct{}{}
	}

	return validator.NewValidator(&validator.ValidationConfig{
		MinUsernameLen:       3,
		MaxUsernameLen:       32,
		MaxNicknameLen:       32,
		MinPasswordLen:       8,
		MaxPasswordLen:       32,
		MaxEmailLen:          32,
		EmailRegexp:          regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`),
		UsernameAllowedRunes: allowedRunes,
	})
}
