package postrepository

import (
	"context"
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/testutils"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/dto"
	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v5"
)

func newRepository(t *testing.T) (*PostRepository, pgxmock.PgxPoolIface) {
	t.Helper()

	pool := testutils.SetupPgxMock(t)
	return &PostRepository{pool: pool, logger: testutils.SetupLogger()}, pool
}

func previewRows(posts []*dto.PostPreview) *pgxmock.Rows {
	rows := pgxmock.NewRows(previewColumns)
	for _, p := range posts {
		rows.AddRow(p.ID, p.Author.ID, p.Title, p.BodyPreview, p.PublishedOrUpdatedAt, p.LikesCount,
			p.Author.Username, p.Author.Nickname, p.Author.AvatarUrl, p.Locked, p.LikedByUser)
	}
	return rows
}

var (
	timedCursor   = &dto.TimedPostsCursor{PublishedOrUpdatedAt: postTime, PostID: postID}
	popularCursor = &dto.PopularPostsCursor{Likes: 12, PublishedAt: postTime, PostID: postID}
	noTime        = (*time.Time)(nil)
	noID          = (*uuid.UUID)(nil)
	noLikes       = (*int64)(nil)
)

type (
	timedFunc       func(*PostRepository) func(context.Context, *uuid.UUID, *dto.TimedPostsCursor, int) ([]*dto.PostPreview, error)
	popularFunc     func(*PostRepository) func(context.Context, *uuid.UUID, *dto.PopularPostsCursor, int) ([]*dto.PostPreview, error)
	userTimedFunc   func(*PostRepository) func(context.Context, *uuid.UUID, string, *dto.TimedPostsCursor, int) ([]*dto.PostPreview, error)
	userPopularFunc func(*PostRepository) func(context.Context, *uuid.UUID, string, *dto.PopularPostsCursor, int) ([]*dto.PostPreview, error)
)

func timed(name, query string, method timedFunc) []listMethod {
	ctx := context.Background()
	return []listMethod{
		{
			name: name + "/first page", query: query,
			args: []any{&postUserID, noTime, noID, pageLimit},
			call: func(r *PostRepository) ([]*dto.PostPreview, error) {
				return method(r)(ctx, &postUserID, nil, pageLimit)
			},
		},
		{
			name: name + "/next page", query: query,
			args: []any{&postUserID, &timedCursor.PublishedOrUpdatedAt, &timedCursor.PostID, pageLimit},
			call: func(r *PostRepository) ([]*dto.PostPreview, error) {
				return method(r)(ctx, &postUserID, timedCursor, pageLimit)
			},
		},
	}
}

func popular(name, query string, method popularFunc) []listMethod {
	ctx := context.Background()
	return []listMethod{
		{
			name: name + "/first page", query: query,
			args: []any{&postUserID, noLikes, noTime, noID, pageLimit},
			call: func(r *PostRepository) ([]*dto.PostPreview, error) {
				return method(r)(ctx, &postUserID, nil, pageLimit)
			},
		},
		{
			name: name + "/next page", query: query,
			args: []any{&postUserID, &popularCursor.Likes, &popularCursor.PublishedAt, &popularCursor.PostID, pageLimit},
			call: func(r *PostRepository) ([]*dto.PostPreview, error) {
				return method(r)(ctx, &postUserID, popularCursor, pageLimit)
			},
		},
	}
}

func userTimed(name, query string, method userTimedFunc) []listMethod {
	ctx := context.Background()
	return []listMethod{
		{
			name: name + "/first page", query: query,
			args: []any{&postUserID, "ivan001", noTime, noID, pageLimit},
			call: func(r *PostRepository) ([]*dto.PostPreview, error) {
				return method(r)(ctx, &postUserID, "ivan001", nil, pageLimit)
			},
		},
		{
			name: name + "/next page", query: query,
			args: []any{&postUserID, "ivan001", &timedCursor.PublishedOrUpdatedAt, &timedCursor.PostID, pageLimit},
			call: func(r *PostRepository) ([]*dto.PostPreview, error) {
				return method(r)(ctx, &postUserID, "ivan001", timedCursor, pageLimit)
			},
		},
	}
}

func userPopular(name, query string, method userPopularFunc) []listMethod {
	ctx := context.Background()
	return []listMethod{
		{
			name: name + "/first page", query: query,
			args: []any{&postUserID, "ivan001", noLikes, noTime, noID, pageLimit},
			call: func(r *PostRepository) ([]*dto.PostPreview, error) {
				return method(r)(ctx, &postUserID, "ivan001", nil, pageLimit)
			},
		},
		{
			name: name + "/next page", query: query,
			args: []any{&postUserID, "ivan001", &popularCursor.Likes, &popularCursor.PublishedAt, &popularCursor.PostID, pageLimit},
			call: func(r *PostRepository) ([]*dto.PostPreview, error) {
				return method(r)(ctx, &postUserID, "ivan001", popularCursor, pageLimit)
			},
		},
	}
}

func expectAvailablePost(pool pgxmock.PgxPoolIface, authorID uuid.UUID) {
	pool.ExpectBeginTx(likeTxOptions)
	pool.ExpectQuery(availablePostQuery).WithArgs(&postUserID, postID).
		WillReturnRows(pgxmock.NewRows(authorIDColumn).AddRow(authorID))
}

func expectLikeInserted(pool pgxmock.PgxPoolIface, authorID uuid.UUID) {
	expectAvailablePost(pool, authorID)
	pool.ExpectQuery(insertLikeQuery).WithArgs(postID, &postUserID).
		WillReturnRows(pgxmock.NewRows(likeIDColumn).AddRow(postLikeID))
}

func expectLikesCount(pool pgxmock.PgxPoolIface, likes int64) {
	pool.ExpectQuery(likesCountQuery).WithArgs(postID).
		WillReturnRows(pgxmock.NewRows(likesCountColumn).AddRow(likes))
}
