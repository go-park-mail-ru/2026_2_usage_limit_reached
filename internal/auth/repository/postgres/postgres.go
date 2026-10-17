package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PgxClient struct {
	Pool *pgxpool.Pool
}

func NewClient(pool *pgxpool.Pool) *PgxClient {
	return &PgxClient{Pool: pool}
}

func (c *PgxClient) Close() {
	c.Pool.Close()
}

func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
