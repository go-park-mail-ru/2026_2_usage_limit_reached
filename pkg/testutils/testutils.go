package testutils

import (
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/validator"
)

func SetupValidator() *validator.Validator {
	v, err := validator.NewValidator(&validator.ValidationConfig{
		MinUsernameLen:       3,
		MaxUsernameLen:       32,
		MaxNicknameLen:       32,
		MinPasswordLen:       8,
		MaxPasswordLen:       32,
		MaxEmailLen:          32,
		EmailRegexp:          `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`,
		UsernameAllowedRunes: "abcdefghijklmnopqrstuvwxyz0123456789_",
	})
	if err != nil {
		panic(err)
	}
	return v
}
