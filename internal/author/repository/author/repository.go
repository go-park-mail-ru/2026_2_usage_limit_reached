package author

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/author/models"
	"github.com/jackc/pgx/v5"
)

type PgxCliPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type AuthorRepository struct {
	pool PgxCliPool
}

func NewAuthorRepository(pool PgxCliPool) *AuthorRepository {
	return &AuthorRepository{pool: pool}
}

func (r *AuthorRepository) GetAuthorByID(ctx context.Context, id uuid.UUID) (*models.Author, error) {
	const query = `
		SELECT id, bio, category, payout_provider, payout_account_id, created_at, updated_at
		FROM author
		WHERE id = $1
	`

	var author models.Author
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&author.ID,
		&author.Bio,
		&author.Category,
		&author.PayoutProvider,
		&author.PayoutAccountID,
		&author.CreatedAt,
		&author.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrAuthorNotFound
		}
		return nil, fmt.Errorf("Get author error: %w", err)
	}
	return &author, nil
}
