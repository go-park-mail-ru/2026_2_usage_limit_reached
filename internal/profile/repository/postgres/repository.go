package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	profileusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/usecase"
	"github.com/jackc/pgx/v5"
)

type PgxCliPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type AvatarKeyRepository struct {
	pool PgxCliPool
}

func NewAvatarKeyRepository(pool PgxCliPool) *AvatarKeyRepository {
	return &AvatarKeyRepository{pool: pool}
}

func (r *AvatarKeyRepository) UpdateAvatarKey(ctx context.Context, userID uuid.UUID, key *string) (*string, error) {
	const query = `
		UPDATE "user" AS u
		SET avatar_key = $2
		FROM (SELECT id, avatar_key FROM "user" WHERE id = $1 FOR UPDATE) AS old
		WHERE u.id = old.id
		RETURNING old.avatar_key
	`
	var oldKey *string
	err := r.pool.QueryRow(ctx, query, userID, key).Scan(&oldKey)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, profileusecase.ErrProfileNotFound
		}
		return nil, fmt.Errorf("Update avatar key error: %w", err)
	}
	return oldKey, nil
}
