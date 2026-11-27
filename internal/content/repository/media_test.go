package repository

import (
	"context"
	"testing"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/testutils"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v5"
	"github.com/stretchr/testify/require"
)

var (
	testMediaID  = uuid.MustParse("00000000-0000-4000-8000-000000000301")
	testMediaKey = "posts/00000000-0000-4000-8000-000000000001/00000000-0000-4000-8000-000000000301.png"
)

func TestRepository_GetPost_Attachments(t *testing.T) {
	t.Parallel()

	post := &dto.PostInfo{
		ID:          postID,
		Author:      dto.AuthorResponse{ID: postAuthorID},
		Title:       "Мой первый пост",
		Status:      "published",
		PublishedAt: &postTime,
		CreatedAt:   postCreatedAt,
		UpdatedAt:   postTime,
		LikesCount:  3,
		Attachments: []*dto.Attachment{
			{
				ID:        testMediaID,
				MediaType: dto.AttachmentTypeImage,
				MimeType:  "image/png",
				Name:      "cat.png",
				SizeBytes: 12345,
			},
		},
	}

	postRow := []any{
		post.ID,
		post.Author.ID,
		post.Title,
		post.Body,
		post.Status,
		post.PublishedAt,
		post.LikesCount,
		post.CreatedAt,
		post.UpdatedAt,
		post.Locked,
		post.LikedByUser,
	}

	repo, pool := newRepository(t)
	pool.ExpectQuery(getPostQuery).WithArgs(&postUserID, postID).
		WillReturnRows(pgxmock.NewRows(postColumns).AddRow(postRow...))
	pool.ExpectQuery(postAttachmentsMatch).WithArgs(postID).
		WillReturnRows(pgxmock.NewRows(attachmentColumns).
			AddRow(testMediaID, string(dto.AttachmentTypeImage), "image/png", "cat.png", int64(12345)))

	got, err := repo.GetPost(context.Background(), &postUserID, postID)

	require.NoError(t, err)
	require.Equal(t, post, got)
}

func TestRepository_GetPost_LockedSkipsAttachments(t *testing.T) {
	t.Parallel()

	post := &dto.PostInfo{
		ID:          postID,
		Author:      dto.AuthorResponse{ID: postAuthorID},
		Title:       "Пост для подписчиков",
		Status:      "published",
		PublishedAt: &postTime,
		CreatedAt:   postCreatedAt,
		UpdatedAt:   postTime,
		Locked:      true,
		Attachments: []*dto.Attachment{},
	}

	postRow := []any{
		post.ID,
		post.Author.ID,
		post.Title,
		post.Body,
		post.Status,
		post.PublishedAt,
		post.LikesCount,
		post.CreatedAt,
		post.UpdatedAt,
		post.Locked,
		post.LikedByUser,
	}

	repo, pool := newRepository(t)
	pool.ExpectQuery(getPostQuery).WithArgs(&postUserID, postID).
		WillReturnRows(pgxmock.NewRows(postColumns).AddRow(postRow...))

	got, err := repo.GetPost(context.Background(), &postUserID, postID)

	require.NoError(t, err)
	require.Equal(t, post, got)
}

func TestRepository_CreateMedia(t *testing.T) {
	t.Parallel()

	media := models.Media{
		AuthorID:  postUserID,
		MediaType: "image",
		MimeType:  "image/png",
		MediaKey:  testMediaKey,
		Filename:  "cat.png",
		SizeBytes: 12345,
	}

	repo, pool := newRepository(t)
	pool.ExpectQuery(createMediaQuery).
		WithArgs(media.AuthorID, media.MediaType, media.MimeType, media.MediaKey, media.Filename, media.SizeBytes).
		WillReturnRows(pgxmock.NewRows([]string{"id", "created_at", "updated_at"}).
			AddRow(testMediaID, postCreatedAt, postTime))

	got, err := repo.CreateMedia(context.Background(), media)

	require.NoError(t, err)
	require.Equal(t, testMediaID, got.ID)
	require.Equal(t, media.MediaKey, got.MediaKey)
	require.Equal(t, postCreatedAt, got.CreatedAt)
	require.Equal(t, postTime, got.UpdatedAt)
}

func TestRepository_CreateMedia_UnknownAuthor(t *testing.T) {
	t.Parallel()

	repo, pool := newRepository(t)
	pool.ExpectQuery(createMediaQuery).
		WithArgs(postUserID, "image", "image/png", testMediaKey, "cat.png", int64(12345)).
		WillReturnError(testutils.PgError("23503"))

	_, err := repo.CreateMedia(context.Background(), models.Media{
		AuthorID:  postUserID,
		MediaType: "image",
		MimeType:  "image/png",
		MediaKey:  testMediaKey,
		Filename:  "cat.png",
		SizeBytes: 12345,
	})

	require.ErrorIs(t, err, models.ErrInvalidAuthorID)
}

func TestRepository_CountPendingMedia(t *testing.T) {
	t.Parallel()

	repo, pool := newRepository(t)
	pool.ExpectQuery(countPendingMediaQuery).WithArgs(postUserID).
		WillReturnRows(pgxmock.NewRows(countColumn).AddRow(int64(2)))

	got, err := repo.CountPendingMedia(context.Background(), postUserID)

	require.NoError(t, err)
	require.Equal(t, int64(2), got)
}

func TestRepository_GetMediaForAccess(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		userID  *uuid.UUID
		row     []any
		dbErr   error
		wantErr error
	}{
		{
			name:   "author of pending media",
			userID: &postUserID,
			row:    []any{testMediaID, postUserID, "image", "image/png", testMediaKey, "cat.png", int64(12345)},
		},
		{
			name:    "anonymous without access",
			userID:  anonymous,
			dbErr:   pgx.ErrNoRows,
			wantErr: models.ErrMediaNotFound,
		},
		{
			name:    "storage failure",
			userID:  &postUserID,
			dbErr:   technicalErr,
			wantErr: technicalErr,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo, pool := newRepository(t)
			query := pool.ExpectQuery(getMediaQuery).WithArgs(testMediaID, tt.userID)
			if tt.row != nil {
				query.WillReturnRows(pgxmock.NewRows(mediaColumns).AddRow(tt.row...))
			} else {
				query.WillReturnError(tt.dbErr)
			}

			got, err := repo.GetMediaForAccess(context.Background(), tt.userID, testMediaID)

			if tt.wantErr != nil {
				require.Zero(t, got)
				require.ErrorIs(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			require.Equal(t, testMediaID, got.ID)
			require.Equal(t, testMediaKey, got.MediaKey)
			require.Equal(t, int64(12345), got.SizeBytes)
		})
	}
}
