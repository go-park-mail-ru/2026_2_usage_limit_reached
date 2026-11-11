package profile

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/testutils"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v5"
	"github.com/stretchr/testify/require"
)

var (
	testUserID    = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	testCreatedAt = time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	profileCols   = []string{"id", "username", "nickname", "email", "avatar_key", "created_at", "status", "bio", "category"}
)

func newTestRepository(t *testing.T) (*ProfileRepository, pgxmock.PgxPoolIface) {
	t.Helper()

	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	return &ProfileRepository{pool: pool, logger: testutils.SetupLogger()}, pool
}

func TestProfileRepository_GetProfileByID(t *testing.T) {
	avatarKey := "avatars/ivan.png"
	category := "Технологии"
	dbErr := errors.New("connection lost")
	tests := []struct {
		name       string
		queryRows  *pgxmock.Rows
		queryErr   error
		want       *models.Profile
		wantErr    error
		wantAnyErr bool
	}{
		{
			name: "reader without avatar",
			queryRows: pgxmock.NewRows(profileCols).
				AddRow(testUserID, "ivan001", "Иван", "ivan@example.test", nil, testCreatedAt, "active", nil, nil),
			want: &models.Profile{UserID: testUserID, Username: "ivan001", Nickname: "Иван", Email: "ivan@example.test", CreatedAt: testCreatedAt, Status: "active"},
		},
		{
			name: "author with avatar",
			queryRows: pgxmock.NewRows(profileCols).
				AddRow(testUserID, "ivan001", "Иван", "ivan@example.test", &avatarKey, testCreatedAt, "active", ptr("Пишу о технологиях"), &category),
			want: &models.Profile{
				UserID: testUserID, Username: "ivan001", Nickname: "Иван", Email: "ivan@example.test",
				AvatarKey: &avatarKey, CreatedAt: testCreatedAt, Status: "active",
				Author: &models.ProfileAuthor{Bio: "Пишу о технологиях", Category: &category},
			},
		},
		{
			name: "author without category",
			queryRows: pgxmock.NewRows(profileCols).
				AddRow(testUserID, "ivan001", "Иван", "ivan@example.test", nil, testCreatedAt, "active", ptr("Пишу"), nil),
			want: &models.Profile{
				UserID: testUserID, Username: "ivan001", Nickname: "Иван", Email: "ivan@example.test", CreatedAt: testCreatedAt, Status: "active",
				Author: &models.ProfileAuthor{Bio: "Пишу"},
			},
		},
		{name: "unknown user", queryRows: pgxmock.NewRows(profileCols), wantErr: models.ErrProfileNotFound},
		{name: "database failure", queryErr: dbErr, wantErr: dbErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, pool := newTestRepository(t)
			expect := pool.ExpectQuery(`WHERE u.id = \$1`).WithArgs(testUserID)
			if tt.queryErr != nil {
				expect.WillReturnError(tt.queryErr)
			} else {
				expect.WillReturnRows(tt.queryRows)
			}

			got, err := repo.GetProfileByID(context.Background(), testUserID)

			require.ErrorIs(t, err, tt.wantErr)
			require.Equal(t, tt.want, got)
			require.NoError(t, pool.ExpectationsWereMet())
		})
	}
}

func TestProfileRepository_GetProfileByUsername(t *testing.T) {
	tests := []struct {
		name      string
		queryRows *pgxmock.Rows
		want      *models.Profile
		wantErr   error
	}{
		{
			name: "found",
			queryRows: pgxmock.NewRows(profileCols).
				AddRow(testUserID, "ivan001", "Иван", "ivan@example.test", nil, testCreatedAt, "active", nil, nil),
			want: &models.Profile{UserID: testUserID, Username: "ivan001", Nickname: "Иван", Email: "ivan@example.test", CreatedAt: testCreatedAt, Status: "active"},
		},
		{name: "unknown username", queryRows: pgxmock.NewRows(profileCols), wantErr: models.ErrProfileNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, pool := newTestRepository(t)
			pool.ExpectQuery(`WHERE u.username = \$1`).WithArgs("ivan001").WillReturnRows(tt.queryRows)

			got, err := repo.GetProfileByUsername(context.Background(), "ivan001")

			require.ErrorIs(t, err, tt.wantErr)
			require.Equal(t, tt.want, got)
			require.NoError(t, pool.ExpectationsWereMet())
		})
	}
}

func TestProfileRepository_ScanFailure(t *testing.T) {
	repo, pool := newTestRepository(t)
	pool.ExpectQuery(`WHERE u.id = \$1`).WithArgs(testUserID).
		WillReturnRows(pgxmock.NewRows(profileCols).
			AddRow(testUserID, "ivan001", "Иван", "ivan@example.test", nil, "not a time", "active", nil, nil))

	got, err := repo.GetProfileByID(context.Background(), testUserID)

	require.Nil(t, got)
	require.Error(t, err)
	require.NotErrorIs(t, err, models.ErrProfileNotFound)
	require.NotErrorIs(t, err, pgx.ErrNoRows)
}

func ptr(s string) *string {
	return &s
}
