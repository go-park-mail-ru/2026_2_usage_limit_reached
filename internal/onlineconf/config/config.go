package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/clients/identity"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/csrf"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/grpcserver"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/httpserver"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/logger"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/spf13/viper"
)

const DefaultConfigPath = "configs/onlineconf.yaml"

type Config struct {
	Onlineconf ServiceConfig           `mapstructure:"onlineconf"`
	Postgres   postgres.PostgresConfig `mapstructure:"postgres"`
	Log        logger.LoggerConfig     `mapstructure:"log"`
	CSRF       csrf.CSRFConfig        `mapstructure:"csrf"`
	CORS       middleware.CORSConfig  `mapstructure:"cors"`
	Identity   identityclient.Config  `mapstructure:"identity"`
}

type ServiceConfig struct {
	HTTP httpserver.HTTPConfig `mapstructure:"http"`
	GRPC grpcserver.GRPCConfig `mapstructure:"grpc"`
}

func DefaultConfig() Config {
	httpCfg := httpserver.DefaultConfig()
	httpCfg.Port = "8084"

	grpcCfg := grpcserver.DefaultConfig()
	grpcCfg.Port = "50052"

	return Config{
		Onlineconf: ServiceConfig{HTTP: httpCfg, GRPC: grpcCfg},
		Postgres:   postgres.DefaultConfig(),
		Log:        logger.DefaultConfig(),
		CSRF:       csrf.DefaultConfig(),
		CORS:       middleware.DefaultCORSConfig(),
		Identity:   identityclient.DefaultConfig(),
	}
}

func (c Config) Validate() error {
	return errors.Join(
		c.Onlineconf.HTTP.Validate(),
		c.Onlineconf.GRPC.Validate(),
		c.Postgres.Validate(),
		c.Log.Validate(),
		c.CSRF.Validate(),
		c.CORS.Validate(),
		c.Identity.Validate(),
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
		return nil, fmt.Errorf("validate onlineconf config: %w", err)
	}

	return &cfg, nil
}
