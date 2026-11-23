package config

import (
	"errors"
	"fmt"
	"strings"

	identityclient "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/clients/identity"
	paymentclient "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/clients/payment"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/csrf"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/grpcserver"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/httpserver"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/logger"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/spf13/viper"
)

const DefaultConfigPath = "configs/monetization.yaml"

type Config struct {
	Monetization ServiceConfig              `mapstructure:"monetization"`
	Identity     identityclient.Config      `mapstructure:"identity"`
	Payment      paymentclient.Config       `mapstructure:"payment"`
	Postgres     postgres.PostgresConfig    `mapstructure:"postgres"`
	Log          logger.LoggerConfig        `mapstructure:"log"`
	CSRF         csrf.CSRFConfig            `mapstructure:"csrf"`
	CORS         middleware.CORSConfig      `mapstructure:"cors"`
	Validation   validator.ValidationConfig `mapstructure:"validation"`
}

type ServiceConfig struct {
	HTTP httpserver.HTTPConfig `mapstructure:"http"`
	GRPC grpcserver.GRPCConfig `mapstructure:"grpc"`
}

func DefaultConfig() Config {
	httpCfg := httpserver.DefaultConfig()
	httpCfg.Port = "8084"

	return Config{
		Monetization: ServiceConfig{HTTP: httpCfg, GRPC: grpcserver.DefaultConfig()},
		Identity:     identityclient.DefaultConfig(),
		Payment:      paymentclient.DefaultConfig(),
		Postgres:     postgres.DefaultConfig(),
		Log:          logger.DefaultConfig(),
		CSRF:         csrf.DefaultConfig(),
		CORS:         middleware.DefaultCORSConfig(),
		Validation:   validator.DefaultConfig(),
	}
}

func (c Config) Validate() error {
	return errors.Join(
		c.Monetization.HTTP.Validate(),
		c.Monetization.GRPC.Validate(),
		c.Identity.Validate(),
		c.Payment.Validate(),
		c.Postgres.Validate(),
		c.Log.Validate(),
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

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	cfg := DefaultConfig()
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate monetization config: %w", err)
	}

	return &cfg, nil
}
