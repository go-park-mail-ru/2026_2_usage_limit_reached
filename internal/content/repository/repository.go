package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Repository struct {
	pool   postgres.DB
	logger *slog.Logger
}

func NewRepository(db *postgres.DBClient, logger *slog.Logger) *Repository {
	return &Repository{pool: db.Pool, logger: logger}
}

func (r *Repository) CreatePost(ctx context.Context, userID *uuid.UUID, p models.Post) (uuid.UUID, error) {
	const query = `
		INSERT INTO post (author_id, title, body, search_text, status, published_at)
		VALUES ($1, $2, $3, $4, $5, CASE WHEN $5 = 'published' THEN now() END)
		RETURNING id
	`

	r.logger.DebugContext(ctx, "executing transaction",
		slog.String("op", "create post"),
		slog.String("userID", userID.String()),
	)

	var postID uuid.UUID
	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, query, userID, p.Title, p.Body, p.SearchText, p.Status).Scan(&postID)
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

		if len(p.Attachments) == 0 {
			return nil
		}

		if err := r.checkMediaOwnership(ctx, tx, *userID, p.Attachments); err != nil {
			return fmt.Errorf("create post error: %w", err)
		}

		if err := r.insertPostBlocks(ctx, tx, postID, p.Attachments); err != nil {
			return fmt.Errorf("create post error: %w", err)
		}

		return nil
	})
	if err != nil {
		return uuid.Nil, err
	}
	return postID, nil
}

func (r *Repository) GetPost(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) (*dto.PostInfo, error) {
	const query = `
		SELECT p.id, p.author_id, p.title,
			CASE WHEN l.locked THEN '' ELSE p.body END AS body,
			p.status, p.published_at, p.likes_count, p.created_at, p.updated_at, l.locked,
			EXISTS (SELECT 1 FROM post_like pl WHERE pl.post_id = p.id AND pl.user_id = $1) AS liked_by_me
		FROM post p
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

	post := &dto.PostInfo{Attachments: make([]*dto.Attachment, 0)}
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
		&post.Locked,
		&post.LikedByUser,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrPostNotFound
		}
		return nil, fmt.Errorf("get post error: %w", err)
	}

	if post.Locked {
		return post, nil
	}

	attachments, err := r.getPostAttachments(ctx, postID)
	if err != nil {
		return nil, fmt.Errorf("get post error: get attachments: %w", err)
	}
	post.Attachments = attachments

	return post, nil
}

func (r *Repository) GetMyNewPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) ([]*dto.PostPreview, error) {
	const query = `
		SELECT
			p.id, p.author_id, p.title, p.body, p.published_at, p.likes_count,
			FALSE,
			EXISTS (SELECT 1 FROM post_like pl WHERE pl.post_id = p.id AND pl.user_id = $1) AS liked_by_me
		FROM post p
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

	posts, err := r.getTimedPosts(ctx, query, userID, cursor, limit, nil)
	if err != nil {
		return nil, fmt.Errorf("get my new posts error: %w", err)
	}
	return posts, nil
}

func (r *Repository) GetMyOldPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) ([]*dto.PostPreview, error) {
	const query = `
		SELECT
			p.id, p.author_id, p.title, p.body, p.published_at, p.likes_count,
			FALSE,
			EXISTS (SELECT 1 FROM post_like pl WHERE pl.post_id = p.id AND pl.user_id = $1) AS liked_by_me
		FROM post p
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

	posts, err := r.getTimedPosts(ctx, query, userID, cursor, limit, nil)
	if err != nil {
		return nil, fmt.Errorf("get my old posts error: %w", err)
	}
	return posts, nil
}

func (r *Repository) GetMyPopularPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.PopularPostsCursor, limit int) ([]*dto.PostPreview, error) {
	const query = `
		SELECT
			p.id, p.author_id, p.title, p.body, p.published_at, p.likes_count,
			FALSE,
			EXISTS (SELECT 1 FROM post_like pl WHERE pl.post_id = p.id AND pl.user_id = $1) AS liked_by_me
		FROM post p
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

	posts, err := r.getPopularPosts(ctx, query, userID, cursor, limit, nil)
	if err != nil {
		return nil, fmt.Errorf("get my popular posts error: %w", err)
	}
	return posts, nil
}

func (r *Repository) GetMyNewDrafts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) ([]*dto.PostPreview, error) {
	const query = `
		SELECT
			p.id, p.author_id, p.title, p.body, p.updated_at, p.likes_count,
			FALSE, FALSE
		FROM post p
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

	posts, err := r.getTimedPosts(ctx, query, userID, cursor, limit, nil)
	if err != nil {
		return nil, fmt.Errorf("get my new drafts error: %w", err)
	}
	return posts, nil
}

func (r *Repository) GetMyOldDrafts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) ([]*dto.PostPreview, error) {
	const query = `
		SELECT
			p.id, p.author_id, p.title, p.body, p.updated_at, p.likes_count,
			FALSE, FALSE
		FROM post p
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

	posts, err := r.getTimedPosts(ctx, query, userID, cursor, limit, nil)
	if err != nil {
		return nil, fmt.Errorf("get my old drafts error: %w", err)
	}
	return posts, nil
}

func (r *Repository) GetUserNewPosts(ctx context.Context, userID *uuid.UUID, authorID uuid.UUID, cursor *dto.TimedPostsCursor, limit int) ([]*dto.PostPreview, error) {
	const query = `
		WITH page AS (
			SELECT p.id, p.author_id, p.title, p.body,
			       p.published_at, p.likes_count
			FROM post p
			WHERE p.status = 'published'
			  AND p.author_id = $2
			  AND ($3::timestamptz IS NULL OR (p.published_at, p.id) < ($3::timestamptz, $4::uuid))
			ORDER BY p.published_at DESC, p.id DESC
			LIMIT $5
		)
		SELECT p.id, p.author_id, p.title, CASE WHEN l.locked THEN '' ELSE p.body END AS body_preview,
		       p.published_at, p.likes_count, l.locked,
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
		slog.String("author_id", authorID.String()),
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

	rows, err := r.pool.Query(ctx, query, userID, authorID, ts, id, limit)
	if err != nil {
		return nil, fmt.Errorf("get user new posts error: %w", err)
	}

	posts, err := scanPosts(rows)
	if err != nil {
		return nil, fmt.Errorf("get user new posts error: %w", err)
	}

	return posts, nil
}

func (r *Repository) GetUserOldPosts(ctx context.Context, userID *uuid.UUID, authorID uuid.UUID, cursor *dto.TimedPostsCursor, limit int) ([]*dto.PostPreview, error) {
	const query = `
		WITH page AS (
			SELECT p.id, p.author_id, p.title, p.body,
			       p.published_at, p.likes_count
			FROM post p
			WHERE p.status = 'published'
			  AND p.author_id = $2
			  AND ($3::timestamptz IS NULL OR (p.published_at, p.id) > ($3::timestamptz, $4::uuid))
			ORDER BY p.published_at ASC, p.id ASC
			LIMIT $5
		)
		SELECT p.id, p.author_id, p.title, CASE WHEN l.locked THEN '' ELSE p.body END AS body_preview,
		       p.published_at, p.likes_count, l.locked,
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
		slog.String("author_id", authorID.String()),
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

	rows, err := r.pool.Query(ctx, query, userID, authorID, ts, id, limit)
	if err != nil {
		return nil, fmt.Errorf("get user old posts error: %w", err)
	}

	posts, err := scanPosts(rows)
	if err != nil {
		return nil, fmt.Errorf("get user old posts error: %w", err)
	}

	return posts, nil
}

func (r *Repository) GetUserPopularPosts(ctx context.Context, userID *uuid.UUID, authorID uuid.UUID, cursor *dto.PopularPostsCursor, limit int) ([]*dto.PostPreview, error) {
	const query = `
		WITH page AS (
			SELECT p.id, p.author_id, p.title, p.body,
			       p.published_at, p.likes_count
			FROM post p
			WHERE p.status = 'published'
			  AND p.author_id = $2
			  AND ($3::bigint IS NULL OR (p.likes_count, p.published_at, p.id) < ($3::bigint, $4::timestamptz, $5::uuid))
			ORDER BY p.likes_count DESC, p.published_at DESC, p.id DESC
			LIMIT $6
		)
		SELECT p.id, p.author_id, p.title, CASE WHEN l.locked THEN '' ELSE p.body END AS body_preview,
		       p.published_at, p.likes_count, l.locked,
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
		slog.String("author_id", authorID.String()),
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

	rows, err := r.pool.Query(ctx, query, userID, authorID, likes, ts, id, limit)
	if err != nil {
		return nil, fmt.Errorf("get user popular posts error: %w", err)
	}

	posts, err := scanPosts(rows)
	if err != nil {
		return nil, fmt.Errorf("get user popular posts error: %w", err)
	}

	return posts, nil
}

const feedLockedExpr = `(pa.post_id IS NOT NULL AND p.author_id IS DISTINCT FROM $1)`

func (r *Repository) GetNewPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int, filters dto.FeedFilters) ([]*dto.PostPreview, error) {
	query, filterArgs := buildFeedQuery(
		`AND ($2::timestamptz IS NULL OR (p.published_at, p.id) < ($2::timestamptz, $3::uuid))`,
		`p.published_at DESC, p.id DESC`,
		filters,
		4,
	)

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "get new posts"),
		slog.String("userID", uid(userID)),
		slog.Int("limit", limit),
		slog.Bool("hasCursor", cursor != nil),
	)

	posts, err := r.getTimedPosts(ctx, query, userID, cursor, limit, filterArgs)
	if err != nil {
		return nil, fmt.Errorf("get new posts error: %w", err)
	}
	return posts, nil
}

func (r *Repository) GetOldPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int, filters dto.FeedFilters) ([]*dto.PostPreview, error) {
	query, filterArgs := buildFeedQuery(
		`AND ($2::timestamptz IS NULL OR (p.published_at, p.id) > ($2::timestamptz, $3::uuid))`,
		`p.published_at, p.id`,
		filters,
		4,
	)

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "get old posts"),
		slog.String("userID", uid(userID)),
		slog.Int("limit", limit),
		slog.Bool("hasCursor", cursor != nil),
	)

	posts, err := r.getTimedPosts(ctx, query, userID, cursor, limit, filterArgs)
	if err != nil {
		return nil, fmt.Errorf("get old posts error: %w", err)
	}
	return posts, nil
}

func (r *Repository) GetPopularPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.PopularPostsCursor, limit int, filters dto.FeedFilters) ([]*dto.PostPreview, error) {
	query, filterArgs := buildFeedQuery(
		`AND ($2::bigint IS NULL OR (p.likes_count, p.published_at, p.id) < ($2::bigint, $3::timestamptz, $4::uuid))`,
		`p.likes_count DESC, p.published_at DESC, p.id DESC`,
		filters,
		5,
	)

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "get popular posts"),
		slog.String("userID", uid(userID)),
		slog.Int("limit", limit),
		slog.Bool("hasCursor", cursor != nil),
	)

	posts, err := r.getPopularPosts(ctx, query, userID, cursor, limit, filterArgs)
	if err != nil {
		return nil, fmt.Errorf("get popular posts error: %w", err)
	}
	return posts, nil
}

func buildFeedQuery(cursorCondition, orderBy string, filters dto.FeedFilters, nextArg int) (string, []any) {
	whereSQL, filterArgs, limitArg := buildFeedFilters(filters, nextArg)

	query := `
		WITH page AS (
			SELECT p.id, p.author_id, p.title, p.body, p.published_at, p.likes_count,
				` + feedLockedExpr + ` AS locked
			FROM post p
			LEFT JOIN post_access pa ON pa.post_id = p.id
			WHERE p.status = 'published'
				` + cursorCondition + `
				` + whereSQL + `
			ORDER BY ` + orderBy + `
			LIMIT $` + strconv.Itoa(limitArg) + `
		)
		SELECT
			p.id, p.author_id, p.title,
			CASE WHEN p.locked THEN '' ELSE p.body END AS body_preview,
			p.published_at, p.likes_count, p.locked,
			EXISTS (SELECT 1 FROM post_like pl WHERE pl.post_id = p.id AND pl.user_id = $1) AS liked_by_me
		FROM page p
		ORDER BY ` + orderBy + `
	`

	return query, filterArgs
}

func buildFeedFilters(filters dto.FeedFilters, nextArg int) (string, []any, int) {
	conditions := make([]string, 0, 3)
	args := make([]any, 0, 3)

	if filters.Query != "" {
		args = append(args, "%"+escapeLike(filters.Query)+"%")
		conditions = append(conditions, fmt.Sprintf(
			"(p.title ILIKE $%d OR (NOT %s AND p.search_text ILIKE $%d))",
			nextArg, feedLockedExpr, nextArg,
		))
		nextArg++
	}

	if filters.Tag != "" {
		args = append(args, filters.Tag)
		conditions = append(conditions, fmt.Sprintf(
			"EXISTS (SELECT 1 FROM author a WHERE a.id = p.author_id AND a.category = $%d)",
			nextArg,
		))
		nextArg++
	}

	if filters.AttachmentType != "" {
		args = append(args, filters.AttachmentType)
		conditions = append(conditions, fmt.Sprintf(
			"NOT %s AND EXISTS (SELECT 1 FROM post_block pb WHERE pb.post_id = p.id AND ($%d = 'any' OR pb.media_type = $%d))",
			feedLockedExpr, nextArg, nextArg,
		))
		nextArg++
	}

	if len(conditions) == 0 {
		return "", args, nextArg
	}

	return "AND " + strings.Join(conditions, " AND "), args, nextArg
}

func escapeLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}

func (r *Repository) getTimedPosts(ctx context.Context, query string, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int, filterArgs []any) ([]*dto.PostPreview, error) {
	var (
		ts *time.Time
		id *uuid.UUID
	)
	if cursor != nil {
		ts, id = &cursor.PublishedOrUpdatedAt, &cursor.PostID
	}

	args := make([]any, 0, 4+len(filterArgs))
	args = append(args, userID, ts, id)
	args = append(args, filterArgs...)
	args = append(args, limit)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	posts, err := scanPosts(rows)
	if err != nil {
		return nil, err
	}

	return posts, nil
}

func (r *Repository) getPopularPosts(ctx context.Context, query string, userID *uuid.UUID, cursor *dto.PopularPostsCursor, limit int, filterArgs []any) ([]*dto.PostPreview, error) {
	var (
		likes *int64
		ts    *time.Time
		id    *uuid.UUID
	)
	if cursor != nil {
		likes, ts, id = &cursor.Likes, &cursor.PublishedAt, &cursor.PostID
	}

	args := make([]any, 0, 5+len(filterArgs))
	args = append(args, userID, likes, ts, id)
	args = append(args, filterArgs...)
	args = append(args, limit)

	rows, err := r.pool.Query(ctx, query, args...)
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
			&post.Locked,
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

func (r *Repository) UpdatePost(ctx context.Context, userID *uuid.UUID, p models.Post, lastUpdatedAt time.Time) ([]string, error) {
	const query = `
		UPDATE post SET
			title = $3,
			body   = $4,
			search_text = $5,
			status = $6,
			published_at = CASE WHEN $6 = 'published'
								THEN COALESCE(published_at, now())
								ELSE published_at END
		WHERE id = $1 AND author_id = $2 AND updated_at = $7
	`
	const deleteBlocks = `DELETE FROM post_block WHERE post_id = $1`

	r.logger.DebugContext(ctx, "executing transaction",
		slog.String("op", "update post"),
		slog.String("postID", p.ID.String()),
		slog.String("userID", userID.String()),
		slog.Time("lastUpdatedAt", lastUpdatedAt),
	)

	detachedKeys := make([]string, 0)
	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, query, p.ID, userID, p.Title, p.Body, p.SearchText, p.Status, lastUpdatedAt)
		if err != nil {
			err = postgres.ParseError(err)
			if errors.Is(err, postgres.ErrCheckViolation) {
				return models.ErrPostValidation
			}
			return fmt.Errorf("update post error: %w", err)
		}

		if result.RowsAffected() == 0 {
			return r.updatePostConflict(ctx, tx, p.ID, *userID, lastUpdatedAt)
		}

		if len(p.Attachments) > 0 {
			if err := r.checkMediaOwnership(ctx, tx, *userID, p.Attachments); err != nil {
				return fmt.Errorf("update post error: %w", err)
			}
		}

		previous, err := r.selectPostMediaIDs(ctx, tx, p.ID)
		if err != nil {
			return fmt.Errorf("update post error: %w", err)
		}

		if _, err := tx.Exec(ctx, deleteBlocks, p.ID); err != nil {
			return fmt.Errorf("update post error: delete blocks: %w", err)
		}

		if len(p.Attachments) > 0 {
			if err := r.insertPostBlocks(ctx, tx, p.ID, p.Attachments); err != nil {
				return fmt.Errorf("update post error: %w", err)
			}
		}

		keys, err := r.deleteDetachedMedia(ctx, tx, *userID, previous)
		if err != nil {
			return fmt.Errorf("update post error: %w", err)
		}
		detachedKeys = keys

		return nil
	})
	if err != nil {
		return nil, err
	}
	return detachedKeys, nil
}

// updatePostConflict разбирает причину, по которой UPDATE post не затронул строку
func (r *Repository) updatePostConflict(ctx context.Context, tx pgx.Tx, postID, userID uuid.UUID, lastUpdatedAt time.Time) error {
	var (
		authorID uuid.UUID
		ts       time.Time
	)
	err := tx.QueryRow(ctx, `SELECT author_id, updated_at FROM post WHERE id = $1`, postID).Scan(
		&authorID,
		&ts,
	)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return models.ErrPostNotFound
	case err != nil:
		return fmt.Errorf("update post error: get post: %w", err)
	case authorID != userID:
		r.logger.WarnContext(ctx, "update post: forbidden (not author)",
			slog.String("op", "update post"),
			slog.String("postID", postID.String()),
			slog.String("userID", userID.String()),
			slog.String("actualAuthorID", authorID.String()),
		)
		return models.ErrPostNotFound
	case !ts.Equal(lastUpdatedAt):
		r.logger.WarnContext(ctx, "update post: conflict (optimistic lock)",
			slog.String("op", "update post"),
			slog.String("postID", postID.String()),
			slog.String("userID", userID.String()),
			slog.Time("expectedUpdatedAt", lastUpdatedAt),
			slog.Time("actualUpdatedAt", ts),
		)
		return models.ErrPostConflict
	default:
		return models.ErrPostConflict
	}
}

func (r *Repository) DeletePost(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) ([]string, error) {
	const query = `DELETE FROM post WHERE id = $1 AND author_id = $2`

	r.logger.DebugContext(ctx, "executing transaction",
		slog.String("op", "delete post"),
		slog.String("postID", postID.String()),
		slog.String("userID", userID.String()),
	)

	detachedKeys := make([]string, 0)
	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		// связи читаются до удаления поста, иначе каскад уже уберёт их
		mediaIDs, err := r.selectPostMediaIDs(ctx, tx, postID)
		if err != nil {
			return fmt.Errorf("delete post error: %w", err)
		}

		result, err := tx.Exec(ctx, query, postID, userID)
		if err != nil {
			return fmt.Errorf("delete post error: %w", err)
		}
		if result.RowsAffected() == 0 {
			return models.ErrPostNotFound
		}

		keys, err := r.deleteDetachedMedia(ctx, tx, *userID, mediaIDs)
		if err != nil {
			return fmt.Errorf("delete post error: %w", err)
		}
		detachedKeys = keys

		return nil
	})
	if err != nil {
		return nil, err
	}
	return detachedKeys, nil
}

func (r *Repository) LikePost(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) (int64, error) {
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

func (r *Repository) DeletePostLike(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) (int64, error) {
	const deletePostLike = `DELETE FROM post_like WHERE post_id = $1 AND user_id = $2`
	const getLikesCount = `SELECT likes_count FROM post WHERE id = $1`

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "unlike post"),
		slog.String("postID", postID.String()),
		slog.String("userID", userID.String()),
	)

	if _, err := r.pool.Exec(ctx, deletePostLike, postID, userID); err != nil {
		return 0, fmt.Errorf("delete post like error: %w", err)
	}

	var likesCount int64
	if err := r.pool.QueryRow(ctx, getLikesCount, postID).Scan(&likesCount); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, models.ErrPostNotFound
		}
		return 0, fmt.Errorf("delete post like error: count likes: %w", err)
	}
	return likesCount, nil
}

const (
	countOwnedMediaQuery = `
		SELECT count(DISTINCT id) FROM media WHERE id = ANY($1::uuid[]) AND author_id = $2
	`
	insertPostBlockQuery = `
		INSERT INTO post_block (post_id, media_id, position)
		SELECT $1, t.id, t.ord - 1
		FROM unnest($2::uuid[]) WITH ORDINALITY AS t(id, ord)
	`
	selectPostMediaIDsQuery  = `SELECT DISTINCT media_id FROM post_block WHERE post_id = $1`
	deleteDetachedMediaQuery = `
		DELETE FROM media m
		WHERE m.id = ANY($1::uuid[]) AND m.author_id = $2
			AND NOT EXISTS (SELECT 1 FROM post_block pb WHERE pb.media_id = m.id)
		RETURNING m.media_key
	`
	postAttachmentsQuery = `
		SELECT m.id, m.media_type, m.mime_type, m.filename, m.size_bytes
		FROM post_block pb
		JOIN media m ON m.id = pb.media_id
		WHERE pb.post_id = $1
		ORDER BY pb.position
	`
)

// uniqueIDs убирает повторы, сохраняя порядок первого вхождения
func uniqueIDs(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(ids))
	result := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

// checkMediaOwnership проверяет, что все медиа существуют и принадлежат автору
func (r *Repository) checkMediaOwnership(ctx context.Context, tx pgx.Tx, userID uuid.UUID, mediaIDs []uuid.UUID) error {
	owned := uniqueIDs(mediaIDs)

	var count int64
	if err := tx.QueryRow(ctx, countOwnedMediaQuery, owned, userID).Scan(&count); err != nil {
		return fmt.Errorf("check media ownership: %w", err)
	}
	if int(count) != len(owned) {
		return models.ErrPostValidation
	}

	return nil
}

// insertPostBlocks раскладывает маркеры по позициям; повторы media_id разрешены
func (r *Repository) insertPostBlocks(ctx context.Context, tx pgx.Tx, postID uuid.UUID, mediaIDs []uuid.UUID) error {
	if _, err := tx.Exec(ctx, insertPostBlockQuery, postID, mediaIDs); err != nil {
		err = postgres.ParseError(err)
		if errors.Is(err, postgres.ErrFKViolation) || errors.Is(err, postgres.ErrCheckViolation) {
			return models.ErrPostValidation
		}
		return fmt.Errorf("insert post blocks: %w", err)
	}

	return nil
}

// selectPostMediaIDs возвращает медиа, привязанные к посту
func (r *Repository) selectPostMediaIDs(ctx context.Context, tx pgx.Tx, postID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := tx.Query(ctx, selectPostMediaIDsQuery, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return ids, nil
}

// deleteDetachedMedia удаляет медиа, потерявшие все связи, и возвращает ключи для очистки в S3
func (r *Repository) deleteDetachedMedia(ctx context.Context, tx pgx.Tx, userID uuid.UUID, mediaIDs []uuid.UUID) ([]string, error) {
	if len(mediaIDs) == 0 {
		return []string{}, nil
	}

	rows, err := tx.Query(ctx, deleteDetachedMediaQuery, mediaIDs, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := make([]string, 0, len(mediaIDs))
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return keys, nil
}

// getPostAttachments возвращает вложения поста в порядке позиций
func (r *Repository) getPostAttachments(ctx context.Context, postID uuid.UUID) ([]*dto.Attachment, error) {
	rows, err := r.pool.Query(ctx, postAttachmentsQuery, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	attachments := make([]*dto.Attachment, 0)
	for rows.Next() {
		attachment := &dto.Attachment{}
		var mediaType string
		if err := rows.Scan(
			&attachment.ID,
			&mediaType,
			&attachment.MimeType,
			&attachment.Name,
			&attachment.SizeBytes,
		); err != nil {
			return nil, err
		}
		attachment.MediaType = dto.AttachmentType(mediaType)
		attachments = append(attachments, attachment)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return attachments, nil
}

func (r *Repository) CreateMedia(ctx context.Context, m models.Media) (models.Media, error) {
	const query = `
		INSERT INTO media (author_id, media_type, mime_type, media_key, filename, size_bytes)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at
	`

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "create media"),
		slog.String("authorID", m.AuthorID.String()),
	)

	err := r.pool.QueryRow(ctx, query, m.AuthorID, m.MediaType, m.MimeType, m.MediaKey, m.Filename, m.SizeBytes).
		Scan(&m.ID, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		err = postgres.ParseError(err)
		if errors.Is(err, postgres.ErrFKViolation) {
			return models.Media{}, models.ErrInvalidAuthorID
		}
		return models.Media{}, fmt.Errorf("create media error: %w", err)
	}

	return m, nil
}

func (r *Repository) CountPendingMedia(ctx context.Context, authorID uuid.UUID) (int64, error) {
	const query = `
		SELECT count(*)
		FROM media m
		WHERE m.author_id = $1
			AND NOT EXISTS (SELECT 1 FROM post_block pb WHERE pb.media_id = m.id)
	`

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "count pending media"),
		slog.String("authorID", authorID.String()),
	)

	var count int64
	if err := r.pool.QueryRow(ctx, query, authorID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count pending media error: %w", err)
	}

	return count, nil
}

func (r *Repository) GetMediaForAccess(ctx context.Context, userID *uuid.UUID, mediaID uuid.UUID) (models.Media, error) {
	const query = `
		SELECT m.id, m.author_id, m.media_type, m.mime_type, m.media_key, m.filename, m.size_bytes
		FROM media m
		WHERE m.id = $1
			AND (
				m.author_id = $2
				OR EXISTS (
					SELECT 1
					FROM post_block pb
					JOIN post p ON p.id = pb.post_id
					LEFT JOIN post_access pa ON pa.post_id = p.id
					WHERE pb.media_id = m.id
						AND p.status = 'published'
						AND NOT (pa.post_id IS NOT NULL AND p.author_id IS DISTINCT FROM $2)
				)
			)
	`

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "get media for access"),
		slog.String("mediaID", mediaID.String()),
		slog.String("userID", uid(userID)),
	)

	media := models.Media{}
	err := r.pool.QueryRow(ctx, query, mediaID, userID).Scan(
		&media.ID,
		&media.AuthorID,
		&media.MediaType,
		&media.MimeType,
		&media.MediaKey,
		&media.Filename,
		&media.SizeBytes,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Media{}, models.ErrMediaNotFound
		}
		return models.Media{}, fmt.Errorf("get media error: %w", err)
	}

	return media, nil
}

func uid(id *uuid.UUID) string {
	if id == nil {
		return "-"
	}
	return id.String()
}
