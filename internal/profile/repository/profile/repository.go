package profile

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type ProfileRepository struct {
	pool postgres.DB
}

func NewProfileRepository(db *postgres.DBClient) *ProfileRepository {
	return &ProfileRepository{pool: db.Pool}
}

const selectProfile = `
	SELECT u.id, u.username, u.nickname, u.email, u.avatar_key, u.created_at, u.status, a.bio, a.category
	FROM "user" u
	LEFT JOIN author a ON a.id = u.id
`

func (r *ProfileRepository) GetProfileByID(ctx context.Context, userID uuid.UUID) (*models.Profile, error) {
	const query = selectProfile + `WHERE u.id = $1`

	return scanProfile(r.pool.QueryRow(ctx, query, userID))
}

func (r *ProfileRepository) GetProfileByUsername(ctx context.Context, username string) (*models.Profile, error) {
	const query = selectProfile + `WHERE u.username = $1`

	return scanProfile(r.pool.QueryRow(ctx, query, username))
}

func scanProfile(row pgx.Row) (*models.Profile, error) {
	var (
		profile  models.Profile
		bio      *string
		category *string
	)

	err := row.Scan(
		&profile.UserID,
		&profile.Username,
		&profile.Nickname,
		&profile.Email,
		&profile.AvatarKey,
		&profile.CreatedAt,
		&profile.Status,
		&bio,
		&category,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrProfileNotFound
		}

		return nil, fmt.Errorf("get profile error: %w", err)
	}

	if bio != nil {
		profile.Author = &models.ProfileAuthor{Bio: *bio, Category: category}
	}

	return &profile, nil
}
