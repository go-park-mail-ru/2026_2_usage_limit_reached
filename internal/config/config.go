package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/httpserver"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/jwt"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/logger"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/validator"
	"github.com/spf13/viper"
)

const DefaultConfigPath = "configs/dev.yaml"

type Config struct {
	Postgres   postgres.PostgresConfig    `mapstructure:"postgres"`
	Log        logger.LoggerConfig        `mapstructure:"log"`
	HTTP       httpserver.HTTPConfig      `mapstructure:"http"`
	JWT        jwt.JWTConfig              `mapstructure:"jwt"`
	CORS       middleware.CORSConfig      `mapstructure:"cors"`
	Validation validator.ValidationConfig `mapstructure:"validation"`
}

func Load(path string) (*Config, error) {
	v := viper.NewWithOptions(viper.ExperimentalBindStruct())
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config file %s: %w", path, err)
	}

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	cfg := Config{
		Postgres:   postgres.DefaultConfig(),
		Log:        logger.DefaultConfig(),
		HTTP:       httpserver.DefaultConfig(),
		JWT:        jwt.DefaultConfig(),
		CORS:       middleware.DefaultCORSConfig(),
		Validation: validator.DefaultConfig(),
	}
	if err := v.UnmarshalExact(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := errors.Join(
		cfg.Postgres.Validate(),
		cfg.Log.Validate(),
		cfg.HTTP.Validate(),
		cfg.JWT.Validate(),
		cfg.CORS.Validate(),
		cfg.Validation.Validate(),
	); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}
