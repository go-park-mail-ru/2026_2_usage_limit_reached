package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) SearchAuthors(ctx context.Context, query string, categories []string, cursor *dto.AuthorSearchCursor, limit int) ([]*models.Author, error) {
	var (
		nickname *string
		id       *uuid.UUID
	)
	if cursor != nil {
		lower := strings.ToLower(cursor.Nickname)
		nickname, id = &lower, &cursor.ID
	}

	sqlQuery := `SELECT a.id, u.username, u.nickname, a.bio, u.avatar_key
		FROM author a
		JOIN "user" u ON u.id = a.id
		WHERE u.status = 'active'`
	args := make([]any, 0, 5)

	if query != "" {
		args = append(args, "%"+escapeLike(query)+"%")
		sqlQuery += fmt.Sprintf(" AND (u.username ILIKE $%d OR u.nickname ILIKE $%d)", len(args), len(args))
	}

	if len(categories) > 0 {
		args = append(args, categories)
		sqlQuery += fmt.Sprintf(" AND a.category = ANY($%d)", len(args))
	}

	textArg := len(args) + 1
	args = append(args, nickname, id)
	sqlQuery += fmt.Sprintf(
		" AND ($%d::text IS NULL OR (lower(u.nickname), u.id) > ($%d::text, $%d::uuid))",
		textArg, textArg, textArg+1,
	)

	limitArg := len(args) + 1
	args = append(args, limit)
	sqlQuery += " ORDER BY lower(u.nickname), u.id LIMIT $" + strconv.Itoa(limitArg)

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "search authors"),
		slog.Bool("hasQuery", query != ""),
		slog.Int("categories", len(categories)),
		slog.Bool("hasCursor", cursor != nil),
		slog.Int("limit", limit),
	)

	rows, err := r.pool.Query(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("search authors error: %w", err)
	}
	defer rows.Close()

	authors := make([]*models.Author, 0)
	for rows.Next() {
		var author models.Author
		if err := rows.Scan(&author.ID, &author.Username, &author.Nickname, &author.Bio, &author.AvatarKey); err != nil {
			return nil, fmt.Errorf("search authors scan error: %w", err)
		}

		authors = append(authors, &author)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("search authors error: %w", err)
	}

	return authors, nil
}

func (r *Repository) CreateAuthor(ctx context.Context, id uuid.UUID, bio string, category *string) (*models.Author, error) {
	const insert = `
		INSERT INTO author (id, bio, category)
		VALUES ($1, $2, $3)
		ON CONFLICT (id) DO NOTHING
	`
	const selectAuthor = `
		SELECT a.id, u.username, u.nickname, a.bio, a.category
		FROM author a
		JOIN "user" u ON u.id = a.id
		WHERE a.id = $1
	`

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "create author"),
		slog.String("author_id", id.String()),
	)

	if _, err := r.pool.Exec(ctx, insert, id, bio, category); err != nil {
		return nil, fmt.Errorf("create author error: %w", err)
	}

	var author models.Author
	err := r.pool.QueryRow(ctx, selectAuthor, id).Scan(
		&author.ID, &author.Username, &author.Nickname, &author.Bio, &author.Category,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrAuthorNotFound
		}

		return nil, fmt.Errorf("create author error: %w", err)
	}

	return &author, nil
}

func (r *Repository) GetAuthorByID(ctx context.Context, id uuid.UUID) (*models.Author, error) {
	const query = `
		SELECT a.id, u.username, u.nickname, a.bio, a.category
		FROM author a
		JOIN "user" u ON u.id = a.id
		WHERE a.id = $1
	`

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "get author"),
		slog.String("author_id", id.String()),
	)

	var author models.Author
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&author.ID, &author.Username, &author.Nickname, &author.Bio, &author.Category,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrAuthorNotFound
		}

		return nil, fmt.Errorf("get author error: %w", err)
	}

	return &author, nil
}

func (r *Repository) UpdateAuthorDetails(ctx context.Context, id uuid.UUID, bio string, category *string) (*models.Author, error) {
	const query = `
		WITH upd AS (
			UPDATE author SET bio = $2, category = $3
			WHERE id = $1
			RETURNING id, bio, category
		)
		SELECT upd.id, u.username, u.nickname, upd.bio, upd.category
		FROM upd
		JOIN "user" u ON u.id = upd.id
	`

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "update author details"),
		slog.String("author_id", id.String()),
	)

	var author models.Author
	err := r.pool.QueryRow(ctx, query, id, bio, category).Scan(
		&author.ID, &author.Username, &author.Nickname, &author.Bio, &author.Category,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrAuthorNotFound
		}

		return nil, fmt.Errorf("update author details error: %w", err)
	}

	return &author, nil
}

func escapeLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}
