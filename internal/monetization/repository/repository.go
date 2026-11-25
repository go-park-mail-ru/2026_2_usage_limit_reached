package repository

import (
	"log/slog"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
)

type MonetizationRepository struct {
	pool   postgres.DB
	logger *slog.Logger
}

func NewMonetizationRepository(db *postgres.DBClient, logger *slog.Logger) *MonetizationRepository {
	return &MonetizationRepository{
		pool:   db.Pool,
		logger: logger,
	}
}

func equalOptionalString(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}

	return *a == *b
}
