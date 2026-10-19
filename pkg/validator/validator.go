package validator

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	ErrValidation = errors.New("validation error")
)

type ValidationConfig struct {
	MinUsernameLen       int    `mapstructure:"minusernamelen"`
	MaxUsernameLen       int    `mapstructure:"maxusernamelen"`
	MaxNicknameLen       int    `mapstructure:"maxnicknamelen"`
	MinPasswordLen       int    `mapstructure:"minpasswordlen"`
	MaxPasswordLen       int    `mapstructure:"maxpasswordlen"`
	MaxEmailLen          int    `mapstructure:"maxemaillen"`
	EmailRegexp          string `mapstructure:"email_regexp"`
	UsernameAllowedRunes string `mapstructure:"username_allowed_chars"`
}

type Validator struct {
	*ValidationConfig // есть мысль, что стоит и тут аналогично другим пакетам неэкспортируемые поля написать
	emailRegexp       *regexp.Regexp
	allowedRunes      map[rune]struct{}
}

func NewValidator(valCfg *ValidationConfig) (*Validator, error) {
	emailRegexp, err := regexp.Compile(valCfg.EmailRegexp)
	if err != nil {
		return nil, fmt.Errorf("invalid email regexp: %w", err)
	}

	allowedRunes := make(map[rune]struct{}, len(valCfg.UsernameAllowedRunes))
	for _, r := range valCfg.UsernameAllowedRunes {
		allowedRunes[r] = struct{}{}
	}

	return &Validator{valCfg, emailRegexp, allowedRunes}, nil
}

func (v Validator) IsValidEmail(email string) bool {
	if len(email) > v.MaxEmailLen {
		return false
	}
	return v.emailRegexp.MatchString(email)
}

func (v Validator) IsValidUsername(username string) bool {
	if strings.TrimSpace(username) == "" {
		return false
	}

	length := len([]rune(username))
	if length < v.MinUsernameLen || length > v.MaxUsernameLen {
		return false
	}
	for _, r := range []rune(username) {
		if _, ok := v.allowedRunes[r]; !ok {
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
