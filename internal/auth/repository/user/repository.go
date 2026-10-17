package user

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/repository/postgres"
	"github.com/jackc/pgx/v5"
)

const (
	IDField       = "id"
	EmailField    = "email"
	UsernameField = "username"
	getQuery      = `
		SELECT id, username, nickname, email, password_hash, COALESCE(avatar_key, '') AS avatar_key, status, created_at, updated_at
		FROM "user"
		WHERE %s = $1
	`
)

type PgxCliPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type UserRepository struct {
	pool PgxCliPool
}

func NewUserRepository(pool PgxCliPool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) CreateUser(ctx context.Context, u *models.User) (*models.User, error) {
	const query = `
		INSERT INTO "user" (username, nickname, email, password_hash, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, query, u.Username, u.Nickname, u.Email, u.PasswordHash, time.Now()).Scan(&id)
	if err != nil {
		if postgres.IsUniqueViolation(err) {
			return nil, models.ErrUserExists
		}
		return nil, fmt.Errorf("Create user error: %w", err)
	}
	u.ID = id
	return u, nil
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
		return nil, fmt.Errorf("Get user error: %w", err)
	}
	return &user, nil
}
