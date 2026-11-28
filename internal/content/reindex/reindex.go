// reindex пересчитывает post.search_text тем же санитайзером, что и приложение
package reindex

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/sanitizer"
	"github.com/google/uuid"
)

const DefaultBatchSize = 500

type Runner struct {
	db        postgres.DB
	sanitizer *sanitizer.Sanitizer
	batchSize int
	logger    *slog.Logger
}

func NewRunner(db postgres.DB, s *sanitizer.Sanitizer, batchSize int, logger *slog.Logger) *Runner {
	if batchSize <= 0 {
		batchSize = DefaultBatchSize
	}

	return &Runner{db: db, sanitizer: s, batchSize: batchSize, logger: logger}
}

func (r *Runner) Run(ctx context.Context) (int64, error) {
	var (
		after uuid.UUID
		total int64
	)

	for {
		items, err := r.fetch(ctx, after)
		if err != nil {
			return total, err
		}
		if len(items) == 0 {
			return total, nil
		}

		ids := make([]uuid.UUID, len(items))
		texts := make([]string, len(items))

		for i, item := range items {
			text, err := r.sanitizer.PlainText(item.body)
			if err != nil {
				return total, fmt.Errorf("reindex post %s: %w", item.id, err)
			}

			ids[i] = item.id
			texts[i] = text
		}

		if err := r.update(ctx, ids, texts); err != nil {
			return total, err
		}

		total += int64(len(items))
		after = items[len(items)-1].id

		if r.logger != nil {
			r.logger.InfoContext(ctx, "reindex batch done",
				slog.Int64("posts", total),
				slog.String("last_id", after.String()),
			)
		}
	}
}

type postRow struct {
	id   uuid.UUID
	body string
}

func (r *Runner) fetch(ctx context.Context, after uuid.UUID) ([]postRow, error) {
	const query = `SELECT id, body FROM post WHERE id > $1 ORDER BY id LIMIT $2`

	rows, err := r.db.Query(ctx, query, after, r.batchSize)
	if err != nil {
		return nil, fmt.Errorf("select posts: %w", err)
	}
	defer rows.Close()

	items := make([]postRow, 0, r.batchSize)
	for rows.Next() {
		var item postRow
		if err := rows.Scan(&item.id, &item.body); err != nil {
			return nil, fmt.Errorf("scan post: %w", err)
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("select posts: %w", err)
	}

	return items, nil
}

func (r *Runner) update(ctx context.Context, ids []uuid.UUID, texts []string) error {
	const query = `
		UPDATE post p SET search_text = v.text
		FROM unnest($1::uuid[], $2::text[]) AS v(id, text)
		WHERE p.id = v.id
	`

	if _, err := r.db.Exec(ctx, query, ids, texts); err != nil {
		return fmt.Errorf("update search_text: %w", err)
	}

	return nil
}
