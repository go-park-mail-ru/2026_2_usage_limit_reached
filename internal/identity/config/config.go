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

const DefaultConfigPath = "configs/identity.yaml"

type Config struct {
	Identity   ServiceConfig              `mapstructure:"identity"`
	Admin      AdminConfig                `mapstructure:"admin"`
	Postgres   postgres.PostgresConfig    `mapstructure:"postgres"`
	S3         s3.S3Config                `mapstructure:"s3"`
	Log        logger.LoggerConfig        `mapstructure:"log"`
	JWT        jwt.JWTConfig              `mapstructure:"jwt"`
	CSRF       csrf.CSRFConfig            `mapstructure:"csrf"`
	CORS       middleware.CORSConfig      `mapstructure:"cors"`
	Validation validator.ValidationConfig `mapstructure:"validation"`
}

type AdminConfig struct {
	Email    string `mapstructure:"email"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

type ServiceConfig struct {
	HTTP httpserver.HTTPConfig `mapstructure:"http"`
	GRPC grpcserver.GRPCConfig `mapstructure:"grpc"`
}

func DefaultConfig() Config {
	httpCfg := httpserver.DefaultConfig()
	httpCfg.Port = "8081"

	return Config{
		Identity:   ServiceConfig{HTTP: httpCfg, GRPC: grpcserver.DefaultConfig()},
		Postgres:   postgres.DefaultConfig(),
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
		c.Identity.HTTP.Validate(),
		c.Identity.GRPC.Validate(),
		c.Admin.Validate(),
		c.Postgres.Validate(),
		c.S3.Validate(),
		c.Log.Validate(),
		c.JWT.Validate(),
		c.CSRF.Validate(),
		c.CORS.Validate(),
		c.Validation.Validate(),
	)
}

func (c AdminConfig) Validate() error {
	if strings.TrimSpace(c.Email) == "" || strings.TrimSpace(c.Username) == "" || c.Password == "" {
		return errors.New("admin email, username, and password must be configured")
	}
	return nil
}

func Load(path string) (*Config, error) {
	v := viper.NewWithOptions(viper.ExperimentalBindStruct())
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config file %s: %w", path, err)
	}

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	if err := v.BindEnv("admin.email", "IDENTITY_ADMIN_EMAIL"); err != nil {
		return nil, fmt.Errorf("bind admin email environment variable: %w", err)
	}
	if err := v.BindEnv("admin.username", "IDENTITY_ADMIN_USERNAME"); err != nil {
		return nil, fmt.Errorf("bind admin username environment variable: %w", err)
	}
	if err := v.BindEnv("admin.password", "IDENTITY_ADMIN_PASSWORD"); err != nil {
		return nil, fmt.Errorf("bind admin password environment variable: %w", err)
	}

	cfg := DefaultConfig()
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate identity config: %w", err)
	}

	return &cfg, nil
}
