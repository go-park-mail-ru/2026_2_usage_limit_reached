package user

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v5"
	"github.com/stretchr/testify/require"
)

var (
	testUserID = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	testTime   = time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	userCols   = []string{"id", "username", "nickname", "email", "password_hash", "avatar_key", "status", "created_at", "updated_at"}
	errDB      = errors.New("connection lost")
)

func newTestRepository(t *testing.T) (*UserRepository, pgxmock.PgxPoolIface) {
	t.Helper()

	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	return &UserRepository{pool: pool}, pool
}

func testUser() *models.User {
	return &models.User{
		ID: testUserID, Username: "ivan001", Nickname: "Иван", Email: "ivan@example.test",
		PasswordHash: "hash", Status: models.StatusActive, CreatedAt: testTime, UpdatedAt: testTime,
	}
}

func userRow() *pgxmock.Rows {
	return pgxmock.NewRows(userCols).
		AddRow(testUserID, "ivan001", "Иван", "ivan@example.test", "hash", nil, models.StatusActive, testTime, testTime)
}

func TestUserRepository_CreateUser(t *testing.T) {
	newUser := models.User{Username: "ivan001", Nickname: "Иван", Email: "ivan@example.test", PasswordHash: "hash", Status: models.StatusActive}
	tests := []struct {
		name    string
		rows    *pgxmock.Rows
		err     error
		want    *models.User
		wantErr error
	}{
		{
			name: "created",
			rows: pgxmock.NewRows([]string{"id", "created_at", "updated_at"}).AddRow(testUserID, testTime, testTime),
			want: testUser(),
		},
		{name: "username or email taken", err: &pgconn.PgError{Code: "23505"}, wantErr: models.ErrUserExists},
		{name: "database failure", err: errDB, wantErr: errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, pool := newTestRepository(t)
			expect := pool.ExpectQuery(`INSERT INTO "user"`).WithArgs("ivan001", "Иван", "ivan@example.test", "hash", models.StatusActive)
			if tt.err != nil {
				expect.WillReturnError(tt.err)
			} else {
				expect.WillReturnRows(tt.rows)
			}

			got, err := repo.CreateUser(context.Background(), newUser)

			require.ErrorIs(t, err, tt.wantErr)
			require.Equal(t, tt.want, got)
			require.NoError(t, pool.ExpectationsWereMet())
		})
	}
}

func TestUserRepository_CreateUser_OtherConstraintIsNotConflict(t *testing.T) {
	repo, pool := newTestRepository(t)
	pool.ExpectQuery(`INSERT INTO "user"`).WithArgs("", "", "", "", "").WillReturnError(&pgconn.PgError{Code: "23514"})

	got, err := repo.CreateUser(context.Background(), models.User{})

	require.Nil(t, got)
	require.ErrorIs(t, err, postgres.ErrCheckViolation)
	require.NotErrorIs(t, err, models.ErrUserExists)
}

func TestUserRepository_UpdateNickname(t *testing.T) {
	tests := []struct {
		name    string
		rows    *pgxmock.Rows
		err     error
		want    string
		wantErr error
	}{
		{name: "updated", rows: pgxmock.NewRows([]string{"nickname"}).AddRow("Новый ник"), want: "Новый ник"},
		{name: "unknown user", err: pgx.ErrNoRows, wantErr: models.ErrUserNotFound},
		{name: "database failure", err: errDB, wantErr: errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, pool := newTestRepository(t)
			expect := pool.ExpectQuery(`SET nickname = \$2`).WithArgs(testUserID, "Новый ник")
			if tt.err != nil {
				expect.WillReturnError(tt.err)
			} else {
				expect.WillReturnRows(tt.rows)
			}

			got, err := repo.UpdateNickname(context.Background(), testUserID, "Новый ник")

			require.ErrorIs(t, err, tt.wantErr)
			require.Equal(t, tt.want, got)
			require.NoError(t, pool.ExpectationsWereMet())
		})
	}
}

func TestUserRepository_UpdateAvatarKey(t *testing.T) {
	newKey, oldKey := "avatars/new.png", "avatars/old.png"
	tests := []struct {
		name    string
		key     *string
		rows    *pgxmock.Rows
		err     error
		want    *string
		wantErr error
	}{
		{name: "replaces old key", key: &newKey, rows: pgxmock.NewRows([]string{"avatar_key"}).AddRow(&oldKey), want: &oldKey},
		{name: "sets first key", key: &newKey, rows: pgxmock.NewRows([]string{"avatar_key"}).AddRow(nil)},
		{name: "clears key", rows: pgxmock.NewRows([]string{"avatar_key"}).AddRow(&oldKey), want: &oldKey},
		{name: "unknown user", key: &newKey, err: pgx.ErrNoRows, wantErr: models.ErrUserNotFound},
		{name: "database failure", key: &newKey, err: errDB, wantErr: errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, pool := newTestRepository(t)
			expect := pool.ExpectQuery(`SET avatar_key = \$2`).WithArgs(testUserID, tt.key)
			if tt.err != nil {
				expect.WillReturnError(tt.err)
			} else {
				expect.WillReturnRows(tt.rows)
			}

			got, err := repo.UpdateAvatarKey(context.Background(), testUserID, tt.key)

			require.ErrorIs(t, err, tt.wantErr)
			require.Equal(t, tt.want, got)
			require.NoError(t, pool.ExpectationsWereMet())
		})
	}
}

func TestUserRepository_GetUser(t *testing.T) {
	lookups := []struct {
		name  string
		where string
		arg   any
		get   func(*UserRepository) (*models.User, error)
	}{
		{name: "by id", where: `WHERE id = \$1`, arg: testUserID, get: func(r *UserRepository) (*models.User, error) {
			return r.GetUserByID(context.Background(), testUserID)
		}},
		{name: "by username", where: `WHERE username = \$1`, arg: "ivan001", get: func(r *UserRepository) (*models.User, error) {
			return r.GetUserByUsername(context.Background(), "ivan001")
		}},
		{name: "by email", where: `WHERE email = \$1`, arg: "ivan@example.test", get: func(r *UserRepository) (*models.User, error) {
			return r.GetUserByEmail(context.Background(), "ivan@example.test")
		}},
	}
	results := []struct {
		name    string
		err     error
		want    *models.User
		wantErr error
	}{
		{name: "found", want: testUser()},
		{name: "not found", err: pgx.ErrNoRows, wantErr: models.ErrUserNotFound},
		{name: "database failure", err: errDB, wantErr: errDB},
	}
	for _, lookup := range lookups {
		for _, res := range results {
			t.Run(lookup.name+"/"+res.name, func(t *testing.T) {
				repo, pool := newTestRepository(t)
				expect := pool.ExpectQuery(lookup.where).WithArgs(lookup.arg)
				if res.err != nil {
					expect.WillReturnError(res.err)
				} else {
					expect.WillReturnRows(userRow())
				}

				got, err := lookup.get(repo)

				require.ErrorIs(t, err, res.wantErr)
				require.Equal(t, res.want, got)
				require.NoError(t, pool.ExpectationsWereMet())
			})
		}
	}
}

func TestUserRepository_GetUser_InvalidField(t *testing.T) {
	repo, pool := newTestRepository(t)

	got, err := repo.getUser(context.Background(), "password_hash", "hash")

	require.Nil(t, got)
	require.Error(t, err)
	require.NoError(t, pool.ExpectationsWereMet(), "запрос к БД не должен выполняться")
}
