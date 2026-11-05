package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/jackc/pgx/v5"
)

const (
	IDField       = "id"
	EmailField    = "email"
	UsernameField = "username"
)

type UserRepository struct {
	pool postgres.DB
}

func NewUserRepository(db *postgres.DBClient) *UserRepository {
	return &UserRepository{pool: db.Pool}
}

func (r *UserRepository) CreateUser(ctx context.Context, u models.User) (*models.User, error) {
	const query = `
		INSERT INTO "user" (username, nickname, email, password_hash, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
	`

	err := r.pool.QueryRow(ctx, query, u.Username, u.Nickname, u.Email, u.PasswordHash, u.Status).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		err = postgres.ParseError(err)
		if errors.Is(err, postgres.ErrUniqViolation) {
			return nil, models.ErrUserExists
		}

		return nil, fmt.Errorf("Create user error: %w", err)
	}

	return &u, nil
}

func (r *UserRepository) UpdateNickname(ctx context.Context, id uuid.UUID, nickname string) (string, error) {
	const query = `
		UPDATE "user"
		SET nickname = $2
		WHERE id = $1
		RETURNING nickname
	`

	var updated string

	err := r.pool.QueryRow(ctx, query, id, nickname).Scan(&updated)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", models.ErrUserNotFound
		}

		return "", fmt.Errorf("update nickname error: %w", err)
	}

	return updated, nil
}

func (r *UserRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return r.getUser(ctx, IDField, id)
}

func (r *UserRepository) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	return r.getUser(ctx, UsernameField, username)
}

func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	return r.getUser(ctx, EmailField, email)
}

func (r *UserRepository) getUser(ctx context.Context, field string, v any) (*models.User, error) {
	const getQuery = `
		SELECT id, username, nickname, email, password_hash, avatar_key, status, created_at, updated_at
		FROM "user"
		WHERE %s = $1
	`

	switch field {
	case IDField:
		v = v.(uuid.UUID)
	case EmailField, UsernameField:
		v = v.(string)
	default:
		return nil, fmt.Errorf("Invalid search field")
	}

	query := fmt.Sprintf(getQuery, field)

	var user models.User
	err := r.pool.QueryRow(ctx, query, v).Scan(
		&user.ID,
		&user.Username,
		&user.Nickname,
		&user.Email,
		&user.PasswordHash,
		&user.AvatarKey,
		&user.Status,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrUserNotFound
		}

		return nil, fmt.Errorf("get user error: %w", err)
	}

	return &user, nil
}
