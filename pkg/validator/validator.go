package validator

import (
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
)

var (
	ErrValidation = errors.New("validation error")
)

const defaultUsernameAllowedRunes = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_"

type UsernameRules struct {
	MinLen       int    `mapstructure:"min_len"`
	MaxLen       int    `mapstructure:"max_len"`
	AllowedRunes string `mapstructure:"allowed_runes"`
}

type NicknameRules struct {
	MaxLen int `mapstructure:"max_len"`
}

type PasswordRules struct {
	MinLen int `mapstructure:"min_len"`
	MaxLen int `mapstructure:"max_len"`
}

type EmailRules struct {
	MaxLen int    `mapstructure:"max_len"`
	Regexp string `mapstructure:"regexp"`
}

type ValidationConfig struct {
	Username UsernameRules `mapstructure:"username"`
	Nickname NicknameRules `mapstructure:"nickname"`
	Password PasswordRules `mapstructure:"password"`
	Email    EmailRules    `mapstructure:"email"`
}

func DefaultConfig() ValidationConfig {
	return ValidationConfig{
		Username: UsernameRules{
			MinLen:       3,
			MaxLen:       32,
			AllowedRunes: defaultUsernameAllowedRunes,
		},
		Nickname: NicknameRules{MaxLen: 64},
		Password: PasswordRules{MinLen: 8, MaxLen: 72},
		Email: EmailRules{
			MaxLen: 254,
			Regexp: `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`,
		},
	}
}

func (c ValidationConfig) Validate() error {
	var errs []error

	positive := []struct {
		name  string
		value int
	}{
		{"username.min_len", c.Username.MinLen},
		{"username.max_len", c.Username.MaxLen},
		{"nickname.max_len", c.Nickname.MaxLen},
		{"password.min_len", c.Password.MinLen},
		{"password.max_len", c.Password.MaxLen},
		{"email.max_len", c.Email.MaxLen},
	}
	for _, p := range positive {
		if p.value <= 0 {
			errs = append(errs, fmt.Errorf("validator: %s must be positive, got %d", p.name, p.value))
		}
	}

	if c.Username.MinLen > c.Username.MaxLen {
		errs = append(errs, fmt.Errorf("validator: username.min_len (%d) > username.max_len (%d)", c.Username.MinLen, c.Username.MaxLen))
	}
	if c.Password.MinLen > c.Password.MaxLen {
		errs = append(errs, fmt.Errorf("validator: password.min_len (%d) > password.max_len (%d)", c.Password.MinLen, c.Password.MaxLen))
	}
	if c.Username.AllowedRunes == "" {
		errs = append(errs, errors.New("validator: username.allowed_runes must not be empty"))
	}
	if _, err := regexp.Compile(c.Email.Regexp); err != nil {
		errs = append(errs, fmt.Errorf("validator: invalid email.regexp: %w", err))
	}

	return errors.Join(errs...)
}

func (c ValidationConfig) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Group("username",
			slog.Int("min_len", c.Username.MinLen),
			slog.Int("max_len", c.Username.MaxLen),
			slog.String("allowed_runes", c.Username.AllowedRunes),
		),
		slog.Group("nickname", slog.Int("max_len", c.Nickname.MaxLen)),
		slog.Group("password",
			slog.Int("min_len", c.Password.MinLen),
			slog.Int("max_len", c.Password.MaxLen),
		),
		slog.Group("email",
			slog.Int("max_len", c.Email.MaxLen),
			slog.String("regexp", c.Email.Regexp),
		),
	)
}

type Validator struct {
	cfg          ValidationConfig
	emailRegexp  *regexp.Regexp
	allowedRunes map[rune]struct{}
}

func NewValidator(valCfg ValidationConfig) (*Validator, error) {
	if err := valCfg.Validate(); err != nil {
		return nil, err
	}

	emailRegexp := regexp.MustCompile(valCfg.Email.Regexp)

	allowedRunes := make(map[rune]struct{}, len(valCfg.Username.AllowedRunes))
	for _, r := range valCfg.Username.AllowedRunes {
		allowedRunes[r] = struct{}{}
	}

	return &Validator{valCfg, emailRegexp, allowedRunes}, nil
}

func (v Validator) IsValidEmail(email string) bool {
	if len(email) > v.cfg.Email.MaxLen {
		return false
	}
	return v.emailRegexp.MatchString(email)
}

func (v Validator) IsValidUsername(username string) bool {
	if strings.TrimSpace(username) == "" {
		return false
	}

	length := len([]rune(username))
	if length < v.cfg.Username.MinLen || length > v.cfg.Username.MaxLen {
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
	return length > 0 && length <= v.cfg.Nickname.MaxLen
}

func (v Validator) IsValidPassword(password string) bool {
	length := len(password)
	return length >= v.cfg.Password.MinLen && length <= v.cfg.Password.MaxLen
}
