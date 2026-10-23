package postgres

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUniqViolation = errors.New("unique constraint violation")
)

type PostgresConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	DB              string        `mapstructure:"db"`
	SSLMode         string        `mapstructure:"sslmode"`
	MinConns        int           `mapstructure:"min_conns"`
	MaxConns        int           `mapstructure:"max_conns"`
	MaxConnLifetime time.Duration `mapstructure:"max_conn_lifetime"`
	MaxConnIdleTime time.Duration `mapstructure:"max_conn_idle_time"`
}

func DefaultConfig() PostgresConfig {
	return PostgresConfig{
		Host:            "postgres",
		Port:            5432,
		SSLMode:         "disable",
		MinConns:        0,
		MaxConns:        10,
		MaxConnLifetime: time.Hour,
		MaxConnIdleTime: 30 * time.Minute,
	}
}

func (c PostgresConfig) DSN() string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.User, c.Password),
		Host:     net.JoinHostPort(c.Host, strconv.Itoa(c.Port)),
		Path:     "/" + c.DB,
		RawQuery: url.Values{"sslmode": {c.SSLMode}}.Encode(),
	}
	return u.String()
}

func (c PostgresConfig) Validate() error {
	var errs []error
	if c.Host == "" {
		errs = append(errs, errors.New("postgres: host must not be empty"))
	}
	if c.Port <= 0 || c.Port > 65535 {
		errs = append(errs, fmt.Errorf("postgres: port must be a number from 1 to 65535, got %d", c.Port))
	}
	if c.User == "" {
		errs = append(errs, errors.New("postgres: user must not be empty"))
	}
	if c.Password == "" {
		errs = append(errs, errors.New("postgres: password must not be empty"))
	}
	if c.DB == "" {
		errs = append(errs, errors.New("postgres: db must not be empty"))
	}
	if c.MinConns < 0 {
		errs = append(errs, fmt.Errorf("postgres: min_conns must be >= 0, got %d", c.MinConns))
	}
	if c.MaxConns <= 0 {
		errs = append(errs, fmt.Errorf("postgres: max_conns must be positive, got %d", c.MaxConns))
	}
	if c.MaxConns < c.MinConns {
		errs = append(errs, fmt.Errorf("postgres: max_conns (%d) must be >= min_conns (%d)", c.MaxConns, c.MinConns))
	}
	if c.MaxConnLifetime < 0 {
		errs = append(errs, fmt.Errorf("postgres: max_conn_lifetime must be >= 0, got %s", c.MaxConnLifetime))
	}
	if c.MaxConnIdleTime < 0 {
		errs = append(errs, fmt.Errorf("postgres: max_conn_idle_time must be >= 0, got %s", c.MaxConnIdleTime))
	}
	return errors.Join(errs...)
}

type PgxClient struct {
	Pool *pgxpool.Pool
}

func NewClient(pCfg PostgresConfig) (*PgxClient, error) {
	cfg, err := pgxpool.ParseConfig(pCfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	cfg.MinConns = int32(pCfg.MinConns)
	cfg.MaxConns = int32(pCfg.MaxConns)
	cfg.MaxConnLifetime = pCfg.MaxConnLifetime
	cfg.MaxConnIdleTime = pCfg.MaxConnIdleTime

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("database new pool error: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("database ping error: %w", err)
	}

	return &PgxClient{Pool: pool}, nil
}

func (c *PgxClient) Close() {
	c.Pool.Close()
}

func ParseError(err error) error {
	var pgErr *pgconn.PgError
	switch {
	case errors.As(err, &pgErr) && pgErr.Code == "23505":
		return ErrUniqViolation
	default:
		return fmt.Errorf("postgres error: %w", err)
	}
}
