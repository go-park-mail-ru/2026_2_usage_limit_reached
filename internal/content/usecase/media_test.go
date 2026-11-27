package usecase_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"io"
	"testing"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/models"
	postusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/usecase/mock_content_usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/testutils"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/sanitizer"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var testAttachmentID = uuid.MustParse("00000000-0000-4000-8000-000000000301")

type testLimits struct {
	image, video, audio, file  int64
	maxAttachments, maxPending int
}

func (l testLimits) MaxImageSize() int64     { return l.image }
func (l testLimits) MaxVideoSize() int64     { return l.video }
func (l testLimits) MaxAudioSize() int64     { return l.audio }
func (l testLimits) MaxFileSize() int64      { return l.file }
func (l testLimits) MaxPostAttachments() int { return l.maxAttachments }
func (l testLimits) MaxPendingMedia() int    { return l.maxPending }

func defaultTestLimits() testLimits {
	return testLimits{
		image:          1 << 20,
		video:          1 << 20,
		audio:          1 << 20,
		file:           1 << 20,
		maxAttachments: 20,
		maxPending:     20,
	}
}

func newMediaTestUsecase(
	t *testing.T,
	repo postusecase.Repository,
	media postusecase.MediaStorage,
	limits postusecase.AttachmentLimiter,
) *postusecase.ContentUsecase {
	t.Helper()

	s, err := sanitizer.NewSanitizer(sanitizer.DefaultConfig())
	require.NoError(t, err)

	return postusecase.New(repo, nil, s, testutils.SetupValidator(), "/media", limits, media, testutils.SetupLogger())
}

func pngFile(t *testing.T) []byte {
	t.Helper()

	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 1, 1))))

	return buf.Bytes()
}

func TestContentUsecase_UploadAttachment_Success(t *testing.T) {
	t.Parallel()

	data := pngFile(t)
	repo := mock_content_usecase.NewMockRepository(t)
	media := mock_content_usecase.NewMockMediaStorage(t)

	repo.EXPECT().CountPendingMedia(mock.Anything, postUserID).Return(int64(0), nil).Once()
	media.EXPECT().
		Put(mock.Anything, mock.Anything, mock.Anything, int64(len(data)), "image/png").
		Return(nil).
		Once()
	repo.EXPECT().
		CreateMedia(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, m models.Media) (models.Media, error) {
			m.ID = testAttachmentID
			return m, nil
		}).
		Once()

	uc := newMediaTestUsecase(t, repo, media, defaultTestLimits())

	got, err := uc.UploadAttachment(context.Background(), &postUserID, bytes.NewReader(data), int64(len(data)), "cat.png")

	require.NoError(t, err)
	require.Equal(t, testAttachmentID, got.ID)
	require.Equal(t, dto.AttachmentTypeImage, got.MediaType)
	require.Equal(t, "image/png", got.MimeType)
	require.Equal(t, "cat.png", got.Name)
	require.Equal(t, int64(len(data)), got.SizeBytes)
	require.Equal(t, "/attachments/"+testAttachmentID.String(), got.URL)
}

func TestContentUsecase_UploadAttachment_Errors(t *testing.T) {
	t.Parallel()

	technicalErr := errors.New("media storage unavailable")

	t.Run("anonymous", func(t *testing.T) {
		t.Parallel()

		uc := newMediaTestUsecase(t, mock_content_usecase.NewMockRepository(t), nil, defaultTestLimits())

		_, err := uc.UploadAttachment(context.Background(), anonymous, bytes.NewReader(pngFile(t)), 8, "cat.png")

		require.ErrorIs(t, err, postusecase.ErrUnauthorized)
	})

	t.Run("empty file", func(t *testing.T) {
		t.Parallel()

		uc := newMediaTestUsecase(t, mock_content_usecase.NewMockRepository(t), nil, defaultTestLimits())

		_, err := uc.UploadAttachment(context.Background(), &postUserID, bytes.NewReader(nil), 0, "cat.png")

		require.ErrorIs(t, err, postusecase.ErrUnsupportedMediaType)
	})

	t.Run("empty filename", func(t *testing.T) {
		t.Parallel()

		uc := newMediaTestUsecase(t, mock_content_usecase.NewMockRepository(t), nil, defaultTestLimits())
		data := pngFile(t)

		_, err := uc.UploadAttachment(context.Background(), &postUserID, bytes.NewReader(data), int64(len(data)), "  ")

		require.ErrorIs(t, err, postusecase.ErrValidation)
	})

	t.Run("unsupported content", func(t *testing.T) {
		t.Parallel()

		repo := mock_content_usecase.NewMockRepository(t)
		repo.EXPECT().CountPendingMedia(mock.Anything, postUserID).Return(int64(0), nil).Once()

		uc := newMediaTestUsecase(t, repo, mock_content_usecase.NewMockMediaStorage(t), defaultTestLimits())
		data := []byte("просто текст, а не файл")

		_, err := uc.UploadAttachment(context.Background(), &postUserID, bytes.NewReader(data), int64(len(data)), "note.txt")

		require.ErrorIs(t, err, postusecase.ErrUnsupportedMediaType)
	})

	t.Run("too many pending files", func(t *testing.T) {
		t.Parallel()

		limits := defaultTestLimits()
		limits.maxPending = 1

		repo := mock_content_usecase.NewMockRepository(t)
		repo.EXPECT().CountPendingMedia(mock.Anything, postUserID).Return(int64(1), nil).Once()

		media := mock_content_usecase.NewMockMediaStorage(t)
		uc := newMediaTestUsecase(t, repo, media, limits)
		data := pngFile(t)

		_, err := uc.UploadAttachment(context.Background(), &postUserID, bytes.NewReader(data), int64(len(data)), "cat.png")

		require.ErrorIs(t, err, postusecase.ErrValidation)
	})

	t.Run("file is too large", func(t *testing.T) {
		t.Parallel()

		limits := defaultTestLimits()
		limits.image = 1

		repo := mock_content_usecase.NewMockRepository(t)
		repo.EXPECT().CountPendingMedia(mock.Anything, postUserID).Return(int64(0), nil).Once()

		media := mock_content_usecase.NewMockMediaStorage(t)
		uc := newMediaTestUsecase(t, repo, media, limits)
		data := pngFile(t)

		_, err := uc.UploadAttachment(context.Background(), &postUserID, bytes.NewReader(data), int64(len(data)), "cat.png")

		require.ErrorIs(t, err, postusecase.ErrMediaTooLarge)
	})

	t.Run("registry failure removes uploaded object", func(t *testing.T) {
		t.Parallel()

		repo := mock_content_usecase.NewMockRepository(t)
		repo.EXPECT().CountPendingMedia(mock.Anything, postUserID).Return(int64(0), nil).Once()
		repo.EXPECT().CreateMedia(mock.Anything, mock.Anything).Return(models.Media{}, technicalErr).Once()

		media := mock_content_usecase.NewMockMediaStorage(t)
		media.EXPECT().Put(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		media.EXPECT().Delete(mock.Anything, mock.Anything).Return(nil).Once()

		uc := newMediaTestUsecase(t, repo, media, defaultTestLimits())
		data := pngFile(t)

		_, err := uc.UploadAttachment(context.Background(), &postUserID, bytes.NewReader(data), int64(len(data)), "cat.png")

		require.ErrorIs(t, err, technicalErr)
	})
}

func TestResolveRange(t *testing.T) {
	t.Parallel()

	const size = 100

	tests := []struct {
		name       string
		rng        *postusecase.AttachmentRange
		wantOffset int64
		wantLength int64
	}{
		{name: "whole file", rng: nil, wantOffset: 0, wantLength: -1},
		{name: "open ended", rng: &postusecase.AttachmentRange{Start: 10, End: -1}, wantOffset: 10, wantLength: -1},
		{name: "bounded", rng: &postusecase.AttachmentRange{Start: 10, End: 19}, wantOffset: 10, wantLength: 10},
		{name: "end beyond size", rng: &postusecase.AttachmentRange{Start: 90, End: 200}, wantOffset: 90, wantLength: -1},
		{name: "suffix", rng: &postusecase.AttachmentRange{Start: -10, End: -1}, wantOffset: 90, wantLength: -1},
		{name: "suffix larger than file", rng: &postusecase.AttachmentRange{Start: -200, End: -1}, wantOffset: 0, wantLength: -1},
		{name: "start beyond size", rng: &postusecase.AttachmentRange{Start: 100, End: -1}, wantOffset: -1, wantLength: 0},
		{name: "inverted bounds", rng: &postusecase.AttachmentRange{Start: 20, End: 10}, wantOffset: -1, wantLength: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			offset, length := postusecase.ResolveRange(tt.rng, size)

			require.Equal(t, tt.wantOffset, offset)
			require.Equal(t, tt.wantLength, length)
		})
	}
}

func TestContentUsecase_OpenAttachment(t *testing.T) {
	t.Parallel()

	normalizer := func(t *testing.T) *mock_content_usecase.MockRepository {
		t.Helper()

		repo := mock_content_usecase.NewMockRepository(t)
		repo.EXPECT().
			GetMediaForAccess(mock.Anything, &postUserID, testAttachmentID).
			Return(models.Media{
				ID:        testAttachmentID,
				AuthorID:  postUserID,
				MediaType: "image",
				MimeType:  "image/png",
				MediaKey:  "posts/a/b.png",
				Filename:  "cat.png",
				SizeBytes: 100,
			}, nil).
			Once()

		return repo
	}

	t.Run("full file", func(t *testing.T) {
		t.Parallel()

		body := io.NopCloser(bytes.NewReader([]byte("data")))
		media := mock_content_usecase.NewMockMediaStorage(t)
		media.EXPECT().Open(mock.Anything, "posts/a/b.png", int64(0), int64(-1)).Return(body, nil).Once()

		uc := newMediaTestUsecase(t, normalizer(t), media, defaultTestLimits())

		got, reader, err := uc.OpenAttachment(context.Background(), &postUserID, testAttachmentID, nil)

		require.NoError(t, err)
		require.NotNil(t, reader)
		require.Equal(t, "image/png", got.MimeType)
		require.Equal(t, "cat.png", got.Name)
		require.Equal(t, int64(100), got.SizeBytes)
	})

	t.Run("range inside file", func(t *testing.T) {
		t.Parallel()

		media := mock_content_usecase.NewMockMediaStorage(t)
		media.EXPECT().Open(mock.Anything, "posts/a/b.png", int64(10), int64(10)).
			Return(io.NopCloser(bytes.NewReader([]byte("data"))), nil).
			Once()

		uc := newMediaTestUsecase(t, normalizer(t), media, defaultTestLimits())

		_, _, err := uc.OpenAttachment(context.Background(), &postUserID, testAttachmentID, &postusecase.AttachmentRange{Start: 10, End: 19})

		require.NoError(t, err)
	})

	t.Run("range outside file", func(t *testing.T) {
		t.Parallel()

		uc := newMediaTestUsecase(t, normalizer(t), mock_content_usecase.NewMockMediaStorage(t), defaultTestLimits())

		_, _, err := uc.OpenAttachment(context.Background(), &postUserID, testAttachmentID, &postusecase.AttachmentRange{Start: 500, End: -1})

		require.ErrorIs(t, err, postusecase.ErrRangeNotSatisfiable)
	})

	t.Run("no access", func(t *testing.T) {
		t.Parallel()

		repo := mock_content_usecase.NewMockRepository(t)
		repo.EXPECT().
			GetMediaForAccess(mock.Anything, anonymous, testAttachmentID).
			Return(models.Media{}, models.ErrMediaNotFound).
			Once()

		uc := newMediaTestUsecase(t, repo, mock_content_usecase.NewMockMediaStorage(t), defaultTestLimits())

		_, _, err := uc.OpenAttachment(context.Background(), anonymous, testAttachmentID, nil)

		require.ErrorIs(t, err, postusecase.ErrNotFound)
	})
}

func TestContentUsecase_DeletePost_RemovesDetachedMedia(t *testing.T) {
	t.Parallel()

	repo := mock_content_usecase.NewMockRepository(t)
	repo.EXPECT().DeletePost(mock.Anything, &postUserID, postID).
		Return([]string{"posts/a/one.png", "posts/a/two.pdf"}, nil).
		Once()

	media := mock_content_usecase.NewMockMediaStorage(t)
	media.EXPECT().Delete(mock.Anything, "posts/a/one.png").Return(nil).Once()
	media.EXPECT().Delete(mock.Anything, "posts/a/two.pdf").Return(nil).Once()

	uc := newMediaTestUsecase(t, repo, media, defaultTestLimits())

	require.NoError(t, uc.DeletePost(context.Background(), &postUserID, postID))
}

func TestContentUsecase_CreatePost_ExtractsAttachments(t *testing.T) {
	t.Parallel()

	body := `<p>Текст</p><figure data-attachment-id="` + testAttachmentID.String() + `"></figure>`

	repo := mock_content_usecase.NewMockRepository(t)
	repo.EXPECT().
		CreatePost(mock.Anything, &postUserID, mock.MatchedBy(func(p models.Post) bool {
			return len(p.Attachments) == 1 && p.Attachments[0] == testAttachmentID
		})).
		Return(postID, nil).
		Once()

	uc := newMediaTestUsecase(t, repo, nil, defaultTestLimits())

	got, err := uc.CreatePost(context.Background(), &postUserID, dto.PostRequest{
		Title:  "Пост",
		Body:   body,
		Status: "draft",
	})

	require.NoError(t, err)
	require.Equal(t, postID, got)
}

func TestContentUsecase_CreatePost_TooManyAttachments(t *testing.T) {
	t.Parallel()

	limits := defaultTestLimits()
	limits.maxAttachments = 1

	body := `<p>Текст</p><figure data-attachment-id="` + testAttachmentID.String() + `"></figure><figure data-attachment-id="` + testAttachmentID.String() + `"></figure>`

	uc := newMediaTestUsecase(t, mock_content_usecase.NewMockRepository(t), nil, limits)

	_, err := uc.CreatePost(context.Background(), &postUserID, dto.PostRequest{
		Title:  "Пост",
		Body:   body,
		Status: "draft",
	})

	require.ErrorIs(t, err, postusecase.ErrValidation)
}
