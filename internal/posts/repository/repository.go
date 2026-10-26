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
	"github.com/jackc/pgx/v5/pgconn"
)

type PgxCliPool interface {
	Query(ctx context.Context, query string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error)
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

type PostRepository struct {
	pool PgxCliPool
}

func NewPostRepository(pool PgxCliPool) *PostRepository {
	return &PostRepository{pool: pool}
}

const (
	postColumns = `
		p.id, p.author_id, p.title,
		CASE WHEN l.locked THEN '' ELSE p.body_preview END AS body_preview,
		p.published_at, p.created_at, p.updated_at,
		p.username, p.nickname, p.avatar_key, l.locked`
	statusPublished  = `'published'`
	statusUserActive = `'active'`
	postLikeEvent    = `'post_liked'`
	countLikes       = `SELECT count(*) FROM post_like WHERE post_id = $1`
)

func (r *PostRepository) CreatePost(ctx context.Context, userID uuid.UUID, p models.Post) error {
	const query = `
		INSERT INTO post (author_id, title, body, status, published_at)
		VALUES ($1, $2, $3, $4, CASE WHEN $4 = ` + statusPublished + ` THEN now() END)
	`

	_, err := r.pool.Exec(ctx, query, userID, p.Title, p.Body, p.Status)
	if err != nil {
		err = postgres.ParseError(err)
		switch {
		case errors.Is(err, postgres.ErrFKViolation):
			return models.ErrInvalidAuthorID
		case errors.Is(err, postgres.ErrCheckViolation):
			return models.ErrPostValidation
		}
		return fmt.Errorf("create post error: %w", err)
	}
	return nil
}

func (r *PostRepository) GetPost(ctx context.Context, userID, postID uuid.UUID) (*dto.PostInfo, error) {
	const query = `
		SELECT p.id, p.author_id, p.title,
			CASE WHEN l.locked THEN '' ELSE p.body END AS body,
			p.status, p.published_at, p.created_at, p.updated_at, u.username, u.nickname, u.avatar_key, l.locked,
			(SELECT count(*) FROM post_like pl WHERE pl.post_id = p.id) AS likes_count,
			EXISTS (SELECT 1 FROM post_like pl WHERE pl.post_id = p.id AND pl.user_id = $1) AS liked_by_me
		FROM post p
		JOIN "user" u ON u.id = p.author_id AND u.status = ` + statusUserActive + `
		LEFT JOIN post_access pa ON pa.post_id = p.id
		CROSS JOIN LATERAL (
			SELECT (pa.post_id IS NOT NULL AND p.author_id IS DISTINCT FROM $1) AS locked
		) l
		WHERE p.id = $2
		AND (p.status = ` + statusPublished + ` OR p.author_id = $1)
	`

	post := &dto.PostInfo{}

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
		return nil, fmt.Errorf("get post error: %w", err)
	}
	return post, nil
}

func (r *PostRepository) GetNewPosts(ctx context.Context, userID uuid.UUID, cursor *dto.NewPostsCursor, limit int) ([]*dto.PostInfo, error) {
	const query = `
		WITH page AS (
			SELECT p.id, p.author_id, p.title, left(p.body, 200) AS body_preview,
			p.published_at, p.created_at, p.updated_at, u.username, u.nickname, u.avatar_key
			FROM post p
			JOIN "user" u ON u.id = p.author_id
			WHERE p.status = ` + statusPublished + `
				AND u.status = ` + statusUserActive + `
				AND ($2 IS NULL OR (p.published_at, p.id) < ($2, $3))
			ORDER BY p.published_at DESC, p.id DESC
			LIMIT $4
		)
		SELECT` + postColumns + `,
				(SELECT count(*) FROM post_like pl WHERE pl.post_id = p.id) AS likes_count,
				EXISTS (SELECT 1 FROM post_like pl WHERE pl.post_id = p.id AND pl.user_id = $1) AS liked_by_me
		FROM page p
		LEFT JOIN post_access pa ON pa.post_id = p.id
		CROSS JOIN LATERAL (
			SELECT (pa.post_id IS NOT NULL AND p.author_id IS DISTINCT FROM $1) AS locked
		) l
		ORDER BY p.published_at DESC, p.id DESC
	`

	var (
		ts *time.Time
		id *uuid.UUID
	)
	if cursor != nil {
		ts, id = &cursor.PublishedAt, &cursor.PostID
	}

	rows, err := r.pool.Query(ctx, query, userID, ts, id, limit)
	if err != nil {
		return nil, fmt.Errorf("get new posts error: %w", err)
	}
	posts, err := scanPosts(rows)
	if err != nil {
		return nil, fmt.Errorf("get new posts error: %w", err)
	}
	return posts, nil
}

func (r *PostRepository) GetPopularPosts(ctx context.Context, userID uuid.UUID, cursor *dto.PopularPostsCursor, limit int) ([]*dto.PostInfo, error) {
	const query = `
		WITH posts_with_likes AS (
			SELECT p.id, p.author_id, p.title, left(p.body, 200) AS body_preview, p.published_at, p.created_at, p.updated_at, u.username, u.nickname, u.avatar_key,
			(SELECT count(*) FROM post_like pl WHERE pl.post_id = p.id) AS likes_count
			FROM post p
			JOIN "user" u ON u.id = p.author_id
			WHERE p.status = ` + statusPublished + ` AND u.status = ` + statusUserActive + `
		), page AS (
			SELECT * FROM posts_with_likes pwl
			WHERE $2 IS NULL OR (pwl.likes_count, pwl.published_at, pwl.id) < ($2, $3, $4)
			ORDER BY pwl.likes_count DESC, pwl.published_at DESC, pwl.id DESC
			LIMIT $5
		)
		SELECT` + postColumns + `, p.likes_count,
				EXISTS (SELECT 1 FROM post_like pl WHERE pl.post_id = p.id AND pl.user_id = $1) AS liked_by_me
		FROM page p
		LEFT JOIN post_access pa ON pa.post_id = p.id
		CROSS JOIN LATERAL (
			SELECT (pa.post_id IS NOT NULL AND p.author_id IS DISTINCT FROM $1) AS locked
		) l
		ORDER BY p.likes_count DESC, p.published_at DESC, p.id DESC
	`
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
		return nil, fmt.Errorf("get popular posts error: %w", err)
	}
	posts, err := scanPosts(rows)
	if err != nil {
		return nil, fmt.Errorf("get popular posts error: %w", err)
	}
	return posts, nil
}

func scanPosts(rows pgx.Rows) ([]*dto.PostInfo, error) {
	posts := make([]*dto.PostInfo, 0)
	defer rows.Close()
	for rows.Next() {
		post := &dto.PostInfo{}
		err := rows.Scan(
			&post.ID,
			&post.AuthorID,
			&post.Title,
			&post.Body,
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
			return nil, err
		}
		posts = append(posts, post)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return posts, nil
}

func (r *PostRepository) UpdatePost(ctx context.Context, userID uuid.UUID, p models.Post, lastUpdatedAt time.Time) error {
	const query = `
		UPDATE post SET
			title = $3,
			body   = $4,
			status = $5,
			published_at = CASE WHEN $5 = ` + statusPublished + `
								THEN COALESCE(published_at, now())
								ELSE published_at END
		WHERE id = $1 AND author_id = $2 AND updated_at = $6
	`

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
		case id != userID:
			return models.ErrForbidden
		case !ts.Equal(lastUpdatedAt):
			return models.ErrPostConflict
		}
	}
	return nil
}

func (r *PostRepository) DeletePost(ctx context.Context, userID, postID uuid.UUID) error {
	result, err := r.pool.Exec(ctx, `DELETE FROM post WHERE id = $1 AND author_id = $2`, postID, userID)
	if err != nil {
		return fmt.Errorf("delete post error: %w", err)
	}
	if result.RowsAffected() == 0 {
		return models.ErrPostNotFound
	}
	return nil
}

func (r *PostRepository) LikePost(ctx context.Context, userID, postID uuid.UUID) (int64, error) {
	const getPostAuthor = `
		SELECT p.author_id
		FROM post p
		LEFT JOIN post_access pa ON pa.post_id = p.id
		WHERE p.id = $2
			AND p.status = ` + statusPublished + `
			AND NOT (pa.post_id IS NOT NULL AND p.author_id IS DISTINCT FROM $1)
		FOR KEY SHARE OF p`
	const insertLike = `
		INSERT INTO post_like (post_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (post_id, user_id) DO NOTHING
		RETURNING id`
	const insertNotification = `
		INSERT INTO notification (user_id, event_kind, post_like_id)
		VALUES ($1, ` + postLikeEvent + `, $2)`

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
			// do nothing on duplicate likes
		case err != nil:
			return fmt.Errorf("like post error: insert like: %w", err)
		case authorID != userID:
			if _, err := tx.Exec(ctx, insertNotification, authorID, likeID); err != nil {
				return fmt.Errorf("like post error: insert notification: %w", err)
			}
		}

		if err := tx.QueryRow(ctx, countLikes, postID).Scan(&likesCount); err != nil {
			return fmt.Errorf("like post error: count likes: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return likesCount, nil
}

func (r *PostRepository) DeletePostLike(ctx context.Context, userID, postID uuid.UUID) (int64, error) {
	if _, err := r.pool.Exec(ctx,
		`DELETE FROM post_like WHERE post_id = $1 AND user_id = $2`, postID, userID); err != nil {
		return 0, fmt.Errorf("delete post like error: %w", err)
	}

	var likesCount int64
	if err := r.pool.QueryRow(ctx, countLikes, postID).Scan(&likesCount); err != nil {
		return 0, fmt.Errorf("delete post like error: count likes: %w", err)
	}
	return likesCount, nil
}
