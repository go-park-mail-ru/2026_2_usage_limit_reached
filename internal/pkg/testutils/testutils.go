package testutils

import (
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
)

func SetupValidator() *validator.Validator {
	v, err := validator.NewValidator(validator.DefaultConfig())
	if err != nil {
		panic(err)
	}
	return v
}
