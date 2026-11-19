package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/csrf"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/grpcserver"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/httpserver"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/jwt"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/logger"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/s3"
	"github.com/spf13/viper"
)

const (
	DefaultConfigPath = "configs/identity.yaml"
	envPrefix         = "IDENTITY"
)

type Config struct {
	Postgres   postgres.PostgresConfig    `mapstructure:"postgres"`
	GRPC       grpcserver.GRPCConfig      `mapstructure:"grpc"`
	HTTP       httpserver.HTTPConfig      `mapstructure:"http"`
	S3         s3.S3Config                `mapstructure:"s3"`
	Log        logger.LoggerConfig        `mapstructure:"log"`
	JWT        jwt.JWTConfig              `mapstructure:"jwt"`
	CSRF       csrf.CSRFConfig            `mapstructure:"csrf"`
	CORS       middleware.CORSConfig      `mapstructure:"cors"`
	Validation validator.ValidationConfig `mapstructure:"validation"`
}

func DefaultConfig() Config {
	httpCfg := httpserver.DefaultConfig()
	httpCfg.Port = "8081"

	return Config{
		Postgres:   postgres.DefaultConfig(),
		GRPC:       grpcserver.DefaultConfig(),
		HTTP:       httpCfg,
		S3:         s3.DefaultConfig(),
		Log:        logger.DefaultConfig(),
		JWT:        jwt.DefaultConfig(),
		CSRF:       csrf.DefaultConfig(),
		CORS:       middleware.DefaultCORSConfig(),
		Validation: validator.DefaultConfig(),
	}
}

func (c Config) Validate() error {
	return errors.Join(
		c.Postgres.Validate(),
		c.GRPC.Validate(),
		c.HTTP.Validate(),
		c.S3.Validate(),
		c.Log.Validate(),
		c.JWT.Validate(),
		c.CSRF.Validate(),
		c.CORS.Validate(),
		c.Validation.Validate(),
	)
}

func Load(path string) (*Config, error) {
	v := viper.NewWithOptions(viper.ExperimentalBindStruct())
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config file %s: %w", path, err)
	}

	v.SetEnvPrefix(envPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	cfg := DefaultConfig()
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate identity config: %w", err)
	}

	return &cfg, nil
}
