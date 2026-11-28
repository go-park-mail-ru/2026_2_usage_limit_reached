package reindex_test

import (
	"context"
	"errors"
	"testing"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/reindex"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/testutils"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/sanitizer"
	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v5"
	"github.com/stretchr/testify/require"
)

var (
	selectQuery = testutils.SQLInOrder(
		"SELECT id, body FROM post",
		"WHERE id > $1 ORDER BY id LIMIT $2",
	)
	updateQuery = testutils.SQLInOrder(
		"UPDATE post p SET search_text = v.text",
		"FROM unnest($1::uuid[], $2::text[]) AS v(id, text)",
	)
	columns = []string{"id", "body"}
)

func newRunner(t *testing.T, pool pgxmock.PgxPoolIface, batchSize int) *reindex.Runner {
	t.Helper()

	s, err := sanitizer.NewSanitizer(sanitizer.DefaultConfig())
	require.NoError(t, err)

	return reindex.NewRunner(pool, s, batchSize, testutils.SetupLogger())
}

func TestRunner_Run(t *testing.T) {
	firstID := uuid.MustParse("00000000-0000-4000-8000-000000000101")
	secondID := uuid.MustParse("00000000-0000-4000-8000-000000000102")

	t.Run("recomputes text in batches and stops on empty page", func(t *testing.T) {
		pool := testutils.SetupPgxMock(t)
		runner := newRunner(t, pool, 2)

		pool.ExpectQuery(selectQuery).
			WithArgs(uuid.Nil, 2).
			WillReturnRows(pgxmock.NewRows(columns).
				AddRow(firstID, `<p>Привет</p><script>alert(1)</script>`).
				AddRow(secondID, `<p>R&amp;D</p>`))

		pool.ExpectExec(updateQuery).
			WithArgs([]uuid.UUID{firstID, secondID}, []string{"Привет", "R&D"}).
			WillReturnResult(pgxmock.NewResult("UPDATE", 2))

		pool.ExpectQuery(selectQuery).
			WithArgs(secondID, 2).
			WillReturnRows(pgxmock.NewRows(columns))

		total, err := runner.Run(context.Background())

		require.NoError(t, err)
		require.Equal(t, int64(2), total)
	})

	t.Run("select error is returned", func(t *testing.T) {
		pool := testutils.SetupPgxMock(t)
		runner := newRunner(t, pool, 2)

		pool.ExpectQuery(selectQuery).
			WithArgs(uuid.Nil, 2).
			WillReturnError(errors.New("db down"))

		total, err := runner.Run(context.Background())

		require.ErrorContains(t, err, "select posts")
		require.Zero(t, total)
	})

	t.Run("update error is returned", func(t *testing.T) {
		pool := testutils.SetupPgxMock(t)
		runner := newRunner(t, pool, 2)

		pool.ExpectQuery(selectQuery).
			WithArgs(uuid.Nil, 2).
			WillReturnRows(pgxmock.NewRows(columns).AddRow(firstID, `<p>x</p>`))

		pool.ExpectExec(updateQuery).
			WithArgs([]uuid.UUID{firstID}, []string{"x"}).
			WillReturnError(errors.New("db down"))

		total, err := runner.Run(context.Background())

		require.ErrorContains(t, err, "update search_text")
		require.Zero(t, total)
	})
}
