package user

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/jackc/pgx/v5"
)

const (
	IDField       = "id"
	EmailField    = "email"
	UsernameField = "username"
)

type UserRepository struct {
	pool   postgres.DB
	logger *slog.Logger
}

func NewUserRepository(pool postgres.DB, logger *slog.Logger) *UserRepository {
	return &UserRepository{pool: pool, logger: logger}
}

func (r *UserRepository) CreateUser(ctx context.Context, u *models.User) (*models.User, error) {
	const query = `
		INSERT INTO "user" (username, nickname, email, password_hash, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
	`

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "create user"),
		slog.String("email", u.Email),
	)

	var (
		id        uuid.UUID
		createdAt time.Time
		updatedAt time.Time
	)
	err := r.pool.QueryRow(ctx, query, u.Username, u.Nickname, u.Email, u.PasswordHash, u.Status).Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		err = postgres.ParseError(err)
		if errors.Is(postgres.ErrUniqViolation, err) {
			r.logger.WarnContext(ctx, "failed to create user: already exists",
				slog.String("op", "create user"),
				slog.String("email", u.Email),
			)
			return nil, models.ErrUserExists
		}
		r.logger.ErrorContext(ctx, "failed to create user",
			slog.String("op", "create user"),
			slog.String("email", u.Email),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("Create user error: %w", err)
	}
	u.ID = id
	u.CreatedAt = createdAt
	u.UpdatedAt = updatedAt

	r.logger.InfoContext(ctx, "user created successfully",
		slog.String("op", "create user"),
		slog.String("userID", id.String()),
		slog.String("email", u.Email),
	)
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
		r.logger.ErrorContext(ctx, "failed to get user: invalid search field",
			slog.String("op", "get user"),
			slog.String("field", field),
			slog.Any("value", v),
		)
		return nil, fmt.Errorf("Invalid search field")
	}

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "get user"),
		slog.String("field", field),
		slog.Any("value", v),
	)

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
			r.logger.WarnContext(ctx, "user not found",
				slog.String("op", "get user"),
				slog.String("field", field),
				slog.Any("value", v),
			)
			return nil, models.ErrUserNotFound
		}
		r.logger.ErrorContext(ctx, "failed to get user",
			slog.String("op", "get user"),
			slog.String("field", field),
			slog.Any("value", v),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("Get user error: %w", err)
	}
	r.logger.DebugContext(ctx, "user fetched successfully",
		slog.String("op", "get user"),
		slog.String("userID", user.ID.String()),
		slog.String("field", field),
	)
	return &user, nil
}
