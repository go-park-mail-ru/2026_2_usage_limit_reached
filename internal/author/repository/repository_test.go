package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/author/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/testutils"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v5"
	"github.com/stretchr/testify/require"
)

var (
	testAuthorID = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	testTime     = time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	errDB        = errors.New("connection lost")
)

func newTestRepository(t *testing.T) (*AuthorRepository, pgxmock.PgxPoolIface) {
	t.Helper()

	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	return &AuthorRepository{pool: pool, logger: testutils.SetupLogger()}, pool
}

func TestAuthorRepository_GetAuthorByID(t *testing.T) {
	category, provider, account := "Технологии", "yookassa", "acc-1"
	tests := []struct {
		name    string
		rows    *pgxmock.Rows
		err     error
		want    *models.Author
		wantErr error
	}{
		{
			name: "author with payout data",
			rows: pgxmock.NewRows([]string{"id", "bio", "category", "payout_provider", "payout_account_id", "created_at", "updated_at"}).
				AddRow(testAuthorID, "Пишу о технологиях", &category, &provider, &account, testTime, testTime),
			want: &models.Author{
				ID:              testAuthorID,
				AuthorDetails:   models.AuthorDetails{Bio: "Пишу о технологиях", Category: &category},
				PayoutProvider:  &provider,
				PayoutAccountID: &account,
				CreatedAt:       testTime,
				UpdatedAt:       testTime,
			},
		},
		{
			name: "author without optional fields",
			rows: pgxmock.NewRows([]string{"id", "bio", "category", "payout_provider", "payout_account_id", "created_at", "updated_at"}).
				AddRow(testAuthorID, "", nil, nil, nil, testTime, testTime),
			want: &models.Author{ID: testAuthorID, CreatedAt: testTime, UpdatedAt: testTime},
		},
		{name: "unknown author", err: pgx.ErrNoRows, wantErr: models.ErrAuthorNotFound},
		{name: "database failure", err: errDB, wantErr: errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, pool := newTestRepository(t)
			expect := pool.ExpectQuery(`FROM author`).WithArgs(testAuthorID)
			if tt.err != nil {
				expect.WillReturnError(tt.err)
			} else {
				expect.WillReturnRows(tt.rows)
			}

			got, err := repo.GetAuthorByID(context.Background(), testAuthorID)

			require.ErrorIs(t, err, tt.wantErr)
			require.Equal(t, tt.want, got)
			require.NoError(t, pool.ExpectationsWereMet())
		})
	}
}

func TestAuthorRepository_UpdateAuthor(t *testing.T) {
	category := "Книги"
	tests := []struct {
		name    string
		details models.AuthorDetails
		rows    *pgxmock.Rows
		err     error
		want    *models.Author
		wantErr error
	}{
		{
			name:    "updates bio and category",
			details: models.AuthorDetails{Bio: "Новое описание", Category: &category},
			rows:    pgxmock.NewRows([]string{"id", "bio", "category"}).AddRow(testAuthorID, "Новое описание", &category),
			want:    &models.Author{ID: testAuthorID, AuthorDetails: models.AuthorDetails{Bio: "Новое описание", Category: &category}},
		},
		{
			name:    "clears category",
			details: models.AuthorDetails{Bio: "Новое описание"},
			rows:    pgxmock.NewRows([]string{"id", "bio", "category"}).AddRow(testAuthorID, "Новое описание", nil),
			want:    &models.Author{ID: testAuthorID, AuthorDetails: models.AuthorDetails{Bio: "Новое описание"}},
		},
		{name: "unknown author", err: pgx.ErrNoRows, wantErr: models.ErrAuthorNotFound},
		{name: "database failure", err: errDB, wantErr: errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, pool := newTestRepository(t)
			expect := pool.ExpectQuery(`UPDATE author`).WithArgs(testAuthorID, tt.details.Bio, tt.details.Category)
			if tt.err != nil {
				expect.WillReturnError(tt.err)
			} else {
				expect.WillReturnRows(tt.rows)
			}

			got, err := repo.UpdateAuthor(context.Background(), testAuthorID, tt.details)

			require.ErrorIs(t, err, tt.wantErr)
			require.Equal(t, tt.want, got)
			require.NoError(t, pool.ExpectationsWereMet())
		})
	}
}
