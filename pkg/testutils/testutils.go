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
		Bio:      validator.BioRules{MaxLen: 1000},
		Category: validator.CategoryRules{MaxLen: 64},
		Password: validator.PasswordRules{MinLen: 8, MaxLen: 32},
		Email: validator.EmailRules{
			MaxLen: 32,
			Regexp: `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`,
		},
		Post: validator.PostRules{
			MaxTitleLen:       255,
			MaxBodyLen:        20000,
			AllowedPostStatus: []string{"published", "draft", "archived"},
		},
	})
	if err != nil {
		panic(err)
	}
	return v
}
