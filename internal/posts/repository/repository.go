package postrepository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type PostRepository struct {
	pool   postgres.DB
	logger *slog.Logger
}

func NewPostRepository(db *postgres.DBClient, logger *slog.Logger) *PostRepository {
	return &PostRepository{pool: db.Pool, logger: logger}
}

func (r *PostRepository) CreatePost(ctx context.Context, userID *uuid.UUID, p models.Post) (uuid.UUID, error) {
	const query = `
		INSERT INTO post (author_id, title, body, status, published_at)
		VALUES ($1, $2, $3, $4, CASE WHEN $4 = 'published' THEN now() END)
		RETURNING id
	`

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "create post"),
		slog.String("userID", userID.String()),
	)

	var postID uuid.UUID
	err := r.pool.QueryRow(ctx, query, userID, p.Title, p.Body, p.Status).Scan(&postID)
	if err != nil {
		err = postgres.ParseError(err)
		switch {
		case errors.Is(err, postgres.ErrFKViolation):
			return uuid.Nil, models.ErrInvalidAuthorID
		case errors.Is(err, postgres.ErrCheckViolation):
			return uuid.Nil, models.ErrPostValidation
		}
		return uuid.Nil, fmt.Errorf("create post error: %w", err)
	}
	return postID, nil
}

func (r *PostRepository) GetPost(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) (*dto.PostInfo, error) {
	const query = `
		SELECT p.id, p.author_id, p.title,
			CASE WHEN l.locked THEN '' ELSE p.body END AS body,
			p.status, p.published_at, p.likes_count, p.created_at, p.updated_at, u.username, u.nickname, u.avatar_key, l.locked,
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

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "get post"),
		slog.String("postID", postID.String()),
		slog.String("userID", uid(userID)),
	)

	post := &dto.PostInfo{}
	err := r.pool.QueryRow(ctx, query, userID, postID).Scan(
		&post.ID,
		&post.Author.ID,
		&post.Title,
		&post.Body,
		&post.Status,
		&post.PublishedAt,
		&post.LikesCount,
		&post.CreatedAt,
		&post.UpdatedAt,
		&post.Author.Username,
		&post.Author.Nickname,
		&post.Author.AvatarUrl,
		&post.Locked,
		&post.LikedByUser,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrPostNotFound
		}
		return nil, fmt.Errorf("get post error: %w", err)
	}
	return post, nil
}

func (r *PostRepository) GetMyNewPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) ([]*dto.PostPreview, error) {
	const query = `
		SELECT 
			p.id, p.author_id, p.title, left(p.body, 200) AS body_preview, p.published_at, p.likes_count,
			u.username, u.nickname, u.avatar_key, FALSE,
			EXISTS (SELECT 1 FROM post_like pl WHERE pl.post_id = p.id AND pl.user_id = $1) AS liked_by_me
		FROM post p
		JOIN "user" u ON u.id = p.author_id
		WHERE p.status = 'published'
			AND ($2::timestamptz IS NULL OR (p.published_at, p.id) < ($2::timestamptz, $3::uuid))
			AND p.author_id = $1
		ORDER BY p.published_at DESC, p.id DESC
		LIMIT $4
	`

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "get my new posts"),
		slog.String("userID", userID.String()),
		slog.Int("limit", limit),
		slog.Bool("hasCursor", cursor != nil),
	)

	posts, err := r.getTimedPosts(ctx, query, userID, cursor, limit)
	if err != nil {
		return nil, fmt.Errorf("get my new posts error: %w", err)
	}
	return posts, nil
}

func (r *PostRepository) GetMyOldPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) ([]*dto.PostPreview, error) {
	const query = `
		SELECT 
			p.id, p.author_id, p.title, left(p.body, 200) AS body_preview, p.published_at, p.likes_count,
			u.username, u.nickname, u.avatar_key, FALSE,
			EXISTS (SELECT 1 FROM post_like pl WHERE pl.post_id = p.id AND pl.user_id = $1) AS liked_by_me
		FROM post p
		JOIN "user" u ON u.id = p.author_id
		WHERE p.status = 'published'
			AND ($2::timestamptz IS NULL OR (p.published_at, p.id) > ($2::timestamptz, $3::uuid))
			AND p.author_id = $1
		ORDER BY p.published_at, p.id
		LIMIT $4
	`

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "get my old posts"),
		slog.String("userID", userID.String()),
		slog.Int("limit", limit),
		slog.Bool("hasCursor", cursor != nil),
	)

	posts, err := r.getTimedPosts(ctx, query, userID, cursor, limit)
	if err != nil {
		return nil, fmt.Errorf("get my old posts error: %w", err)
	}
	return posts, nil
}

func (r *PostRepository) GetMyPopularPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.PopularPostsCursor, limit int) ([]*dto.PostPreview, error) {
	const query = `
		SELECT 
			p.id, p.author_id, p.title, left(p.body, 200) AS body_preview, p.published_at, p.likes_count,
			u.username, u.nickname, u.avatar_key, FALSE,
			EXISTS (SELECT 1 FROM post_like pl WHERE pl.post_id = p.id AND pl.user_id = $1) AS liked_by_me
		FROM post p
		JOIN "user" u ON u.id = p.author_id
		WHERE p.status = 'published'
				AND ($2::bigint IS NULL OR (p.likes_count, p.published_at, p.id) < ($2::bigint, $3::timestamptz, $4::uuid))
				AND p.author_id = $1
		ORDER BY p.likes_count DESC, p.published_at DESC, p.id DESC
		LIMIT $5
	`

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "get my popular posts"),
		slog.String("userID", userID.String()),
		slog.Int("limit", limit),
		slog.Bool("hasCursor", cursor != nil),
	)

	posts, err := r.getPopularPosts(ctx, query, userID, cursor, limit)
	if err != nil {
		return nil, fmt.Errorf("get my popular posts error: %w", err)
	}
	return posts, nil
}

func (r *PostRepository) GetMyNewDrafts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) ([]*dto.PostPreview, error) {
	const query = `
		SELECT 
			p.id, p.author_id, p.title, left(p.body, 200) AS body_preview, p.updated_at, p.likes_count,
			u.username, u.nickname, u.avatar_key, FALSE, FALSE
		FROM post p
		JOIN "user" u ON u.id = p.author_id
		WHERE p.status = 'draft'
			AND ($2::timestamptz IS NULL OR (p.updated_at, p.id) < ($2::timestamptz, $3::uuid))
			AND p.author_id = $1
		ORDER BY p.updated_at DESC, p.id DESC
		LIMIT $4
	`

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "get my new drafts"),
		slog.String("userID", userID.String()),
		slog.Int("limit", limit),
		slog.Bool("hasCursor", cursor != nil),
	)

	posts, err := r.getTimedPosts(ctx, query, userID, cursor, limit)
	if err != nil {
		return nil, fmt.Errorf("get my new drafts error: %w", err)
	}
	return posts, nil
}

func (r *PostRepository) GetMyOldDrafts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) ([]*dto.PostPreview, error) {
	const query = `
		SELECT 
			p.id, p.author_id, p.title, left(p.body, 200) AS body_preview, p.updated_at, p.likes_count,
			u.username, u.nickname, u.avatar_key, FALSE, FALSE
		FROM post p
		JOIN "user" u ON u.id = p.author_id
		WHERE p.status = 'draft'
			AND ($2::timestamptz IS NULL OR (p.updated_at, p.id) > ($2::timestamptz, $3::uuid))
			AND p.author_id = $1
		ORDER BY p.updated_at, p.id
		LIMIT $4
	`

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "get my old drafts"),
		slog.String("userID", userID.String()),
		slog.Int("limit", limit),
		slog.Bool("hasCursor", cursor != nil),
	)

	posts, err := r.getTimedPosts(ctx, query, userID, cursor, limit)
	if err != nil {
		return nil, fmt.Errorf("get my old drafts error: %w", err)
	}
	return posts, nil
}

func (r *PostRepository) GetUserNewPosts(ctx context.Context, userID *uuid.UUID, authorUsername string, cursor *dto.TimedPostsCursor, limit int) ([]*dto.PostPreview, error) {
	const query = `
		WITH page AS (
			SELECT p.id, p.author_id, p.title, left(p.body, 200) AS body_preview,
			       p.published_at, p.likes_count,
			       u.username, u.nickname, u.avatar_key
			FROM post p
			JOIN "user" u ON u.id = p.author_id
			WHERE p.status = 'published'
			  AND u.username = $2
			  AND ($3::timestamptz IS NULL OR (p.published_at, p.id) < ($3::timestamptz, $4::uuid))
			ORDER BY p.published_at DESC, p.id DESC
			LIMIT $5
		)
		SELECT p.id, p.author_id, p.title, CASE WHEN l.locked THEN '' ELSE p.body_preview END AS body_preview,
		       p.published_at, p.likes_count, p.username, p.nickname, p.avatar_key,
		       l.locked,
		       EXISTS (SELECT 1 FROM post_like pl WHERE pl.post_id = p.id AND pl.user_id = $1) AS liked_by_me
		FROM page p
		LEFT JOIN post_access pa ON pa.post_id = p.id
		CROSS JOIN LATERAL (
			SELECT (pa.post_id IS NOT NULL AND p.author_id IS DISTINCT FROM $1) AS locked
		) l
		ORDER BY p.published_at DESC, p.id DESC
	`

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "get user new posts"),
		slog.String("author_username", authorUsername),
		slog.String("userID", uid(userID)),
		slog.Int("limit", limit),
		slog.Bool("hasCursor", cursor != nil),
	)

	var (
		ts *time.Time
		id *uuid.UUID
	)
	if cursor != nil {
		ts, id = &cursor.PublishedOrUpdatedAt, &cursor.PostID
	}

	rows, err := r.pool.Query(ctx, query, userID, authorUsername, ts, id, limit)
	if err != nil {
		return nil, fmt.Errorf("get user new posts error: %w", err)
	}

	posts, err := scanPosts(rows)
	if err != nil {
		return nil, fmt.Errorf("get user new posts error: %w", err)
	}

	return posts, nil
}

func (r *PostRepository) GetUserOldPosts(ctx context.Context, userID *uuid.UUID, authorUsername string, cursor *dto.TimedPostsCursor, limit int) ([]*dto.PostPreview, error) {
	const query = `
		WITH page AS (
			SELECT p.id, p.author_id, p.title, left(p.body, 200) AS body_preview,
			       p.published_at, p.likes_count,
			       u.username, u.nickname, u.avatar_key
			FROM post p
			JOIN "user" u ON u.id = p.author_id
			WHERE p.status = 'published'
			  AND u.username = $2
			  AND ($3::timestamptz IS NULL OR (p.published_at, p.id) > ($3::timestamptz, $4::uuid))
			ORDER BY p.published_at ASC, p.id ASC
			LIMIT $5
		)
		SELECT p.id, p.author_id, p.title, CASE WHEN l.locked THEN '' ELSE p.body_preview END AS body_preview,
		       p.published_at, p.likes_count, p.username, p.nickname, p.avatar_key,
		       l.locked,
		       EXISTS (SELECT 1 FROM post_like pl WHERE pl.post_id = p.id AND pl.user_id = $1) AS liked_by_me
		FROM page p
		LEFT JOIN post_access pa ON pa.post_id = p.id
		CROSS JOIN LATERAL (
			SELECT (pa.post_id IS NOT NULL AND p.author_id IS DISTINCT FROM $1) AS locked
		) l
		ORDER BY p.published_at ASC, p.id ASC
	`

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "get user old posts"),
		slog.String("author_username", authorUsername),
		slog.String("userID", uid(userID)),
		slog.Int("limit", limit),
		slog.Bool("hasCursor", cursor != nil),
	)

	var (
		ts *time.Time
		id *uuid.UUID
	)
	if cursor != nil {
		ts, id = &cursor.PublishedOrUpdatedAt, &cursor.PostID
	}

	rows, err := r.pool.Query(ctx, query, userID, authorUsername, ts, id, limit)
	if err != nil {
		return nil, fmt.Errorf("get user old posts error: %w", err)
	}

	posts, err := scanPosts(rows)
	if err != nil {
		return nil, fmt.Errorf("get user old posts error: %w", err)
	}

	return posts, nil
}

func (r *PostRepository) GetUserPopularPosts(ctx context.Context, userID *uuid.UUID, authorUsername string, cursor *dto.PopularPostsCursor, limit int) ([]*dto.PostPreview, error) {
	const query = `
		WITH page AS (
			SELECT p.id, p.author_id, p.title, left(p.body, 200) AS body_preview,
			       p.published_at, p.likes_count,
			       u.username, u.nickname, u.avatar_key
			FROM post p
			JOIN "user" u ON u.id = p.author_id
			WHERE p.status = 'published'
			  AND u.username = $2
			  AND ($3::bigint IS NULL OR (p.likes_count, p.published_at, p.id) < ($3::bigint, $4::timestamptz, $5::uuid))
			ORDER BY p.likes_count DESC, p.published_at DESC, p.id DESC
			LIMIT $6
		)
		SELECT p.id, p.author_id, p.title, CASE WHEN l.locked THEN '' ELSE p.body_preview END AS body_preview,
		       p.published_at, p.likes_count, p.username, p.nickname, p.avatar_key,
		       l.locked,
		       EXISTS (SELECT 1 FROM post_like pl WHERE pl.post_id = p.id AND pl.user_id = $1) AS liked_by_me
		FROM page p
		LEFT JOIN post_access pa ON pa.post_id = p.id
		CROSS JOIN LATERAL (
			SELECT (pa.post_id IS NOT NULL AND p.author_id IS DISTINCT FROM $1) AS locked
		) l
		ORDER BY p.likes_count DESC, p.published_at DESC, p.id DESC
	`

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "get user popular posts"),
		slog.String("author_username", authorUsername),
		slog.String("userID", uid(userID)),
		slog.Int("limit", limit),
		slog.Bool("hasCursor", cursor != nil),
	)

	var (
		likes *int64
		ts    *time.Time
		id    *uuid.UUID
	)
	if cursor != nil {
		likes, ts, id = &cursor.Likes, &cursor.PublishedAt, &cursor.PostID
	}

	rows, err := r.pool.Query(ctx, query, userID, authorUsername, likes, ts, id, limit)
	if err != nil {
		return nil, fmt.Errorf("get user popular posts error: %w", err)
	}

	posts, err := scanPosts(rows)
	if err != nil {
		return nil, fmt.Errorf("get user popular posts error: %w", err)
	}

	return posts, nil
}

func (r *PostRepository) GetNewPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) ([]*dto.PostPreview, error) {
	const query = `
		WITH page AS (
			SELECT p.id, p.author_id, p.title, left(p.body, 200) AS body_preview,
				p.published_at, p.likes_count, u.username, u.nickname, u.avatar_key
			FROM post p
			JOIN "user" u ON u.id = p.author_id
			WHERE p.status = 'published' AND u.status = 'active'
				AND ($2::timestamptz IS NULL OR (p.published_at, p.id) < ($2::timestamptz, $3::uuid))
			ORDER BY p.published_at DESC, p.id DESC
			LIMIT $4
		)
		SELECT 
			p.id, p.author_id, p.title,
			CASE WHEN l.locked THEN '' ELSE p.body_preview END AS body_preview,
			p.published_at, p.likes_count,
			p.username, p.nickname, p.avatar_key, l.locked,
			EXISTS (SELECT 1 FROM post_like pl WHERE pl.post_id = p.id AND pl.user_id = $1) AS liked_by_me
		FROM page p
		LEFT JOIN post_access pa ON pa.post_id = p.id
		CROSS JOIN LATERAL (
			SELECT (pa.post_id IS NOT NULL AND p.author_id IS DISTINCT FROM $1) AS locked
		) l
		ORDER BY p.published_at DESC, p.id DESC
	`

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "get new posts"),
		slog.String("userID", uid(userID)),
		slog.Int("limit", limit),
		slog.Bool("hasCursor", cursor != nil),
	)

	posts, err := r.getTimedPosts(ctx, query, userID, cursor, limit)
	if err != nil {
		return nil, fmt.Errorf("get new posts error: %w", err)
	}
	return posts, nil
}

func (r *PostRepository) GetOldPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) ([]*dto.PostPreview, error) {
	const query = `
		WITH page AS (
			SELECT p.id, p.author_id, p.title, left(p.body, 200) AS body_preview,
				p.published_at, p.likes_count, u.username, u.nickname, u.avatar_key
			FROM post p
			JOIN "user" u ON u.id = p.author_id
			WHERE p.status = 'published' AND u.status = 'active'
				AND ($2::timestamptz IS NULL OR (p.published_at, p.id) > ($2::timestamptz, $3::uuid))
			ORDER BY p.published_at, p.id
			LIMIT $4
		)
		SELECT 
			p.id, p.author_id, p.title,
			CASE WHEN l.locked THEN '' ELSE p.body_preview END AS body_preview,
			p.published_at, p.likes_count,
			p.username, p.nickname, p.avatar_key, l.locked,
			EXISTS (SELECT 1 FROM post_like pl WHERE pl.post_id = p.id AND pl.user_id = $1) AS liked_by_me
		FROM page p
		LEFT JOIN post_access pa ON pa.post_id = p.id
		CROSS JOIN LATERAL (
			SELECT (pa.post_id IS NOT NULL AND p.author_id IS DISTINCT FROM $1) AS locked
		) l
		ORDER BY p.published_at, p.id
	`

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "get old posts"),
		slog.String("userID", uid(userID)),
		slog.Int("limit", limit),
		slog.Bool("hasCursor", cursor != nil),
	)

	posts, err := r.getTimedPosts(ctx, query, userID, cursor, limit)
	if err != nil {
		return nil, fmt.Errorf("get old posts error: %w", err)
	}
	return posts, nil
}

func (r *PostRepository) GetPopularPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.PopularPostsCursor, limit int) ([]*dto.PostPreview, error) {
	const query = `
		WITH page AS (
			SELECT p.id, p.author_id, p.title, left(p.body, 200) AS body_preview,
			p.published_at, p.likes_count, u.username, u.nickname, u.avatar_key
			FROM post p
			JOIN "user" u ON u.id = p.author_id
			WHERE p.status = 'published' AND u.status = 'active'
				AND ($2::bigint IS NULL OR (p.likes_count, p.published_at, p.id) < ($2::bigint, $3::timestamptz, $4::uuid))
			ORDER BY p.likes_count DESC, p.published_at DESC, p.id DESC
			LIMIT $5
		)
		SELECT p.id, p.author_id, p.title,
				CASE WHEN l.locked THEN '' ELSE p.body_preview END AS body_preview,
				p.published_at, p.likes_count, p.username, p.nickname, p.avatar_key, l.locked,
				EXISTS (SELECT 1 FROM post_like pl WHERE pl.post_id = p.id AND pl.user_id = $1) AS liked_by_me
		FROM page p
		LEFT JOIN post_access pa ON pa.post_id = p.id
		CROSS JOIN LATERAL (
			SELECT (pa.post_id IS NOT NULL AND p.author_id IS DISTINCT FROM $1) AS locked
		) l
		ORDER BY p.likes_count DESC, p.published_at DESC, p.id DESC
	`

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "get popular posts"),
		slog.String("userID", uid(userID)),
		slog.Int("limit", limit),
		slog.Bool("hasCursor", cursor != nil),
	)

	posts, err := r.getPopularPosts(ctx, query, userID, cursor, limit)
	if err != nil {
		return nil, fmt.Errorf("get popular posts error: %w", err)
	}
	return posts, nil
}

func (r *PostRepository) getTimedPosts(ctx context.Context, query string, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) ([]*dto.PostPreview, error) {
	var (
		ts *time.Time
		id *uuid.UUID
	)
	if cursor != nil {
		ts, id = &cursor.PublishedOrUpdatedAt, &cursor.PostID
	}

	rows, err := r.pool.Query(ctx, query, userID, ts, id, limit)
	if err != nil {
		return nil, err
	}
	posts, err := scanPosts(rows)
	if err != nil {
		return nil, err
	}
	return posts, nil
}

func (r *PostRepository) getPopularPosts(ctx context.Context, query string, userID *uuid.UUID, cursor *dto.PopularPostsCursor, limit int) ([]*dto.PostPreview, error) {
	var (
		likes *int64
		ts    *time.Time
		id    *uuid.UUID
	)
	if cursor != nil {
		likes, ts, id = &cursor.Likes, &cursor.PublishedAt, &cursor.PostID
	}

	rows, err := r.pool.Query(ctx, query, userID, likes, ts, id, limit)
	if err != nil {
		return nil, err
	}
	posts, err := scanPosts(rows)
	if err != nil {
		return nil, err
	}
	return posts, nil
}

func scanPosts(rows pgx.Rows) ([]*dto.PostPreview, error) {
	posts := make([]*dto.PostPreview, 0)
	defer rows.Close()
	for rows.Next() {
		post := &dto.PostPreview{}
		err := rows.Scan(
			&post.ID,
			&post.Author.ID,
			&post.Title,
			&post.BodyPreview,
			&post.PublishedOrUpdatedAt,
			&post.LikesCount,
			&post.Author.Username,
			&post.Author.Nickname,
			&post.Author.AvatarUrl,
			&post.Locked,
			&post.LikedByUser,
		)
		if err != nil {
			return nil, err
		}
		if len([]rune(post.BodyPreview)) == 200 {
			post.BodyPreview += "..."
		}
		posts = append(posts, post)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return posts, nil
}

func (r *PostRepository) UpdatePost(ctx context.Context, userID *uuid.UUID, p models.Post, lastUpdatedAt time.Time) error {
	const query = `
		UPDATE post SET
			title = $3,
			body   = $4,
			status = $5,
			published_at = CASE WHEN $5 = 'published'
								THEN COALESCE(published_at, now())
								ELSE published_at END
		WHERE id = $1 AND author_id = $2 AND updated_at = $6
	`

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "update post"),
		slog.String("postID", p.ID.String()),
		slog.String("userID", userID.String()),
		slog.Time("lastUpdatedAt", lastUpdatedAt),
	)

	result, err := r.pool.Exec(ctx, query, p.ID, userID, p.Title, p.Body, p.Status, lastUpdatedAt)
	if err != nil {
		err = postgres.ParseError(err)
		if errors.Is(err, postgres.ErrCheckViolation) {
			return models.ErrPostValidation
		}
		return fmt.Errorf("update post error: %w", err)
	}

	if result.RowsAffected() == 0 {
		var (
			id uuid.UUID
			ts time.Time
		)
		err := r.pool.QueryRow(ctx, `SELECT author_id, updated_at FROM post WHERE id = $1`, p.ID).Scan(
			&id,
			&ts,
		)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return models.ErrPostNotFound
		case err != nil:
			return fmt.Errorf("update post error: get post: %w", err)
		case id != *userID:
			r.logger.WarnContext(ctx, "update post: forbidden (not author)",
				slog.String("op", "update post"),
				slog.String("postID", p.ID.String()),
				slog.String("userID", userID.String()),
				slog.String("actualAuthorID", id.String()),
			)
			return models.ErrPostNotFound
		case !ts.Equal(lastUpdatedAt):
			r.logger.WarnContext(ctx, "update post: conflict (optimistic lock)",
				slog.String("op", "update post"),
				slog.String("postID", p.ID.String()),
				slog.String("userID", userID.String()),
				slog.Time("expectedUpdatedAt", lastUpdatedAt),
				slog.Time("actualUpdatedAt", ts),
			)
			return models.ErrPostConflict
		default:
			return models.ErrPostConflict
		}
	}
	return nil
}

func (r *PostRepository) DeletePost(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) error {
	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "delete post"),
		slog.String("postID", postID.String()),
		slog.String("userID", userID.String()),
	)

	result, err := r.pool.Exec(ctx, `DELETE FROM post WHERE id = $1 AND author_id = $2`, postID, userID)
	if err != nil {
		return fmt.Errorf("delete post error: %w", err)
	}
	if result.RowsAffected() == 0 {
		return models.ErrPostNotFound
	}
	return nil
}

func (r *PostRepository) LikePost(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) (int64, error) {
	const getPostAuthor = `
		SELECT p.author_id
		FROM post p
		LEFT JOIN post_access pa ON pa.post_id = p.id
		WHERE p.id = $2
			AND p.status = 'published'
			AND NOT (pa.post_id IS NOT NULL AND p.author_id IS DISTINCT FROM $1)
		FOR KEY SHARE OF p`
	const insertLike = `
		INSERT INTO post_like (post_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (post_id, user_id) DO NOTHING
		RETURNING id`
	const insertNotification = `
		INSERT INTO notification (user_id, event_kind, post_like_id)
		VALUES ($1, 'post_liked', $2)`
	const getLikesCount = `SELECT likes_count FROM post WHERE id = $1`

	r.logger.DebugContext(ctx, "executing transaction",
		slog.String("op", "like post"),
		slog.String("postID", postID.String()),
		slog.String("userID", userID.String()),
	)

	var likesCount int64
	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		var authorID uuid.UUID
		err := tx.QueryRow(ctx, getPostAuthor, userID, postID).Scan(&authorID)
		if errors.Is(err, pgx.ErrNoRows) {
			return models.ErrPostNotFound
		}
		if err != nil {
			return fmt.Errorf("like post error: get post: %w", err)
		}

		var likeID uuid.UUID
		err = tx.QueryRow(ctx, insertLike, postID, userID).Scan(&likeID)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			// ничего не делаем при дупликате лайков
			r.logger.DebugContext(ctx, "like post: duplicate like (no-op)",
				slog.String("op", "like post"),
				slog.String("postID", postID.String()),
				slog.String("userID", userID.String()),
			)
		case err != nil:
			return fmt.Errorf("like post error: insert like: %w", err)
		case authorID != *userID:
			if _, err := tx.Exec(ctx, insertNotification, authorID, likeID); err != nil {
				return fmt.Errorf("like post error: insert notification: %w", err)
			}
		}

		if err := tx.QueryRow(ctx, getLikesCount, postID).Scan(&likesCount); err != nil {
			return fmt.Errorf("like post error: count likes: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return likesCount, nil
}

func (r *PostRepository) DeletePostLike(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) (int64, error) {
	const deletePostLike = `DELETE FROM post_like WHERE post_id = $1 AND user_id = $2`
	const getLikesCount = `SELECT likes_count FROM post WHERE id = $1`

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "unlike post"),
		slog.String("postID", postID.String()),
		slog.String("userID", userID.String()),
	)

	if result, err := r.pool.Exec(ctx, deletePostLike, postID, userID); err != nil {
		if result.RowsAffected() == 0 {
			return 0, models.ErrPostNotFound
		}
		return 0, fmt.Errorf("delete post like error: %w", err)
	}

	var likesCount int64
	if err := r.pool.QueryRow(ctx, getLikesCount, postID).Scan(&likesCount); err != nil {
		return 0, fmt.Errorf("delete post like error: count likes: %w", err)
	}
	return likesCount, nil
}

func uid(id *uuid.UUID) string {
	if id == nil {
		return "-"
	}
	return id.String()
}
