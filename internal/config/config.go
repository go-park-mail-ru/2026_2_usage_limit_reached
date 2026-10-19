package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/httpserver"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/jwt"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/validator"
	"github.com/spf13/viper"
)

type Config struct {
	Level      string                      `mapstructure:"level"`
	HTTP       *httpserver.HTTPConfig      `mapstructure:"http"`
	JWT        *jwt.JWTConfig              `mapstructure:"jwt"`
	CORS       *middleware.CORSConfig      `mapstructure:"cors"`
	Validation *validator.ValidationConfig `mapstructure:"val"`
	Logger     *slog.Logger                `mapstructure:"-"`
}

const defaultUsernameAllowedRunes = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_"

// Каждая поле-структура конфига, заданное указателем, (напр. HTTP) должна быть создана здесь, т.к. для nil viper не найдёт ключи и env.
func Default() Config {
	return Config{
		Level: "local",
		HTTP: &httpserver.HTTPConfig{
			Port:            "8080",
			Timeout:         5 * time.Second,
			ShutdownTimeout: 25 * time.Second,
		},
		JWT: &jwt.JWTConfig{
			TokenTTL: 24 * time.Hour,
		},
		CORS: &middleware.CORSConfig{
			AllowedOrigins: []string{"http://localhost:3000"},
			AllowedHeaders: "Content-Type, Authorization",
			AllowedMethods: "GET, POST, PUT, DELETE, OPTIONS, PATCH",
		},
		Validation: &validator.ValidationConfig{
			MinUsernameLen:       3,
			MaxUsernameLen:       32,
			MaxNicknameLen:       64,
			MinPasswordLen:       8,
			MaxPasswordLen:       72,
			MaxEmailLen:          254,
			EmailRegexp:          `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`,
			UsernameAllowedRunes: defaultUsernameAllowedRunes,
		},
	}
}

func Load() (*Config, error) {
	v := viper.NewWithOptions(viper.ExperimentalBindStruct())
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	cfg := Default()
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	cfg.Logger = newLogger(cfg.Level)
	return &cfg, nil
}

func (c *Config) validate() error {
	var errs []error

	if c.JWT.Secret == "" {
		errs = append(errs, errors.New("JWT_SECRET is required"))
	}

	val := c.Validation
	positive := []struct {
		name  string
		value int
	}{
		{"VAL_MINUSERNAMELEN", val.MinUsernameLen},
		{"VAL_MAXUSERNAMELEN", val.MaxUsernameLen},
		{"VAL_MAXNICKNAMELEN", val.MaxNicknameLen},
		{"VAL_MINPASSWORDLEN", val.MinPasswordLen},
		{"VAL_MAXPASSWORDLEN", val.MaxPasswordLen},
		{"VAL_MAXEMAILLEN", val.MaxEmailLen},
	}
	for _, p := range positive {
		if p.value <= 0 {
			errs = append(errs, fmt.Errorf("%s must be positive, got %d", p.name, p.value))
		}
	}

	if val.MinUsernameLen > val.MaxUsernameLen {
		errs = append(errs, fmt.Errorf("VAL_MINUSERNAMELEN (%d) > VAL_MAXUSERNAMELEN (%d)", val.MinUsernameLen, val.MaxUsernameLen))
	}
	if val.MinPasswordLen > val.MaxPasswordLen {
		errs = append(errs, fmt.Errorf("VAL_MINPASSWORDLEN (%d) > VAL_MAXPASSWORDLEN (%d)", val.MinPasswordLen, val.MaxPasswordLen))
	}

	return errors.Join(errs...)
}

func newLogger(level string) *slog.Logger {
	var baseHandler slog.Handler
	switch level {
	case "dev":
		baseHandler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	default:
		baseHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	}

	handler := middleware.NewRequestIDHandler(baseHandler)
	return slog.New(handler)
}
