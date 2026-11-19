package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/csrf"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/httpserver"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/jwt"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/logger"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/spf13/viper"
)

const DefaultConfigPath = "configs/dev.yaml"

type Config struct {
	Gateway GatewayConfig         `mapstructure:"gateway"`
	Log     logger.LoggerConfig   `mapstructure:"log"`
	HTTP    httpserver.HTTPConfig `mapstructure:"http"`
	JWT     jwt.JWTConfig         `mapstructure:"jwt"`
	CSRF    csrf.CSRFConfig       `mapstructure:"csrf"`
	CORS    middleware.CORSConfig `mapstructure:"cors"`
}

type GatewayConfig struct {
	Identity IdentityClientConfig `mapstructure:"identity"`
}

type IdentityClientConfig struct {
	Address string        `mapstructure:"address"`
	Timeout time.Duration `mapstructure:"timeout"`
}

func DefaultConfig() Config {
	return Config{
		Gateway: GatewayConfig{
			Identity: IdentityClientConfig{
				Address: "localhost:50051",
				Timeout: 5 * time.Second,
			},
		},
		Log:  logger.DefaultConfig(),
		HTTP: httpserver.DefaultConfig(),
		JWT:  jwt.DefaultConfig(),
		CSRF: csrf.DefaultConfig(),
		CORS: middleware.DefaultCORSConfig(),
	}
}

func (c Config) Validate() error {
	var errs []error

	if strings.TrimSpace(c.Gateway.Identity.Address) == "" {
		errs = append(errs, errors.New("gateway: identity.address must not be empty"))
	}
	if c.Gateway.Identity.Timeout <= 0 {
		errs = append(errs, fmt.Errorf("gateway: identity.timeout must be positive, got %s", c.Gateway.Identity.Timeout))
	}

	errs = append(errs,
		c.Log.Validate(),
		c.HTTP.Validate(),
		c.JWT.Validate(),
		c.CSRF.Validate(),
		c.CORS.Validate(),
	)

	return errors.Join(errs...)
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
		return nil, fmt.Errorf("validate gateway config: %w", err)
	}

	return &cfg, nil
}
