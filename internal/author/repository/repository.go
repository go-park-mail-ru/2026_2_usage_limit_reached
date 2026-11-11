package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/author/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/jackc/pgx/v5"
)

type AuthorRepository struct {
	pool   postgres.DB
	logger *slog.Logger
}

func NewAuthorRepository(db *postgres.DBClient, logger *slog.Logger) *AuthorRepository {
	return &AuthorRepository{pool: db.Pool, logger: logger}
}

func (r *AuthorRepository) GetAuthorByID(ctx context.Context, id uuid.UUID) (*models.Author, error) {
	const query = `
		SELECT id, bio, category, payout_provider, payout_account_id, created_at, updated_at
		FROM author
		WHERE id = $1
	`

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "get author by ID"),
		slog.String("authorID", id.String()),
	)

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

func (r *AuthorRepository) UpdateAuthor(ctx context.Context, id uuid.UUID, details models.AuthorDetails) (*models.Author, error) {
	const query = `
		UPDATE author
		SET bio = $2, category = $3
		WHERE id = $1
		RETURNING id, bio, category
	`

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "update author"),
		slog.String("authorID", id.String()),
	)

	var author models.Author

	err := r.pool.QueryRow(ctx, query, id, details.Bio, details.Category).Scan(&author.ID, &author.Bio, &author.Category)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrAuthorNotFound
		}

		return nil, fmt.Errorf("update author error: %w", err)
	}

	return &author, nil
}

func (r *AuthorRepository) CreateAuthor(ctx context.Context, id uuid.UUID, details models.AuthorDetails) (*models.Author, error) {
	const query = `
		INSERT INTO author (id, bio, category)
		VALUES ($1, $2, $3)
		ON CONFLICT (id) DO NOTHING
		RETURNING id, bio, category
	`

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "create author"),
		slog.String("authorID", id.String()),
	)

	var author models.Author

	err := r.pool.QueryRow(ctx, query, id, details.Bio, details.Category).Scan(&author.ID, &author.Bio, &author.Category)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrAuthorExists
		}

		return nil, fmt.Errorf("create author error: %w", err)
	}

	return &author, nil
}
