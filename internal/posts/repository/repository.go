package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type PgxCliPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type PostRepository struct {
	pool PgxCliPool
}

func NewUserRepository(pool PgxCliPool) *PostRepository {
	return &PostRepository{pool: pool}
}

func (r *PostRepository) CreatePost(ctx context.Context, p *models.Post) (*models.Post, error) {
	const query = `
		INSERT INTO post (author_id, title, body, status, published_at)
		VALUES ($1, $2, $3, $4, CASE WHEN $4 = 'published' THEN now() END)
		RETURNING id, created_at, updated_at
	`

	var (
		id        uuid.UUID
		createdAt time.Time
		updatedAt time.Time
	)
	err := r.pool.QueryRow(ctx, query, p.AuthorID, p.Title, p.Body, p.Status).Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		err = postgres.ParseError(err)
		if errors.Is(postgres.ErrFKViolation, err) {
			return nil, models.ErrInvalidAuthorID
		}
		return nil, fmt.Errorf("Create post error: %w", err)
	}
	p.ID = id
	p.CreatedAt = createdAt
	p.UpdatedAt = updatedAt
	return p, nil
}

func (r *PostRepository) GetPost(ctx context.Context, userID, postID uuid.UUID) (*dto.PostInfo, error) {
	const query = `
		SELECT p.id, p.author_id, p.title,
			CASE WHEN l.locked THEN '' ELSE p.body END AS body,
			CASE WHEN p.author_id = $1 THEN p.status END AS status,
			p.published_at, p.created_at, p.updated_at, u.username, u.nickname, u.avatar_key, l.locked,
			(SELECT count(*) FROM post_like pl WHERE pl.post_id = p.id) AS likes_count,
			EXISTS (SELECT 1 FROM post_like pl WHERE pl.post_id = p.id AND pl.user_id = $1) AS liked_by_me
		FROM post p
		JOIN "user" u ON u.id = p.author_id AND u.status = 'active'
		LEFT JOIN post_access pa ON pa.post_id = p.id
		CROSS JOIN LATERAL (
			SELECT (pa.post_id IS NOT NULL AND p.author_id IS DISTINCT FROM $1) AS locked
		) l
		WHERE p.id = $2
		AND (p.status = 'published' OR p.author_id = $1)
	`

	var post dto.PostInfo

	err := r.pool.QueryRow(ctx, query, userID, postID).Scan(
		&post.ID,
		&post.AuthorID,
		&post.Title,
		&post.Body,
		&post.Status,
		&post.PublishedAt,
		&post.CreatedAt,
		&post.UpdatedAt,
		&post.AuthorUsername,
		&post.AuthorNickname,
		&post.AuthorAvatarKey,
		&post.Locked,
		&post.LikesCount,
		&post.LikedByUser,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrPostNotFound
		}
		return nil, fmt.Errorf("Get post error: %w", err)
	}
	return &post, nil
}
