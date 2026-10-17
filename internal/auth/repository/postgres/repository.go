package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	searchID       = "id"
	searchEmail    = "email"
	searchUsername = "username"
)

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) CreateUser(ctx context.Context, u *models.User) (*models.User, error) {
	const query = `
		INSERT INTO "user" (username, nickname, email, password_hash)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, query, u.Username, u.Nickname, u.Email, u.PasswordHash).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, models.ErrUserExists
		}
		return nil, fmt.Errorf("Create user error: %w", err)
	}
	u.ID = id
	return u, nil
}

func (r *UserRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	const query = `
		SELECT id, username, nickname, email, password_hash, COALESCE(avatar_key, '') AS avatar_key, status, created_at, updated_at
		FROM "user"
		WHERE id = $1
	`
	return r.getUser(ctx, query, searchID, id)
}

func (r *UserRepository) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	const query = `
		SELECT id, username, nickname, email, password_hash, COALESCE(avatar_key, '') AS avatar_key, status, created_at, updated_at
		FROM "user"
		WHERE username = $1
	`
	return r.getUser(ctx, query, searchUsername, username)
}

func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	const query = `
		SELECT id, username, nickname, email, password_hash, COALESCE(avatar_key, '') AS avatar_key, status, created_at, updated_at
		FROM "user"
		WHERE email = $1
	`
	return r.getUser(ctx, query, searchEmail, email)
}

func (r *UserRepository) getUser(ctx context.Context, query, field string, v any) (*models.User, error) {
	switch field {
	case "id":
		v = v.(uuid.UUID)
	case "email", "username":
		v = v.(string)
	default:
		return nil, fmt.Errorf("Invalid search field")
	}
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
		return nil, fmt.Errorf("Get user error: %w", err)
	}
	return &user, nil
}
