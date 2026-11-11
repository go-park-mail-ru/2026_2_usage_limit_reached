package validator

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"golang.org/x/text/currency"
)

var (
	ErrValidation = errors.New("validation error")
)

const (
	defaultUsernameAllowedRunes = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_"
	postPublishedStatus         = "published"
	postDraftStatus             = "draft"
	postArchivedStatus          = "archived"
)

type UsernameRules struct {
	MinLen       int    `mapstructure:"min_len"`
	MaxLen       int    `mapstructure:"max_len"`
	AllowedRunes string `mapstructure:"allowed_runes"`
}

type NicknameRules struct {
	MaxLen int `mapstructure:"max_len"`
}

type BioRules struct {
	MaxLen int `mapstructure:"max_len"`
}

type CategoryRules struct {
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

type PostRules struct {
	MaxTitleLen          int             `mapstructure:"max_title_len"`
	MaxBodyLen           int             `mapstructure:"max_body_len"`
	MaxSanitizedBodyLen  int             `mapstructure:"max_sanitized_body_len"`
	MaxSanitizeExpansion int             `mapstructure:"max_sanitize_expansion"`
	AllowedPostStatus    map[string]bool `mapstructure:"allowed_post_status"`
}

type PaymentRules struct {
	RequestKeyLen   int                    `mapstructure:"request_key_len"`
	MinAmountMinor  int                    `mapstructure:"min_amount_minor"`
	AllowedCurrency map[currency.Unit]bool `mapstructure:"allowed_currency"`
}

type DonationRules struct {
	MaxMessageLen int `mapstructure:"max_message_len"`
}

type ValidationConfig struct {
	Username UsernameRules `mapstructure:"username"`
	Nickname NicknameRules `mapstructure:"nickname"`
	Bio      BioRules      `mapstructure:"bio"`
	Category CategoryRules `mapstructure:"category"`
	Password PasswordRules `mapstructure:"password"`
	Email    EmailRules    `mapstructure:"email"`
	Post     PostRules     `mapstructure:"post"`
	Payment  PaymentRules  `mapstructure:"payment"`
	Donation DonationRules `mapstructure:"donation"`
}

func DefaultConfig() ValidationConfig {
	return ValidationConfig{
		Username: UsernameRules{
			MinLen:       3,
			MaxLen:       32,
			AllowedRunes: defaultUsernameAllowedRunes,
		},
		Nickname: NicknameRules{MaxLen: 64},
		Bio:      BioRules{MaxLen: 1000},
		Category: CategoryRules{MaxLen: 64},
		Password: PasswordRules{MinLen: 8, MaxLen: 72},
		Email: EmailRules{
			MaxLen: 254,
			Regexp: `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`,
		},
		Post: PostRules{
			MaxTitleLen:          255,
			MaxBodyLen:           20000,
			MaxSanitizedBodyLen:  200000,
			MaxSanitizeExpansion: 10,
			AllowedPostStatus:    map[string]bool{postPublishedStatus: true, postDraftStatus: true, postArchivedStatus: true},
		},
		Payment: PaymentRules{
			RequestKeyLen:   36,
			MinAmountMinor:  5,
			AllowedCurrency: map[currency.Unit]bool{currency.RUB: true},
		},
		Donation: DonationRules{
			MaxMessageLen: 5000,
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
		{"bio.max_len", c.Bio.MaxLen},
		{"category.max_len", c.Category.MaxLen},
		{"password.min_len", c.Password.MinLen},
		{"password.max_len", c.Password.MaxLen},
		{"email.max_len", c.Email.MaxLen},
		{"post.max_title_len", c.Post.MaxTitleLen},
		{"post.max_body_len", c.Post.MaxBodyLen},
		{"post.max_sanitized_body_len", c.Post.MaxSanitizedBodyLen},
		{"post.max_sanitize_expansion", c.Post.MaxSanitizeExpansion},
		{"payment.request_key_len", c.Payment.RequestKeyLen},
		{"payment.min_amount_minor", c.Payment.MinAmountMinor},
		{"donation.max_message_len", c.Donation.MaxMessageLen},
	}
	for _, p := range positive {
		if p.value <= 0 {
			errs = append(errs, fmt.Errorf("validator: %s must be positive, got %d", p.name, p.value))
		}
	}

	// Потолок после санитайзинга не может быть меньше лимита на вход, иначе допустимый пост не пройдет проверку
	if c.Post.MaxSanitizedBodyLen < c.Post.MaxBodyLen {
		errs = append(errs, fmt.Errorf("validator: post.max_sanitized_body_len (%d) < post.max_body_len (%d)", c.Post.MaxSanitizedBodyLen, c.Post.MaxBodyLen))
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
	if len(c.Post.AllowedPostStatus) == 0 {
		errs = append(errs, errors.New("validator: post.allowed_post_status must not be empty"))
	}
	if len(c.Payment.AllowedCurrency) == 0 {
		errs = append(errs, errors.New("validator: payment.allowed_currency must not be empty"))
	}

	return errors.Join(errs...)
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
	if strings.TrimSpace(nickname) == "" {
		return false
	}

	return len([]rune(nickname)) <= v.cfg.Nickname.MaxLen
}

func (v Validator) IsValidBio(bio string) bool {
	length := len([]rune(strings.TrimSpace(bio)))
	return length > 0 && length <= v.cfg.Bio.MaxLen
}

func (v Validator) IsValidCategory(category string) bool {
	length := len([]rune(strings.TrimSpace(category)))
	return length > 0 && length <= v.cfg.Category.MaxLen
}

func (v Validator) IsValidPassword(password string) bool {
	length := len(password)
	return length >= v.cfg.Password.MinLen && length <= v.cfg.Password.MaxLen
}

func (v Validator) IsValidPostTitle(title string) bool {
	length := len([]rune(strings.TrimSpace(title)))
	return length > 0 && length <= v.cfg.Post.MaxTitleLen
}

func (v Validator) IsValidPostBody(body string) bool {
	length := len([]rune(strings.TrimSpace(body)))
	return length > 0 && length <= v.cfg.Post.MaxBodyLen
}

// Проверяет тело поста после санитайзинга: не больше потолка и не больше чем в MaxSanitizeExpansion раз относительно исходного
func (v Validator) IsValidSanitizedPostBody(raw, sanitized string) bool {
	rawLen := utf8.RuneCountInString(raw)
	sanitizedLen := utf8.RuneCountInString(sanitized)

	return sanitizedLen <= v.cfg.Post.MaxSanitizedBodyLen &&
		sanitizedLen <= rawLen*v.cfg.Post.MaxSanitizeExpansion
}

func (v Validator) IsValidPostStatus(status string) bool {
	return v.cfg.Post.AllowedPostStatus[strings.TrimSpace(status)]
}

func (v Validator) IsValidPaymentReqKey(reqKey string) bool {
	reqKey = strings.TrimSpace(reqKey)
	_, err := uuid.Parse(reqKey)
	return len([]rune(reqKey)) == v.cfg.Payment.RequestKeyLen && err == nil
}

func (v Validator) IsValidPaymentAmountMinor(amount int64) bool {
	return int64(v.cfg.Payment.MinAmountMinor) <= amount
}

func (v Validator) IsValidPaymentCurrency(cur string) bool {
	unit, err := currency.ParseISO(cur)
	return v.cfg.Payment.AllowedCurrency[unit] && err == nil
}

func (v Validator) IsValidDonationMessage(mes string) bool {
	return len([]rune(strings.TrimSpace(mes))) < v.cfg.Donation.MaxMessageLen
}
