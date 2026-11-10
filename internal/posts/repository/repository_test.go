package postrepository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/testutils"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v5"
	"github.com/stretchr/testify/require"
)

const pageLimit = 21

var (
	postUserID    = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	postAuthorID  = uuid.MustParse("00000000-0000-4000-8000-000000000002")
	postID        = uuid.MustParse("00000000-0000-4000-8000-000000000101")
	lockedPostID  = uuid.MustParse("00000000-0000-4000-8000-000000000102")
	postLikeID    = uuid.MustParse("00000000-0000-4000-8000-000000000201")
	postTime      = time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	postCreatedAt = time.Date(2026, time.September, 1, 11, 0, 0, 0, time.UTC)
	postAvatar    = "avatars/ivan.png"
	anonymous     = (*uuid.UUID)(nil)
	technicalErr  = errors.New("post storage unavailable")
	likeTxOptions = pgx.TxOptions{IsoLevel: pgx.ReadCommitted}

	createPostQuery     = testutils.SQLInOrder("INSERT INTO post (author_id, title, body, status, published_at)", "RETURNING id")
	getPostQuery        = testutils.SQLInOrder("FROM post p", `JOIN "user" u ON u.id = p.author_id AND u.status = 'active'`, "WHERE p.id = $2", "AND (p.status = 'published' OR p.author_id = $1)")
	updatePostQuery     = testutils.SQLInOrder("UPDATE post SET", "WHERE id = $1 AND author_id = $2 AND updated_at = $6")
	postVersionQuery    = testutils.SQLInOrder("SELECT author_id, updated_at FROM post WHERE id = $1")
	deletePostQuery     = testutils.SQLInOrder("DELETE FROM post WHERE id = $1 AND author_id = $2")
	availablePostQuery  = testutils.SQLInOrder("SELECT p.author_id", "WHERE p.id = $2", "AND p.status = 'published'", "FOR KEY SHARE OF p")
	insertLikeQuery     = testutils.SQLInOrder("INSERT INTO post_like (post_id, user_id)", "ON CONFLICT (post_id, user_id) DO NOTHING", "RETURNING id")
	insertNotifyQuery   = testutils.SQLInOrder("INSERT INTO notification (user_id, event_kind, post_like_id)", "'post_liked'")
	likesCountQuery     = testutils.SQLInOrder("SELECT likes_count FROM post WHERE id = $1")
	deleteLikeQuery     = testutils.SQLInOrder("DELETE FROM post_like WHERE post_id = $1 AND user_id = $2")
	postColumns         = []string{"id", "author_id", "title", "body", "status", "published_at", "likes_count", "created_at", "updated_at", "username", "nickname", "avatar_key", "locked", "liked_by_me"}
	previewColumns      = []string{"id", "author_id", "title", "body_preview", "published_at", "likes_count", "username", "nickname", "avatar_key", "locked", "liked_by_me"}
	postVersionColumns  = []string{"author_id", "updated_at"}
	authorIDColumn      = []string{"author_id"}
	likeIDColumn        = []string{"id"}
	likesCountColumn    = []string{"likes_count"}
	createdPostIDColumn = []string{"id"}
)

func previews() []*dto.PostPreview {
	return []*dto.PostPreview{
		{
			ID:                   postID,
			Author:               dto.AuthorResponse{ID: postAuthorID, Username: "ivan001", Nickname: "Иван", AvatarUrl: &postAvatar},
			Title:                "Мой первый пост",
			BodyPreview:          "Текст поста",
			PublishedOrUpdatedAt: &postTime,
			LikesCount:           12,
			LikedByUser:          true,
		},
		{
			ID:                   lockedPostID,
			Author:               dto.AuthorResponse{ID: postAuthorID, Username: "ivan001", Nickname: "Иван"},
			Title:                "Пост для подписчиков",
			PublishedOrUpdatedAt: &postTime,
			Locked:               true,
		},
	}
}

type listMethod struct {
	name  string
	query string
	args  []any
	call  func(r *PostRepository) ([]*dto.PostPreview, error)
}

func listMethods() []listMethod {
	var methods []listMethod
	methods = append(methods, timed("GetMyNewPosts",
		testutils.SQLInOrder("WHERE p.status = 'published'", "(p.published_at, p.id) < ($2::timestamptz, $3::uuid)",
			"AND p.author_id = $1", "ORDER BY p.published_at DESC, p.id DESC", "LIMIT $4"),
		func(r *PostRepository) func(context.Context, *uuid.UUID, *dto.TimedPostsCursor, int) ([]*dto.PostPreview, error) {
			return r.GetMyNewPosts
		})...)
	methods = append(methods, timed("GetMyOldPosts",
		testutils.SQLInOrder("WHERE p.status = 'published'", "(p.published_at, p.id) > ($2::timestamptz, $3::uuid)",
			"AND p.author_id = $1", "ORDER BY p.published_at, p.id", "LIMIT $4"),
		func(r *PostRepository) func(context.Context, *uuid.UUID, *dto.TimedPostsCursor, int) ([]*dto.PostPreview, error) {
			return r.GetMyOldPosts
		})...)
	methods = append(methods, timed("GetMyNewDrafts",
		testutils.SQLInOrder("p.updated_at", "WHERE p.status = 'draft'", "(p.updated_at, p.id) < ($2::timestamptz, $3::uuid)",
			"AND p.author_id = $1", "ORDER BY p.updated_at DESC, p.id DESC", "LIMIT $4"),
		func(r *PostRepository) func(context.Context, *uuid.UUID, *dto.TimedPostsCursor, int) ([]*dto.PostPreview, error) {
			return r.GetMyNewDrafts
		})...)
	methods = append(methods, timed("GetMyOldDrafts",
		testutils.SQLInOrder("p.updated_at", "WHERE p.status = 'draft'", "(p.updated_at, p.id) > ($2::timestamptz, $3::uuid)",
			"AND p.author_id = $1", "ORDER BY p.updated_at, p.id", "LIMIT $4"),
		func(r *PostRepository) func(context.Context, *uuid.UUID, *dto.TimedPostsCursor, int) ([]*dto.PostPreview, error) {
			return r.GetMyOldDrafts
		})...)
	methods = append(methods, timed("GetNewPosts",
		testutils.SQLInOrder("WITH page AS", "WHERE p.status = 'published' AND u.status = 'active'",
			"(p.published_at, p.id) < ($2::timestamptz, $3::uuid)", "ORDER BY p.published_at DESC, p.id DESC", "LIMIT $4",
			"LEFT JOIN post_access pa", "ORDER BY p.published_at DESC, p.id DESC"),
		func(r *PostRepository) func(context.Context, *uuid.UUID, *dto.TimedPostsCursor, int) ([]*dto.PostPreview, error) {
			return r.GetNewPosts
		})...)
	methods = append(methods, timed("GetOldPosts",
		testutils.SQLInOrder("WITH page AS", "WHERE p.status = 'published' AND u.status = 'active'",
			"(p.published_at, p.id) > ($2::timestamptz, $3::uuid)", "ORDER BY p.published_at, p.id", "LIMIT $4",
			"LEFT JOIN post_access pa", "ORDER BY p.published_at, p.id"),
		func(r *PostRepository) func(context.Context, *uuid.UUID, *dto.TimedPostsCursor, int) ([]*dto.PostPreview, error) {
			return r.GetOldPosts
		})...)
	methods = append(methods, popular("GetMyPopularPosts",
		testutils.SQLInOrder("WHERE p.status = 'published'",
			"(p.likes_count, p.published_at, p.id) < ($2::bigint, $3::timestamptz, $4::uuid)",
			"AND p.author_id = $1", "ORDER BY p.likes_count DESC, p.published_at DESC, p.id DESC", "LIMIT $5"),
		func(r *PostRepository) func(context.Context, *uuid.UUID, *dto.PopularPostsCursor, int) ([]*dto.PostPreview, error) {
			return r.GetMyPopularPosts
		})...)
	methods = append(methods, popular("GetPopularPosts",
		testutils.SQLInOrder("WITH page AS", "WHERE p.status = 'published' AND u.status = 'active'",
			"(p.likes_count, p.published_at, p.id) < ($2::bigint, $3::timestamptz, $4::uuid)",
			"ORDER BY p.likes_count DESC, p.published_at DESC, p.id DESC", "LIMIT $5", "LEFT JOIN post_access pa"),
		func(r *PostRepository) func(context.Context, *uuid.UUID, *dto.PopularPostsCursor, int) ([]*dto.PostPreview, error) {
			return r.GetPopularPosts
		})...)
	methods = append(methods, userTimed("GetUserNewPosts",
		testutils.SQLInOrder("WITH page AS", "WHERE p.status = 'published'", "AND u.username = $2",
			"(p.published_at, p.id) < ($3::timestamptz, $4::uuid)", "ORDER BY p.published_at DESC, p.id DESC", "LIMIT $5",
			"LEFT JOIN post_access pa"),
		func(r *PostRepository) func(context.Context, *uuid.UUID, string, *dto.TimedPostsCursor, int) ([]*dto.PostPreview, error) {
			return r.GetUserNewPosts
		})...)
	methods = append(methods, userTimed("GetUserOldPosts",
		testutils.SQLInOrder("WITH page AS", "WHERE p.status = 'published'", "AND u.username = $2",
			"(p.published_at, p.id) > ($3::timestamptz, $4::uuid)", "ORDER BY p.published_at ASC, p.id ASC", "LIMIT $5",
			"LEFT JOIN post_access pa"),
		func(r *PostRepository) func(context.Context, *uuid.UUID, string, *dto.TimedPostsCursor, int) ([]*dto.PostPreview, error) {
			return r.GetUserOldPosts
		})...)
	methods = append(methods, userPopular("GetUserPopularPosts",
		testutils.SQLInOrder("WITH page AS", "WHERE p.status = 'published'", "AND u.username = $2",
			"(p.likes_count, p.published_at, p.id) < ($3::bigint, $4::timestamptz, $5::uuid)",
			"ORDER BY p.likes_count DESC, p.published_at DESC, p.id DESC", "LIMIT $6", "LEFT JOIN post_access pa"),
		func(r *PostRepository) func(context.Context, *uuid.UUID, string, *dto.PopularPostsCursor, int) ([]*dto.PostPreview, error) {
			return r.GetUserPopularPosts
		})...)
	return methods
}

func TestPostRepository_CreatePost_Success(t *testing.T) {
	t.Parallel()

	post := models.Post{Title: "Мой первый пост", Body: "Текст поста", Status: "published"}
	repo, pool := newRepository(t)
	pool.ExpectQuery(createPostQuery).WithArgs(&postUserID, post.Title, post.Body, post.Status).
		WillReturnRows(pgxmock.NewRows(createdPostIDColumn).AddRow(postID))

	got, err := repo.CreatePost(context.Background(), &postUserID, post)

	require.NoError(t, err)
	require.Equal(t, postID, got)
}

func TestPostRepository_CreatePost_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		dbErr   error
		wantErr error
	}{
		{name: "unknown author", dbErr: testutils.PgError("23503"), wantErr: models.ErrInvalidAuthorID},
		{name: "check constraint", dbErr: testutils.PgError("23514"), wantErr: models.ErrPostValidation},
		{name: "storage failure", dbErr: technicalErr, wantErr: technicalErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			post := models.Post{Title: "Мой первый пост", Body: "Текст поста", Status: "draft"}
			repo, pool := newRepository(t)
			pool.ExpectQuery(createPostQuery).WithArgs(&postUserID, post.Title, post.Body, post.Status).WillReturnError(tt.dbErr)

			got, err := repo.CreatePost(context.Background(), &postUserID, post)

			require.Equal(t, uuid.Nil, got)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestPostRepository_GetPost(t *testing.T) {
	t.Parallel()

	post := &dto.PostInfo{
		ID:          postID,
		Author:      dto.AuthorResponse{ID: postAuthorID, Username: "ivan001", Nickname: "Иван", AvatarUrl: &postAvatar},
		Title:       "Мой первый пост",
		Status:      "published",
		PublishedAt: &postTime,
		CreatedAt:   postCreatedAt,
		UpdatedAt:   postTime,
		Locked:      true,
		LikesCount:  3,
	}
	postRow := []any{
		post.ID, post.Author.ID, post.Title, post.Body, post.Status, post.PublishedAt, post.LikesCount,
		post.CreatedAt, post.UpdatedAt, post.Author.Username, post.Author.Nickname, post.Author.AvatarUrl,
		post.Locked, post.LikedByUser,
	}
	tests := []struct {
		name    string
		userID  *uuid.UUID
		rows    *pgxmock.Rows
		dbErr   error
		want    *dto.PostInfo
		wantErr error
	}{
		{name: "authorized viewer", userID: &postUserID, rows: pgxmock.NewRows(postColumns).AddRow(postRow...), want: post},
		{name: "anonymous viewer", userID: anonymous, rows: pgxmock.NewRows(postColumns).AddRow(postRow...), want: post},
		{name: "hidden or unknown post", userID: &postUserID, dbErr: pgx.ErrNoRows, wantErr: models.ErrPostNotFound},
		{name: "storage failure", userID: &postUserID, dbErr: technicalErr, wantErr: technicalErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, pool := newRepository(t)
			query := pool.ExpectQuery(getPostQuery).WithArgs(tt.userID, postID)
			if tt.rows != nil {
				query.WillReturnRows(tt.rows)
			} else {
				query.WillReturnError(tt.dbErr)
			}

			got, err := repo.GetPost(context.Background(), tt.userID, postID)

			if tt.wantErr != nil {
				require.Nil(t, got)
				require.ErrorIs(t, err, tt.wantErr)
				if tt.wantErr != models.ErrPostNotFound {
					require.False(t, errors.Is(err, models.ErrPostNotFound))
				}

				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestPostRepository_ListPosts_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		posts []*dto.PostPreview
	}{
		{name: "open and locked posts", posts: previews()},
		{name: "empty page", posts: []*dto.PostPreview{}},
	}
	for _, m := range listMethods() {
		for _, tt := range tests {
			t.Run(m.name+"/"+tt.name, func(t *testing.T) {
				t.Parallel()

				repo, pool := newRepository(t)
				pool.ExpectQuery(m.query).WithArgs(m.args...).WillReturnRows(previewRows(tt.posts))

				got, err := m.call(repo)

				require.NoError(t, err)
				require.Equal(t, tt.posts, got)
			})
		}
	}
}

func TestPostRepository_ListPosts_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		expect  func(q *pgxmock.ExpectedQuery)
		wantErr error
	}{
		{
			name:    "query failure",
			expect:  func(q *pgxmock.ExpectedQuery) { q.WillReturnError(technicalErr) },
			wantErr: technicalErr,
		},
		{
			name:    "row iteration failure",
			expect:  func(q *pgxmock.ExpectedQuery) { q.WillReturnRows(previewRows(previews()).RowError(1, technicalErr)) },
			wantErr: technicalErr,
		},
		{
			name:   "unexpected columns",
			expect: func(q *pgxmock.ExpectedQuery) { q.WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(postID)) },
		},
	}
	for _, m := range listMethods() {
		for _, tt := range tests {
			t.Run(m.name+"/"+tt.name, func(t *testing.T) {
				t.Parallel()

				repo, pool := newRepository(t)
				tt.expect(pool.ExpectQuery(m.query).WithArgs(m.args...))

				got, err := m.call(repo)

				require.Nil(t, got)
				require.Error(t, err)
				if tt.wantErr != nil {
					require.ErrorIs(t, err, tt.wantErr)
				}
			})
		}
	}
}

func TestPostRepository_UpdatePost(t *testing.T) {
	t.Parallel()

	post := models.Post{ID: postID, Title: "Новый заголовок", Body: "Новый текст", Status: "published"}
	notUpdated := func(pool pgxmock.PgxPoolIface) {
		pool.ExpectExec(updatePostQuery).WithArgs(post.ID, &postUserID, post.Title, post.Body, post.Status, postTime).
			WillReturnResult(pgxmock.NewResult("UPDATE", 0))
	}
	currentVersion := func(pool pgxmock.PgxPoolIface, authorID uuid.UUID, updatedAt time.Time) {
		notUpdated(pool)
		pool.ExpectQuery(postVersionQuery).WithArgs(post.ID).
			WillReturnRows(pgxmock.NewRows(postVersionColumns).AddRow(authorID, updatedAt))
	}
	tests := []struct {
		name    string
		expect  func(pool pgxmock.PgxPoolIface)
		wantErr error
	}{
		{
			name: "actual version",
			expect: func(pool pgxmock.PgxPoolIface) {
				pool.ExpectExec(updatePostQuery).WithArgs(post.ID, &postUserID, post.Title, post.Body, post.Status, postTime).
					WillReturnResult(pgxmock.NewResult("UPDATE", 1))
			},
		},
		{
			name: "check constraint",
			expect: func(pool pgxmock.PgxPoolIface) {
				pool.ExpectExec(updatePostQuery).WithArgs(post.ID, &postUserID, post.Title, post.Body, post.Status, postTime).
					WillReturnError(testutils.PgError("23514"))
			},
			wantErr: models.ErrPostValidation,
		},
		{
			name: "update storage failure",
			expect: func(pool pgxmock.PgxPoolIface) {
				pool.ExpectExec(updatePostQuery).WithArgs(post.ID, &postUserID, post.Title, post.Body, post.Status, postTime).
					WillReturnError(technicalErr)
			},
			wantErr: technicalErr,
		},
		{
			name: "unknown post",
			expect: func(pool pgxmock.PgxPoolIface) {
				notUpdated(pool)
				pool.ExpectQuery(postVersionQuery).WithArgs(post.ID).WillReturnError(pgx.ErrNoRows)
			},
			wantErr: models.ErrPostNotFound,
		},
		{
			name:    "foreign post is hidden",
			expect:  func(pool pgxmock.PgxPoolIface) { currentVersion(pool, postAuthorID, postTime) },
			wantErr: models.ErrPostNotFound,
		},
		{
			name:    "stale version",
			expect:  func(pool pgxmock.PgxPoolIface) { currentVersion(pool, postUserID, postTime.Add(time.Second)) },
			wantErr: models.ErrPostConflict,
		},
		{
			name:    "version changed and restored concurrently",
			expect:  func(pool pgxmock.PgxPoolIface) { currentVersion(pool, postUserID, postTime) },
			wantErr: models.ErrPostConflict,
		},
		{
			name: "version lookup storage failure",
			expect: func(pool pgxmock.PgxPoolIface) {
				notUpdated(pool)
				pool.ExpectQuery(postVersionQuery).WithArgs(post.ID).WillReturnError(technicalErr)
			},
			wantErr: technicalErr,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, pool := newRepository(t)
			tt.expect(pool)

			err := repo.UpdatePost(context.Background(), &postUserID, post, postTime)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
		})
	}
}

func TestPostRepository_DeletePost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		deleted int64
		dbErr   error
		wantErr error
	}{
		{name: "own post", deleted: 1},
		{name: "foreign or unknown post", deleted: 0, wantErr: models.ErrPostNotFound},
		{name: "storage failure", dbErr: technicalErr, wantErr: technicalErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, pool := newRepository(t)
			exec := pool.ExpectExec(deletePostQuery).WithArgs(postID, &postUserID)
			if tt.dbErr != nil {
				exec.WillReturnError(tt.dbErr)
			} else {
				exec.WillReturnResult(pgxmock.NewResult("DELETE", tt.deleted))
			}

			err := repo.DeletePost(context.Background(), &postUserID, postID)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
		})
	}
}

func TestPostRepository_LikePost_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		expect func(pool pgxmock.PgxPoolIface)
	}{
		{
			name: "new like notifies author",
			expect: func(pool pgxmock.PgxPoolIface) {
				expectLikeInserted(pool, postAuthorID)
				pool.ExpectExec(insertNotifyQuery).WithArgs(postAuthorID, postLikeID).
					WillReturnResult(pgxmock.NewResult("INSERT", 1))
			},
		},
		{
			name:   "own post like does not notify",
			expect: func(pool pgxmock.PgxPoolIface) { expectLikeInserted(pool, postUserID) },
		},
		{
			name: "repeated like is idempotent",
			expect: func(pool pgxmock.PgxPoolIface) {
				expectAvailablePost(pool, postAuthorID)
				pool.ExpectQuery(insertLikeQuery).WithArgs(postID, &postUserID).WillReturnRows(pgxmock.NewRows(likeIDColumn))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, pool := newRepository(t)
			tt.expect(pool)
			expectLikesCount(pool, 13)
			testutils.ExpectTxCommitted(pool)

			got, err := repo.LikePost(context.Background(), &postUserID, postID)

			require.NoError(t, err)
			require.Equal(t, int64(13), got)
		})
	}
}

func TestPostRepository_LikePost_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		expect  func(pool pgxmock.PgxPoolIface)
		wantErr error
	}{
		{
			name: "unavailable post",
			expect: func(pool pgxmock.PgxPoolIface) {
				pool.ExpectBeginTx(likeTxOptions)
				pool.ExpectQuery(availablePostQuery).WithArgs(&postUserID, postID).WillReturnRows(pgxmock.NewRows(authorIDColumn))
			},
			wantErr: models.ErrPostNotFound,
		},
		{
			name: "post lookup storage failure",
			expect: func(pool pgxmock.PgxPoolIface) {
				pool.ExpectBeginTx(likeTxOptions)
				pool.ExpectQuery(availablePostQuery).WithArgs(&postUserID, postID).WillReturnError(technicalErr)
			},
			wantErr: technicalErr,
		},
		{
			name: "like insert storage failure",
			expect: func(pool pgxmock.PgxPoolIface) {
				expectAvailablePost(pool, postAuthorID)
				pool.ExpectQuery(insertLikeQuery).WithArgs(postID, &postUserID).WillReturnError(technicalErr)
			},
			wantErr: technicalErr,
		},
		{
			name: "notification storage failure",
			expect: func(pool pgxmock.PgxPoolIface) {
				expectLikeInserted(pool, postAuthorID)
				pool.ExpectExec(insertNotifyQuery).WithArgs(postAuthorID, postLikeID).WillReturnError(technicalErr)
			},
			wantErr: technicalErr,
		},
		{
			name: "likes count storage failure",
			expect: func(pool pgxmock.PgxPoolIface) {
				expectLikeInserted(pool, postUserID)
				pool.ExpectQuery(likesCountQuery).WithArgs(postID).WillReturnError(technicalErr)
			},
			wantErr: technicalErr,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, pool := newRepository(t)
			tt.expect(pool)
			testutils.ExpectTxRolledBack(pool)

			got, err := repo.LikePost(context.Background(), &postUserID, postID)

			require.Zero(t, got)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestPostRepository_LikePost_BeginFailure(t *testing.T) {
	t.Parallel()

	repo, pool := newRepository(t)
	pool.ExpectBeginTx(likeTxOptions).WillReturnError(technicalErr)

	got, err := repo.LikePost(context.Background(), &postUserID, postID)

	require.Zero(t, got)
	require.ErrorIs(t, err, technicalErr)
}

func TestPostRepository_DeletePostLike(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		expect    func(pool pgxmock.PgxPoolIface)
		wantLikes int64
		wantErr   error
	}{
		{
			name: "existing like is removed",
			expect: func(pool pgxmock.PgxPoolIface) {
				pool.ExpectExec(deleteLikeQuery).WithArgs(postID, &postUserID).WillReturnResult(pgxmock.NewResult("DELETE", 1))
				expectLikesCount(pool, 12)
			},
			wantLikes: 12,
		},
		{
			name: "missing like is idempotent",
			expect: func(pool pgxmock.PgxPoolIface) {
				pool.ExpectExec(deleteLikeQuery).WithArgs(postID, &postUserID).WillReturnResult(pgxmock.NewResult("DELETE", 0))
				expectLikesCount(pool, 12)
			},
			wantLikes: 12,
		},
		{
			name: "unknown post",
			expect: func(pool pgxmock.PgxPoolIface) {
				pool.ExpectExec(deleteLikeQuery).WithArgs(postID, &postUserID).WillReturnResult(pgxmock.NewResult("DELETE", 0))
				pool.ExpectQuery(likesCountQuery).WithArgs(postID).WillReturnRows(pgxmock.NewRows(likesCountColumn))
			},
			wantErr: models.ErrPostNotFound,
		},
		{
			name: "delete storage failure",
			expect: func(pool pgxmock.PgxPoolIface) {
				pool.ExpectExec(deleteLikeQuery).WithArgs(postID, &postUserID).WillReturnError(technicalErr)
			},
			wantErr: technicalErr,
		},
		{
			name: "likes count storage failure",
			expect: func(pool pgxmock.PgxPoolIface) {
				pool.ExpectExec(deleteLikeQuery).WithArgs(postID, &postUserID).WillReturnResult(pgxmock.NewResult("DELETE", 1))
				pool.ExpectQuery(likesCountQuery).WithArgs(postID).WillReturnError(technicalErr)
			},
			wantErr: technicalErr,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, pool := newRepository(t)
			tt.expect(pool)

			got, err := repo.DeletePostLike(context.Background(), &postUserID, postID)

			require.Equal(t, tt.wantLikes, got)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				if tt.wantErr != models.ErrPostNotFound {
					require.False(t, errors.Is(err, models.ErrPostNotFound))
				}

				return
			}
			require.NoError(t, err)
		})
	}
}
