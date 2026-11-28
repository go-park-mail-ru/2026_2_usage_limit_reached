package repository

import (
	"context"
	"testing"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/testutils"
	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v5"
	"github.com/stretchr/testify/require"
)

var (
	authorCols   = []string{"id", "username", "nickname", "bio", "avatar_key"}
	authorMeCols = []string{"id", "username", "nickname", "bio", "category"}
	noNickname   = (*string)(nil)
	noCategory   = (*string)(nil)
	noID         = (*uuid.UUID)(nil)
)

func authorRows(items []*models.Author) *pgxmock.Rows {
	rows := pgxmock.NewRows(authorCols)
	for _, item := range items {
		rows.AddRow(item.ID, item.Username, item.Nickname, item.Bio, item.AvatarKey)
	}
	return rows
}

func TestRepository_SearchAuthors(t *testing.T) {
	firstID := uuid.New()
	avatarKey := "avatars/ivan.png"

	t.Run("by query, first page", func(t *testing.T) {
		repo, pool := newTestRepository(t)
		author := &models.Author{ID: firstID, Username: "ivan", Nickname: "Иван", Bio: "Био", AvatarKey: &avatarKey}

		pool.ExpectQuery(testutils.SQLInOrder(
			"SELECT a.id, u.username, u.nickname, a.bio, u.avatar_key",
			`JOIN "user" u ON u.id = a.id`,
			"WHERE u.status = 'active'",
			"u.username ILIKE $1 OR u.nickname ILIKE $1",
			"($2::text IS NULL OR (lower(u.nickname), u.id) > ($2::text, $3::uuid))",
			"ORDER BY lower(u.nickname), u.id",
			"LIMIT $4",
		)).
			WithArgs("%иван%", noNickname, noID, 21).
			WillReturnRows(authorRows([]*models.Author{author}))

		got, err := repo.SearchAuthors(context.Background(), "иван", nil, nil, 21)

		require.NoError(t, err)
		require.Equal(t, []*models.Author{author}, got)
		require.NoError(t, pool.ExpectationsWereMet())
	})

	t.Run("by categories, next page", func(t *testing.T) {
		repo, pool := newTestRepository(t)
		categories := []string{"Творчество", "Образование"}
		cursor := &dto.AuthorSearchCursor{Nickname: "иван", ID: firstID}
		author := &models.Author{ID: firstID, Username: "ivan", Nickname: "Иван 2", Bio: "Био"}

		pool.ExpectQuery(testutils.SQLInOrder(
			"SELECT a.id, u.username, u.nickname, a.bio, u.avatar_key",
			`JOIN "user" u ON u.id = a.id`,
			"WHERE u.status = 'active'",
			"a.category = ANY($1)",
			"($2::text IS NULL OR (lower(u.nickname), u.id) > ($2::text, $3::uuid))",
			"ORDER BY lower(u.nickname), u.id",
			"LIMIT $4",
		)).
			WithArgs(categories, &cursor.Nickname, &cursor.ID, 21).
			WillReturnRows(authorRows([]*models.Author{author}))

		got, err := repo.SearchAuthors(context.Background(), "", categories, cursor, 21)

		require.NoError(t, err)
		require.Equal(t, []*models.Author{author}, got)
		require.NoError(t, pool.ExpectationsWereMet())
	})

	t.Run("by query and categories", func(t *testing.T) {
		repo, pool := newTestRepository(t)
		categories := []string{"Игры"}

		pool.ExpectQuery(testutils.SQLInOrder(
			"SELECT a.id, u.username, u.nickname, a.bio, u.avatar_key",
			`JOIN "user" u ON u.id = a.id`,
			"WHERE u.status = 'active'",
			"u.username ILIKE $1 OR u.nickname ILIKE $1",
			"a.category = ANY($2)",
			"($3::text IS NULL OR (lower(u.nickname), u.id) > ($3::text, $4::uuid))",
			"ORDER BY lower(u.nickname), u.id",
			"LIMIT $5",
		)).
			WithArgs("%ivan%", categories, noNickname, noID, 21).
			WillReturnRows(authorRows(nil))

		got, err := repo.SearchAuthors(context.Background(), "ivan", categories, nil, 21)

		require.NoError(t, err)
		require.Empty(t, got)
		require.NoError(t, pool.ExpectationsWereMet())
	})
}

func TestRepository_CreateAuthor(t *testing.T) {
	authorID := uuid.New()
	category := "Творчество"

	insertQuery := testutils.SQLInOrder(
		"INSERT INTO author (id, bio, category)",
		"ON CONFLICT (id) DO NOTHING",
	)
	selectQuery := testutils.SQLInOrder(
		"SELECT a.id, u.username, u.nickname, a.bio, a.category",
		`JOIN "user" u ON u.id = a.id`,
		"WHERE a.id = $1",
	)

	t.Run("creates author", func(t *testing.T) {
		repo, pool := newTestRepository(t)
		pool.ExpectExec(insertQuery).
			WithArgs(authorID, "Пишу о книгах", &category).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))
		pool.ExpectQuery(selectQuery).
			WithArgs(authorID).
			WillReturnRows(pgxmock.NewRows(authorMeCols).AddRow(authorID, "ivan001", "Иван", "Пишу о книгах", &category))

		got, err := repo.CreateAuthor(context.Background(), authorID, "Пишу о книгах", &category)

		require.NoError(t, err)
		require.Equal(t, &models.Author{ID: authorID, Username: "ivan001", Nickname: "Иван", Bio: "Пишу о книгах", Category: &category}, got)
		require.NoError(t, pool.ExpectationsWereMet())
	})

	t.Run("creates author without category", func(t *testing.T) {
		repo, pool := newTestRepository(t)
		pool.ExpectExec(insertQuery).
			WithArgs(authorID, "Пишу о книгах", noCategory).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))
		pool.ExpectQuery(selectQuery).
			WithArgs(authorID).
			WillReturnRows(pgxmock.NewRows(authorMeCols).AddRow(authorID, "ivan001", "Иван", "Пишу о книгах", nil))

		got, err := repo.CreateAuthor(context.Background(), authorID, "Пишу о книгах", nil)

		require.NoError(t, err)
		require.Nil(t, got.Category)
		require.NoError(t, pool.ExpectationsWereMet())
	})

	t.Run("already exists returns existing row", func(t *testing.T) {
		repo, pool := newTestRepository(t)
		pool.ExpectExec(insertQuery).
			WithArgs(authorID, "Пишу о книгах", &category).
			WillReturnResult(pgxmock.NewResult("INSERT", 0))
		pool.ExpectQuery(selectQuery).
			WithArgs(authorID).
			WillReturnRows(pgxmock.NewRows(authorMeCols).AddRow(authorID, "ivan001", "Иван", "Старое био", &category))

		got, err := repo.CreateAuthor(context.Background(), authorID, "Пишу о книгах", &category)

		require.NoError(t, err)
		require.Equal(t, "Старое био", got.Bio)
		require.NoError(t, pool.ExpectationsWereMet())
	})
}

func TestRepository_GetAuthorByID(t *testing.T) {
	authorID := uuid.New()
	category := "Игры"

	query := testutils.SQLInOrder(
		"SELECT a.id, u.username, u.nickname, a.bio, a.category",
		`JOIN "user" u ON u.id = a.id`,
		"WHERE a.id = $1",
	)

	t.Run("found", func(t *testing.T) {
		repo, pool := newTestRepository(t)
		pool.ExpectQuery(query).
			WithArgs(authorID).
			WillReturnRows(pgxmock.NewRows(authorMeCols).AddRow(authorID, "ivan001", "Иван", "Пишу об играх", &category))

		got, err := repo.GetAuthorByID(context.Background(), authorID)

		require.NoError(t, err)
		require.Equal(t, &models.Author{ID: authorID, Username: "ivan001", Nickname: "Иван", Bio: "Пишу об играх", Category: &category}, got)
		require.NoError(t, pool.ExpectationsWereMet())
	})

	t.Run("not found", func(t *testing.T) {
		repo, pool := newTestRepository(t)
		pool.ExpectQuery(query).
			WithArgs(authorID).
			WillReturnRows(pgxmock.NewRows(authorMeCols))

		got, err := repo.GetAuthorByID(context.Background(), authorID)

		require.ErrorIs(t, err, models.ErrAuthorNotFound)
		require.Nil(t, got)
		require.NoError(t, pool.ExpectationsWereMet())
	})
}

func TestRepository_UpdateAuthorDetails(t *testing.T) {
	authorID := uuid.New()
	category := "Игры"

	updateQuery := testutils.SQLInOrder(
		"WITH upd AS (",
		"UPDATE author SET bio = $2, category = $3",
		"WHERE id = $1",
		"RETURNING id, bio, category",
		")",
		"SELECT upd.id, u.username, u.nickname, upd.bio, upd.category",
		`JOIN "user" u ON u.id = upd.id`,
	)

	t.Run("updates and returns block", func(t *testing.T) {
		repo, pool := newTestRepository(t)
		pool.ExpectQuery(updateQuery).
			WithArgs(authorID, "Новое био", &category).
			WillReturnRows(pgxmock.NewRows(authorMeCols).AddRow(authorID, "ivan001", "Иван", "Новое био", &category))

		got, err := repo.UpdateAuthorDetails(context.Background(), authorID, "Новое био", &category)

		require.NoError(t, err)
		require.Equal(t, &models.Author{ID: authorID, Username: "ivan001", Nickname: "Иван", Bio: "Новое био", Category: &category}, got)
		require.NoError(t, pool.ExpectationsWereMet())
	})

	t.Run("clears category", func(t *testing.T) {
		repo, pool := newTestRepository(t)
		pool.ExpectQuery(updateQuery).
			WithArgs(authorID, "Новое био", noCategory).
			WillReturnRows(pgxmock.NewRows(authorMeCols).AddRow(authorID, "ivan001", "Иван", "Новое био", nil))

		got, err := repo.UpdateAuthorDetails(context.Background(), authorID, "Новое био", nil)

		require.NoError(t, err)
		require.Nil(t, got.Category)
		require.NoError(t, pool.ExpectationsWereMet())
	})

	t.Run("author row not found", func(t *testing.T) {
		repo, pool := newTestRepository(t)
		pool.ExpectQuery(updateQuery).
			WithArgs(authorID, "Новое био", &category).
			WillReturnRows(pgxmock.NewRows(authorMeCols))

		got, err := repo.UpdateAuthorDetails(context.Background(), authorID, "Новое био", &category)

		require.ErrorIs(t, err, models.ErrAuthorNotFound)
		require.Nil(t, got)
		require.NoError(t, pool.ExpectationsWereMet())
	})
}
