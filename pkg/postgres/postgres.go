package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUniqViolation = errors.New("unique constraint violation")
)

type PostgresConfig struct {
	DSN             string
	MinConns        int
	MaxConns        int
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
}

type PgxClient struct {
	Pool *pgxpool.Pool
}

func NewClient(pCfg *PostgresConfig) (*PgxClient, error) {
	cfg, err := pgxpool.ParseConfig(pCfg.DSN)
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
