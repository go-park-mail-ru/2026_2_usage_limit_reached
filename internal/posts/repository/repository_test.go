package postrepository

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/models"
)

var (
	errDB = errors.New("connection reset")

	userID   = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	authorID = uuid.MustParse("00000000-0000-4000-8000-000000000002")
	postID   = uuid.MustParse("00000000-0000-4000-8000-000000000101")
	likeID   = uuid.MustParse("00000000-0000-4000-8000-000000000201")

	publishedAt = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	createdAt   = time.Date(2026, 9, 1, 11, 0, 0, 0, time.UTC)

	previewColumns = []string{
		"id", "author_id", "title", "body_preview", "published_at", "likes_count",
		"username", "nickname", "avatar_key", "locked", "liked_by_me",
	}
)

func newRepo(t *testing.T) (*PostRepository, pgxmock.PgxPoolIface) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, mock.ExpectationsWereMet())
		mock.Close()
	})
	return &PostRepository{pool: mock, logger: slog.New(slog.DiscardHandler)}, mock
}

func sqlInOrder(fragments ...string) string {
	quoted := make([]string, len(fragments))
	for i, f := range fragments {
		quoted[i] = regexp.QuoteMeta(f)
	}
	return "(?s)" + strings.Join(quoted, ".*")
}

func pgError(code string) error {
	return &pgconn.PgError{Code: code}
}

func ptr[T any](v T) *T {
	return &v
}

func samplePreview() *dto.PostPreview {
	return &dto.PostPreview{
		ID: postID,
		Author: dto.AuthorResponse{
			ID:        authorID,
			Username:  "author",
			Nickname:  "Author",
			AvatarUrl: ptr("avatars/author.png"),
		},
		Title:                "Title",
		BodyPreview:          "Body",
		PublishedOrUpdatedAt: ptr(publishedAt),
		LikesCount:           12,
		LikedByUser:          true,
	}
}

func previewRows(posts ...*dto.PostPreview) *pgxmock.Rows {
	rows := pgxmock.NewRows(previewColumns)
	for _, p := range posts {
		rows.AddRow(p.ID, p.Author.ID, p.Title, p.BodyPreview, p.PublishedOrUpdatedAt, p.LikesCount,
			p.Author.Username, p.Author.Nickname, p.Author.AvatarUrl, p.Locked, p.LikedByUser)
	}
	return rows
}

func TestCreatePost(t *testing.T) {
	post := models.Post{Title: "Title", Body: "Body", Status: "published"}
	createQuery := sqlInOrder("INSERT INTO post (author_id, title, body, status, published_at)", "RETURNING id")

	t.Run("returns id of created post", func(t *testing.T) {
		repo, mock := newRepo(t)
		mock.ExpectQuery(createQuery).
			WithArgs(&userID, post.Title, post.Body, post.Status).
			WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(postID))

		id, err := repo.CreatePost(context.Background(), &userID, post)

		require.NoError(t, err)
		assert.Equal(t, postID, id)
	})

	tests := []struct {
		name    string
		dbErr   error
		wantErr error
	}{
		{"unknown author is reported as invalid author", pgError("23503"), models.ErrInvalidAuthorID},
		{"check constraint is reported as validation error", pgError("23514"), models.ErrPostValidation},
		{"other database errors are propagated", errDB, errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newRepo(t)
			mock.ExpectQuery(createQuery).
				WithArgs(&userID, post.Title, post.Body, post.Status).
				WillReturnError(tt.dbErr)

			id, err := repo.CreatePost(context.Background(), &userID, post)

			require.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, uuid.Nil, id)
		})
	}
}

func TestGetPost(t *testing.T) {
	getQuery := sqlInOrder("FROM post p", `JOIN "user" u ON u.id = p.author_id AND u.status = 'active'`,
		"WHERE p.id = $2", "AND (p.status = 'published' OR p.author_id = $1)")
	columns := []string{
		"id", "author_id", "title", "body", "status", "published_at", "likes_count", "created_at", "updated_at",
		"username", "nickname", "avatar_key", "locked", "liked_by_me",
	}
	want := &dto.PostInfo{
		ID:          postID,
		Author:      dto.AuthorResponse{ID: authorID, Username: "author", Nickname: "Author"},
		Title:       "Title",
		Body:        "",
		Status:      "published",
		PublishedAt: ptr(publishedAt),
		CreatedAt:   createdAt,
		UpdatedAt:   publishedAt,
		Locked:      true,
		LikesCount:  3,
		LikedByUser: false,
	}

	for _, viewer := range []struct {
		name string
		id   *uuid.UUID
	}{{"authorized viewer", &userID}, {"anonymous viewer", nil}} {
		t.Run("maps all columns for "+viewer.name, func(t *testing.T) {
			repo, mock := newRepo(t)
			mock.ExpectQuery(getQuery).
				WithArgs(viewer.id, postID).
				WillReturnRows(pgxmock.NewRows(columns).AddRow(
					want.ID, want.Author.ID, want.Title, want.Body, want.Status, want.PublishedAt, want.LikesCount,
					want.CreatedAt, want.UpdatedAt, want.Author.Username, want.Author.Nickname, want.Author.AvatarUrl,
					want.Locked, want.LikedByUser,
				))

			got, err := repo.GetPost(context.Background(), viewer.id, postID)

			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}

	t.Run("missing or hidden post is not found", func(t *testing.T) {
		repo, mock := newRepo(t)
		mock.ExpectQuery(getQuery).WithArgs(&userID, postID).WillReturnError(pgx.ErrNoRows)

		got, err := repo.GetPost(context.Background(), &userID, postID)

		require.ErrorIs(t, err, models.ErrPostNotFound)
		assert.Nil(t, got)
	})

	t.Run("database error is propagated", func(t *testing.T) {
		repo, mock := newRepo(t)
		mock.ExpectQuery(getQuery).WithArgs(&userID, postID).WillReturnError(errDB)

		got, err := repo.GetPost(context.Background(), &userID, postID)

		require.ErrorIs(t, err, errDB)
		assert.NotErrorIs(t, err, models.ErrPostNotFound)
		assert.Nil(t, got)
	})
}

type listCase struct {
	name  string
	query string
	args  []any
	call  func(ctx context.Context, r *PostRepository) ([]*dto.PostPreview, error)
}

func timedCursor() *dto.TimedPostsCursor {
	return &dto.TimedPostsCursor{PublishedOrUpdatedAt: publishedAt, PostID: postID}
}

func popularCursor() *dto.PopularPostsCursor {
	return &dto.PopularPostsCursor{Likes: 12, PublishedAt: publishedAt, PostID: postID}
}

func timedCases(name, query string, method func(*PostRepository) func(context.Context, *uuid.UUID, *dto.TimedPostsCursor, int) ([]*dto.PostPreview, error)) []listCase {
	cur := timedCursor()
	return []listCase{
		{
			name:  name + " first page",
			query: query,
			args:  []any{&userID, (*time.Time)(nil), (*uuid.UUID)(nil), 21},
			call: func(ctx context.Context, r *PostRepository) ([]*dto.PostPreview, error) {
				return method(r)(ctx, &userID, nil, 21)
			},
		},
		{
			name:  name + " next page",
			query: query,
			args:  []any{&userID, &cur.PublishedOrUpdatedAt, &cur.PostID, 21},
			call: func(ctx context.Context, r *PostRepository) ([]*dto.PostPreview, error) {
				return method(r)(ctx, &userID, cur, 21)
			},
		},
	}
}

func popularCases(name, query string, method func(*PostRepository) func(context.Context, *uuid.UUID, *dto.PopularPostsCursor, int) ([]*dto.PostPreview, error)) []listCase {
	cur := popularCursor()
	return []listCase{
		{
			name:  name + " first page",
			query: query,
			args:  []any{&userID, (*int64)(nil), (*time.Time)(nil), (*uuid.UUID)(nil), 21},
			call: func(ctx context.Context, r *PostRepository) ([]*dto.PostPreview, error) {
				return method(r)(ctx, &userID, nil, 21)
			},
		},
		{
			name:  name + " next page",
			query: query,
			args:  []any{&userID, &cur.Likes, &cur.PublishedAt, &cur.PostID, 21},
			call: func(ctx context.Context, r *PostRepository) ([]*dto.PostPreview, error) {
				return method(r)(ctx, &userID, cur, 21)
			},
		},
	}
}

func userTimedCases(name, query string, method func(*PostRepository) func(context.Context, *uuid.UUID, string, *dto.TimedPostsCursor, int) ([]*dto.PostPreview, error)) []listCase {
	cur := timedCursor()
	return []listCase{
		{
			name:  name + " first page",
			query: query,
			args:  []any{&userID, "author", (*time.Time)(nil), (*uuid.UUID)(nil), 21},
			call: func(ctx context.Context, r *PostRepository) ([]*dto.PostPreview, error) {
				return method(r)(ctx, &userID, "author", nil, 21)
			},
		},
		{
			name:  name + " next page",
			query: query,
			args:  []any{&userID, "author", &cur.PublishedOrUpdatedAt, &cur.PostID, 21},
			call: func(ctx context.Context, r *PostRepository) ([]*dto.PostPreview, error) {
				return method(r)(ctx, &userID, "author", cur, 21)
			},
		},
	}
}

func userPopularCases(name, query string, method func(*PostRepository) func(context.Context, *uuid.UUID, string, *dto.PopularPostsCursor, int) ([]*dto.PostPreview, error)) []listCase {
	cur := popularCursor()
	return []listCase{
		{
			name:  name + " first page",
			query: query,
			args:  []any{&userID, "author", (*int64)(nil), (*time.Time)(nil), (*uuid.UUID)(nil), 21},
			call: func(ctx context.Context, r *PostRepository) ([]*dto.PostPreview, error) {
				return method(r)(ctx, &userID, "author", nil, 21)
			},
		},
		{
			name:  name + " next page",
			query: query,
			args:  []any{&userID, "author", &cur.Likes, &cur.PublishedAt, &cur.PostID, 21},
			call: func(ctx context.Context, r *PostRepository) ([]*dto.PostPreview, error) {
				return method(r)(ctx, &userID, "author", cur, 21)
			},
		},
	}
}

func allListCases() []listCase {
	var cases []listCase
	cases = append(cases, timedCases("GetMyNewPosts",
		sqlInOrder("WHERE p.status = 'published'", "(p.published_at, p.id) < ($2::timestamptz, $3::uuid)",
			"AND p.author_id = $1", "ORDER BY p.published_at DESC, p.id DESC", "LIMIT $4"),
		func(r *PostRepository) func(context.Context, *uuid.UUID, *dto.TimedPostsCursor, int) ([]*dto.PostPreview, error) {
			return r.GetMyNewPosts
		})...)
	cases = append(cases, timedCases("GetMyOldPosts",
		sqlInOrder("WHERE p.status = 'published'", "(p.published_at, p.id) > ($2::timestamptz, $3::uuid)",
			"AND p.author_id = $1", "ORDER BY p.published_at, p.id", "LIMIT $4"),
		func(r *PostRepository) func(context.Context, *uuid.UUID, *dto.TimedPostsCursor, int) ([]*dto.PostPreview, error) {
			return r.GetMyOldPosts
		})...)
	cases = append(cases, timedCases("GetMyNewDrafts",
		sqlInOrder("p.updated_at", "WHERE p.status = 'draft'", "(p.updated_at, p.id) < ($2::timestamptz, $3::uuid)",
			"AND p.author_id = $1", "ORDER BY p.updated_at DESC, p.id DESC", "LIMIT $4"),
		func(r *PostRepository) func(context.Context, *uuid.UUID, *dto.TimedPostsCursor, int) ([]*dto.PostPreview, error) {
			return r.GetMyNewDrafts
		})...)
	cases = append(cases, timedCases("GetMyOldDrafts",
		sqlInOrder("p.updated_at", "WHERE p.status = 'draft'", "(p.updated_at, p.id) > ($2::timestamptz, $3::uuid)",
			"AND p.author_id = $1", "ORDER BY p.updated_at, p.id", "LIMIT $4"),
		func(r *PostRepository) func(context.Context, *uuid.UUID, *dto.TimedPostsCursor, int) ([]*dto.PostPreview, error) {
			return r.GetMyOldDrafts
		})...)
	cases = append(cases, timedCases("GetNewPosts",
		sqlInOrder("WITH page AS", "WHERE p.status = 'published' AND u.status = 'active'",
			"(p.published_at, p.id) < ($2::timestamptz, $3::uuid)", "ORDER BY p.published_at DESC, p.id DESC", "LIMIT $4",
			"LEFT JOIN post_access pa", "ORDER BY p.published_at DESC, p.id DESC"),
		func(r *PostRepository) func(context.Context, *uuid.UUID, *dto.TimedPostsCursor, int) ([]*dto.PostPreview, error) {
			return r.GetNewPosts
		})...)
	cases = append(cases, timedCases("GetOldPosts",
		sqlInOrder("WITH page AS", "WHERE p.status = 'published' AND u.status = 'active'",
			"(p.published_at, p.id) > ($2::timestamptz, $3::uuid)", "ORDER BY p.published_at, p.id", "LIMIT $4",
			"LEFT JOIN post_access pa", "ORDER BY p.published_at, p.id"),
		func(r *PostRepository) func(context.Context, *uuid.UUID, *dto.TimedPostsCursor, int) ([]*dto.PostPreview, error) {
			return r.GetOldPosts
		})...)
	cases = append(cases, popularCases("GetMyPopularPosts",
		sqlInOrder("WHERE p.status = 'published'",
			"(p.likes_count, p.published_at, p.id) < ($2::bigint, $3::timestamptz, $4::uuid)",
			"AND p.author_id = $1", "ORDER BY p.likes_count DESC, p.published_at DESC, p.id DESC", "LIMIT $5"),
		func(r *PostRepository) func(context.Context, *uuid.UUID, *dto.PopularPostsCursor, int) ([]*dto.PostPreview, error) {
			return r.GetMyPopularPosts
		})...)
	cases = append(cases, popularCases("GetPopularPosts",
		sqlInOrder("WITH page AS", "WHERE p.status = 'published' AND u.status = 'active'",
			"(p.likes_count, p.published_at, p.id) < ($2::bigint, $3::timestamptz, $4::uuid)",
			"ORDER BY p.likes_count DESC, p.published_at DESC, p.id DESC", "LIMIT $5", "LEFT JOIN post_access pa"),
		func(r *PostRepository) func(context.Context, *uuid.UUID, *dto.PopularPostsCursor, int) ([]*dto.PostPreview, error) {
			return r.GetPopularPosts
		})...)
	cases = append(cases, userTimedCases("GetUserNewPosts",
		sqlInOrder("WITH page AS", "WHERE p.status = 'published'", "AND u.username = $2",
			"(p.published_at, p.id) < ($3::timestamptz, $4::uuid)", "ORDER BY p.published_at DESC, p.id DESC", "LIMIT $5",
			"LEFT JOIN post_access pa"),
		func(r *PostRepository) func(context.Context, *uuid.UUID, string, *dto.TimedPostsCursor, int) ([]*dto.PostPreview, error) {
			return r.GetUserNewPosts
		})...)
	cases = append(cases, userTimedCases("GetUserOldPosts",
		sqlInOrder("WITH page AS", "WHERE p.status = 'published'", "AND u.username = $2",
			"(p.published_at, p.id) > ($3::timestamptz, $4::uuid)", "ORDER BY p.published_at ASC, p.id ASC", "LIMIT $5",
			"LEFT JOIN post_access pa"),
		func(r *PostRepository) func(context.Context, *uuid.UUID, string, *dto.TimedPostsCursor, int) ([]*dto.PostPreview, error) {
			return r.GetUserOldPosts
		})...)
	cases = append(cases, userPopularCases("GetUserPopularPosts",
		sqlInOrder("WITH page AS", "WHERE p.status = 'published'", "AND u.username = $2",
			"(p.likes_count, p.published_at, p.id) < ($3::bigint, $4::timestamptz, $5::uuid)",
			"ORDER BY p.likes_count DESC, p.published_at DESC, p.id DESC", "LIMIT $6", "LEFT JOIN post_access pa"),
		func(r *PostRepository) func(context.Context, *uuid.UUID, string, *dto.PopularPostsCursor, int) ([]*dto.PostPreview, error) {
			return r.GetUserPopularPosts
		})...)
	return cases
}

func TestListPosts_ReturnsScannedPreviews(t *testing.T) {
	locked := samplePreview()
	locked.ID = uuid.MustParse("00000000-0000-4000-8000-000000000102")
	locked.BodyPreview = ""
	locked.Locked = true
	locked.LikedByUser = false
	locked.Author.AvatarUrl = nil

	for _, tc := range allListCases() {
		t.Run(tc.name, func(t *testing.T) {
			repo, mock := newRepo(t)
			mock.ExpectQuery(tc.query).WithArgs(tc.args...).WillReturnRows(previewRows(samplePreview(), locked))

			got, err := tc.call(context.Background(), repo)

			require.NoError(t, err)
			assert.Equal(t, []*dto.PostPreview{samplePreview(), locked}, got)
		})
	}
}

func TestListPosts_EmptyResultIsEmptySlice(t *testing.T) {
	for _, tc := range allListCases() {
		t.Run(tc.name, func(t *testing.T) {
			repo, mock := newRepo(t)
			mock.ExpectQuery(tc.query).WithArgs(tc.args...).WillReturnRows(previewRows())

			got, err := tc.call(context.Background(), repo)

			require.NoError(t, err)
			assert.NotNil(t, got)
			assert.Empty(t, got)
		})
	}
}

func TestListPosts_PropagatesQueryError(t *testing.T) {
	for _, tc := range allListCases() {
		t.Run(tc.name, func(t *testing.T) {
			repo, mock := newRepo(t)
			mock.ExpectQuery(tc.query).WithArgs(tc.args...).WillReturnError(errDB)

			got, err := tc.call(context.Background(), repo)

			require.ErrorIs(t, err, errDB)
			assert.Nil(t, got)
		})
	}
}

func TestListPosts_PropagatesRowIterationError(t *testing.T) {
	for _, tc := range allListCases() {
		t.Run(tc.name, func(t *testing.T) {
			repo, mock := newRepo(t)
			mock.ExpectQuery(tc.query).WithArgs(tc.args...).
				WillReturnRows(previewRows(samplePreview(), samplePreview()).RowError(1, errDB))

			got, err := tc.call(context.Background(), repo)

			require.ErrorIs(t, err, errDB)
			assert.Nil(t, got)
		})
	}
}

func TestListPosts_FailsOnUnexpectedColumns(t *testing.T) {
	for _, tc := range allListCases() {
		t.Run(tc.name, func(t *testing.T) {
			repo, mock := newRepo(t)
			mock.ExpectQuery(tc.query).WithArgs(tc.args...).
				WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(postID))

			got, err := tc.call(context.Background(), repo)

			require.Error(t, err)
			assert.Nil(t, got)
		})
	}
}

func TestUpdatePost(t *testing.T) {
	post := models.Post{ID: postID, Title: "New title", Body: "New body", Status: "published"}
	lastUpdatedAt := publishedAt
	updateQuery := sqlInOrder("UPDATE post SET", "WHERE id = $1 AND author_id = $2 AND updated_at = $6")
	selectQuery := sqlInOrder("SELECT author_id, updated_at FROM post WHERE id = $1")
	expectUpdate := func(mock pgxmock.PgxPoolIface) *pgxmock.ExpectedExec {
		return mock.ExpectExec(updateQuery).
			WithArgs(post.ID, &userID, post.Title, post.Body, post.Status, lastUpdatedAt)
	}

	t.Run("updates post with actual version", func(t *testing.T) {
		repo, mock := newRepo(t)
		expectUpdate(mock).WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		require.NoError(t, repo.UpdatePost(context.Background(), &userID, post, lastUpdatedAt))
	})

	t.Run("check constraint is reported as validation error", func(t *testing.T) {
		repo, mock := newRepo(t)
		expectUpdate(mock).WillReturnError(pgError("23514"))

		require.ErrorIs(t, repo.UpdatePost(context.Background(), &userID, post, lastUpdatedAt), models.ErrPostValidation)
	})

	t.Run("update database error is propagated", func(t *testing.T) {
		repo, mock := newRepo(t)
		expectUpdate(mock).WillReturnError(errDB)

		require.ErrorIs(t, repo.UpdatePost(context.Background(), &userID, post, lastUpdatedAt), errDB)
	})

	notUpdated := []struct {
		name    string
		author  uuid.UUID
		version time.Time
		wantErr error
	}{
		{"post of another author is hidden as not found", authorID, lastUpdatedAt, models.ErrPostNotFound},
		{"stale version is a conflict", userID, lastUpdatedAt.Add(time.Second), models.ErrPostConflict},
		{"version changed and restored concurrently is a conflict", userID, lastUpdatedAt, models.ErrPostConflict},
	}
	for _, tt := range notUpdated {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newRepo(t)
			expectUpdate(mock).WillReturnResult(pgxmock.NewResult("UPDATE", 0))
			mock.ExpectQuery(selectQuery).WithArgs(post.ID).
				WillReturnRows(pgxmock.NewRows([]string{"author_id", "updated_at"}).AddRow(tt.author, tt.version))

			require.ErrorIs(t, repo.UpdatePost(context.Background(), &userID, post, lastUpdatedAt), tt.wantErr)
		})
	}

	t.Run("missing post is not found", func(t *testing.T) {
		repo, mock := newRepo(t)
		expectUpdate(mock).WillReturnResult(pgxmock.NewResult("UPDATE", 0))
		mock.ExpectQuery(selectQuery).WithArgs(post.ID).WillReturnError(pgx.ErrNoRows)

		require.ErrorIs(t, repo.UpdatePost(context.Background(), &userID, post, lastUpdatedAt), models.ErrPostNotFound)
	})

	t.Run("lookup database error is propagated", func(t *testing.T) {
		repo, mock := newRepo(t)
		expectUpdate(mock).WillReturnResult(pgxmock.NewResult("UPDATE", 0))
		mock.ExpectQuery(selectQuery).WithArgs(post.ID).WillReturnError(errDB)

		err := repo.UpdatePost(context.Background(), &userID, post, lastUpdatedAt)

		require.ErrorIs(t, err, errDB)
		assert.NotErrorIs(t, err, models.ErrPostNotFound)
	})
}

func TestDeletePost(t *testing.T) {
	deleteQuery := sqlInOrder("DELETE FROM post WHERE id = $1 AND author_id = $2")

	t.Run("deletes own post", func(t *testing.T) {
		repo, mock := newRepo(t)
		mock.ExpectExec(deleteQuery).WithArgs(postID, &userID).WillReturnResult(pgxmock.NewResult("DELETE", 1))

		require.NoError(t, repo.DeletePost(context.Background(), &userID, postID))
	})

	t.Run("missing or foreign post is not found", func(t *testing.T) {
		repo, mock := newRepo(t)
		mock.ExpectExec(deleteQuery).WithArgs(postID, &userID).WillReturnResult(pgxmock.NewResult("DELETE", 0))

		require.ErrorIs(t, repo.DeletePost(context.Background(), &userID, postID), models.ErrPostNotFound)
	})

	t.Run("database error is propagated", func(t *testing.T) {
		repo, mock := newRepo(t)
		mock.ExpectExec(deleteQuery).WithArgs(postID, &userID).WillReturnError(errDB)

		err := repo.DeletePost(context.Background(), &userID, postID)

		require.ErrorIs(t, err, errDB)
		assert.NotErrorIs(t, err, models.ErrPostNotFound)
	})
}

var (
	likeTxOptions       = pgx.TxOptions{IsoLevel: pgx.ReadCommitted}
	selectAvailablePost = sqlInOrder("SELECT p.author_id", "WHERE p.id = $2", "AND p.status = 'published'", "FOR KEY SHARE OF p")
	insertLikeQuery     = sqlInOrder("INSERT INTO post_like (post_id, user_id)", "ON CONFLICT (post_id, user_id) DO NOTHING", "RETURNING id")
	insertNotification  = sqlInOrder("INSERT INTO notification (user_id, event_kind, post_like_id)", "'post_liked'")
	selectLikesCount    = sqlInOrder("SELECT likes_count FROM post WHERE id = $1")
	authorColumn        = []string{"author_id"}
	likeIDColumn        = []string{"id"}
	likesCountColumn    = []string{"likes_count"}
)

func expectTxCommitted(mock pgxmock.PgxPoolIface) {
	mock.ExpectCommit()
	mock.ExpectRollback().WillReturnError(pgx.ErrTxClosed)
}

func expectTxRolledBack(mock pgxmock.PgxPoolIface) {
	mock.ExpectRollback()
	mock.ExpectRollback().WillReturnError(pgx.ErrTxClosed)
}

func TestLikePost(t *testing.T) {
	t.Run("new like notifies author and returns likes count", func(t *testing.T) {
		repo, mock := newRepo(t)
		mock.ExpectBeginTx(likeTxOptions)
		mock.ExpectQuery(selectAvailablePost).WithArgs(&userID, postID).
			WillReturnRows(pgxmock.NewRows(authorColumn).AddRow(authorID))
		mock.ExpectQuery(insertLikeQuery).WithArgs(postID, &userID).
			WillReturnRows(pgxmock.NewRows(likeIDColumn).AddRow(likeID))
		mock.ExpectExec(insertNotification).WithArgs(authorID, likeID).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))
		mock.ExpectQuery(selectLikesCount).WithArgs(postID).
			WillReturnRows(pgxmock.NewRows(likesCountColumn).AddRow(int64(13)))
		expectTxCommitted(mock)

		likes, err := repo.LikePost(context.Background(), &userID, postID)

		require.NoError(t, err)
		assert.Equal(t, int64(13), likes)
	})

	t.Run("liking own post does not notify", func(t *testing.T) {
		repo, mock := newRepo(t)
		mock.ExpectBeginTx(likeTxOptions)
		mock.ExpectQuery(selectAvailablePost).WithArgs(&userID, postID).
			WillReturnRows(pgxmock.NewRows(authorColumn).AddRow(userID))
		mock.ExpectQuery(insertLikeQuery).WithArgs(postID, &userID).
			WillReturnRows(pgxmock.NewRows(likeIDColumn).AddRow(likeID))
		mock.ExpectQuery(selectLikesCount).WithArgs(postID).
			WillReturnRows(pgxmock.NewRows(likesCountColumn).AddRow(int64(1)))
		expectTxCommitted(mock)

		likes, err := repo.LikePost(context.Background(), &userID, postID)

		require.NoError(t, err)
		assert.Equal(t, int64(1), likes)
	})

	t.Run("repeated like is idempotent and does not notify", func(t *testing.T) {
		repo, mock := newRepo(t)
		mock.ExpectBeginTx(likeTxOptions)
		mock.ExpectQuery(selectAvailablePost).WithArgs(&userID, postID).
			WillReturnRows(pgxmock.NewRows(authorColumn).AddRow(authorID))
		mock.ExpectQuery(insertLikeQuery).WithArgs(postID, &userID).
			WillReturnRows(pgxmock.NewRows(likeIDColumn))
		mock.ExpectQuery(selectLikesCount).WithArgs(postID).
			WillReturnRows(pgxmock.NewRows(likesCountColumn).AddRow(int64(13)))
		expectTxCommitted(mock)

		likes, err := repo.LikePost(context.Background(), &userID, postID)

		require.NoError(t, err)
		assert.Equal(t, int64(13), likes)
	})

	t.Run("unavailable post is not found and transaction is rolled back", func(t *testing.T) {
		repo, mock := newRepo(t)
		mock.ExpectBeginTx(likeTxOptions)
		mock.ExpectQuery(selectAvailablePost).WithArgs(&userID, postID).
			WillReturnRows(pgxmock.NewRows(authorColumn))
		expectTxRolledBack(mock)

		likes, err := repo.LikePost(context.Background(), &userID, postID)

		require.ErrorIs(t, err, models.ErrPostNotFound)
		assert.Zero(t, likes)
	})

	t.Run("begin error is propagated", func(t *testing.T) {
		repo, mock := newRepo(t)
		mock.ExpectBeginTx(likeTxOptions).WillReturnError(errDB)

		likes, err := repo.LikePost(context.Background(), &userID, postID)

		require.ErrorIs(t, err, errDB)
		assert.Zero(t, likes)
	})

	failures := []struct {
		name   string
		expect func(mock pgxmock.PgxPoolIface)
	}{
		{"post lookup fails", func(mock pgxmock.PgxPoolIface) {
			mock.ExpectQuery(selectAvailablePost).WithArgs(&userID, postID).WillReturnError(errDB)
		}},
		{"like insert fails", func(mock pgxmock.PgxPoolIface) {
			mock.ExpectQuery(selectAvailablePost).WithArgs(&userID, postID).
				WillReturnRows(pgxmock.NewRows(authorColumn).AddRow(authorID))
			mock.ExpectQuery(insertLikeQuery).WithArgs(postID, &userID).WillReturnError(errDB)
		}},
		{"notification insert fails", func(mock pgxmock.PgxPoolIface) {
			mock.ExpectQuery(selectAvailablePost).WithArgs(&userID, postID).
				WillReturnRows(pgxmock.NewRows(authorColumn).AddRow(authorID))
			mock.ExpectQuery(insertLikeQuery).WithArgs(postID, &userID).
				WillReturnRows(pgxmock.NewRows(likeIDColumn).AddRow(likeID))
			mock.ExpectExec(insertNotification).WithArgs(authorID, likeID).WillReturnError(errDB)
		}},
		{"likes count fails", func(mock pgxmock.PgxPoolIface) {
			mock.ExpectQuery(selectAvailablePost).WithArgs(&userID, postID).
				WillReturnRows(pgxmock.NewRows(authorColumn).AddRow(userID))
			mock.ExpectQuery(insertLikeQuery).WithArgs(postID, &userID).
				WillReturnRows(pgxmock.NewRows(likeIDColumn).AddRow(likeID))
			mock.ExpectQuery(selectLikesCount).WithArgs(postID).WillReturnError(errDB)
		}},
	}
	for _, tt := range failures {
		t.Run(tt.name+" rolls back", func(t *testing.T) {
			repo, mock := newRepo(t)
			mock.ExpectBeginTx(likeTxOptions)
			tt.expect(mock)
			expectTxRolledBack(mock)

			likes, err := repo.LikePost(context.Background(), &userID, postID)

			require.ErrorIs(t, err, errDB)
			assert.Zero(t, likes)
		})
	}
}

func TestDeletePostLike(t *testing.T) {
	deleteLikeQuery := sqlInOrder("DELETE FROM post_like WHERE post_id = $1 AND user_id = $2")

	for _, deleted := range []int64{1, 0} {
		t.Run("returns likes count regardless of whether like existed", func(t *testing.T) {
			repo, mock := newRepo(t)
			mock.ExpectExec(deleteLikeQuery).WithArgs(postID, &userID).
				WillReturnResult(pgxmock.NewResult("DELETE", deleted))
			mock.ExpectQuery(selectLikesCount).WithArgs(postID).
				WillReturnRows(pgxmock.NewRows(likesCountColumn).AddRow(int64(12)))

			likes, err := repo.DeletePostLike(context.Background(), &userID, postID)

			require.NoError(t, err)
			assert.Equal(t, int64(12), likes)
		})
	}

	t.Run("delete database error is propagated", func(t *testing.T) {
		repo, mock := newRepo(t)
		mock.ExpectExec(deleteLikeQuery).WithArgs(postID, &userID).WillReturnError(errDB)

		likes, err := repo.DeletePostLike(context.Background(), &userID, postID)

		require.ErrorIs(t, err, errDB)
		assert.NotErrorIs(t, err, models.ErrPostNotFound)
		assert.Zero(t, likes)
	})

	t.Run("missing post is not found", func(t *testing.T) {
		repo, mock := newRepo(t)
		mock.ExpectExec(deleteLikeQuery).WithArgs(postID, &userID).
			WillReturnResult(pgxmock.NewResult("DELETE", 0))
		mock.ExpectQuery(selectLikesCount).WithArgs(postID).WillReturnRows(pgxmock.NewRows(likesCountColumn))

		likes, err := repo.DeletePostLike(context.Background(), &userID, postID)

		require.ErrorIs(t, err, models.ErrPostNotFound)
		assert.Zero(t, likes)
	})

	t.Run("likes count database error is propagated", func(t *testing.T) {
		repo, mock := newRepo(t)
		mock.ExpectExec(deleteLikeQuery).WithArgs(postID, &userID).
			WillReturnResult(pgxmock.NewResult("DELETE", 1))
		mock.ExpectQuery(selectLikesCount).WithArgs(postID).WillReturnError(errDB)

		likes, err := repo.DeletePostLike(context.Background(), &userID, postID)

		require.ErrorIs(t, err, errDB)
		assert.Zero(t, likes)
	})
}
