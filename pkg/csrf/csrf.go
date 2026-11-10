package csrf

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"
)

const (
	ContextCookieName = "__Host-csrf"

	authBindingPrefix = "auth:"
	anonBindingPrefix = "anon:"

	minSecretLen = 32
	contextSize  = 32
	saltSize     = 16
)

var (
	ErrInvalidSecret     = errors.New("secret must be at least 32 characters")
	ErrInvalidContextTTL = errors.New("context ttl must be positive")
)

var encoding = base64.RawURLEncoding

type CSRFConfig struct {
	Secret     string        `mapstructure:"secret"`
	ContextTTL time.Duration `mapstructure:"context_ttl"`
}

func DefaultConfig() CSRFConfig {
	return CSRFConfig{
		ContextTTL: 24 * time.Hour,
	}
}

func (c CSRFConfig) Validate() error {
	var errs []error

	if len(c.Secret) < minSecretLen {
		errs = append(errs, fmt.Errorf("csrf: %w, got %d", ErrInvalidSecret, len(c.Secret)))
	}

	if c.ContextTTL <= 0 {
		errs = append(errs, fmt.Errorf("csrf: %w, got %s", ErrInvalidContextTTL, c.ContextTTL))
	}

	return errors.Join(errs...)
}

// Привязка csrf-токена авторизованного пользователя к его JWT
// Новый вход или выход меняет JWT, поэтому старые токены перестают подходить
func AuthBinding(sessionToken string) string {
	return authBindingPrefix + sessionToken
}

// Привязка токена неавторизованного пользователя к анонимному контексту браузера
// Анонимный контекст пользователя нужен, потому что у неавторизованного пользователя нет JWT, а подписать токен чем-то нужно.
// Контекст защищает от атаки login CSRF, если злоумышленнику каким-то образом удалось отправить запрос с заголовком X-CSRF-Token
func AnonBinding(csrfContext string) string {
	return anonBindingPrefix + csrfContext
}

type BindingInput struct {
	SessionToken string
	AnonContext  string
}

func Binding(in BindingInput) (string, bool) {
	if in.SessionToken != "" {
		return AuthBinding(in.SessionToken), true
	}

	if IsValidContext(in.AnonContext) {
		return AnonBinding(in.AnonContext), true
	}

	return "", false
}

func IsValidContext(csrfContext string) bool {
	raw, err := encoding.DecodeString(csrfContext)

	return err == nil && len(raw) == contextSize
}

type Manager struct {
	secret     []byte
	contextTTL time.Duration
}

func NewManager(cfg CSRFConfig) *Manager {
	return &Manager{secret: []byte(cfg.Secret), contextTTL: cfg.ContextTTL}
}

func (m *Manager) ContextTTL() time.Duration {
	return m.contextTTL
}

func (m *Manager) NewContext() (string, error) {
	raw := make([]byte, contextSize)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("csrf new context: %w", err)
	}

	return encoding.EncodeToString(raw), nil
}

func (m *Manager) Resolve(in BindingInput) (string, string, error) {
	if binding, ok := Binding(in); ok {
		return binding, "", nil
	}

	newAnonContext, err := m.NewContext()
	if err != nil {
		return "", "", err
	}

	return AnonBinding(newAnonContext), newAnonContext, nil
}

func (m *Manager) Token(binding string) (string, error) {
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("csrf new token: %w", err)
	}

	return encoding.EncodeToString(append(salt, m.sign(binding, salt)...)), nil
}

func (m *Manager) Verify(binding, token string) bool {
	raw, err := encoding.DecodeString(token)
	if err != nil || len(raw) != saltSize+sha256.Size {
		return false
	}

	salt, signature := raw[:saltSize], raw[saltSize:]

	return hmac.Equal(signature, m.sign(binding, salt))
}

func (m *Manager) sign(binding string, salt []byte) []byte {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(binding))
	mac.Write(salt)

	return mac.Sum(nil)
}
