package testutils

import (
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/validator"
)

func SetupValidator() *validator.Validator {
	v, err := validator.NewValidator(validator.ValidationConfig{
		Username: validator.UsernameRules{
			MinLen:       3,
			MaxLen:       32,
			AllowedRunes: "abcdefghijklmnopqrstuvwxyz0123456789_",
		},
		Nickname: validator.NicknameRules{MaxLen: 32},
		Password: validator.PasswordRules{MinLen: 8, MaxLen: 32},
		Email: validator.EmailRules{
			MaxLen: 32,
			Regexp: `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`,
		},
	})
	if err != nil {
		panic(err)
	}
	return v
}
