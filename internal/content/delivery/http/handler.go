package handlers

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/dto"
	contentusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
	"github.com/google/uuid"
)

type ContentUsecase interface {
	CreatePost(ctx context.Context, userID *uuid.UUID, p dto.PostRequest) (uuid.UUID, error)
	GetPost(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) (*dto.PostInfo, error)
	GetMyNewPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) (*dto.TimedPostsPage, error)
	GetMyOldPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) (*dto.TimedPostsPage, error)
	GetMyPopularPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.PopularPostsCursor, limit int) (*dto.PopularPostsPage, error)
	GetMyNewDrafts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) (*dto.TimedPostsPage, error)
	GetMyOldDrafts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) (*dto.TimedPostsPage, error)
	GetNewPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) (*dto.TimedPostsPage, error)
	GetOldPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) (*dto.TimedPostsPage, error)
	GetPopularPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.PopularPostsCursor, limit int) (*dto.PopularPostsPage, error)
	GetUserNewPosts(ctx context.Context, userID *uuid.UUID, authorUsername string, cursor *dto.TimedPostsCursor, limit int) (*dto.TimedPostsPage, error)
	GetUserOldPosts(ctx context.Context, userID *uuid.UUID, authorUsername string, cursor *dto.TimedPostsCursor, limit int) (*dto.TimedPostsPage, error)
	GetUserPopularPosts(ctx context.Context, userID *uuid.UUID, authorUsername string, cursor *dto.PopularPostsCursor, limit int) (*dto.PopularPostsPage, error)
	UpdatePost(ctx context.Context, userID *uuid.UUID, postID uuid.UUID, p dto.PostRequest, lastUpdatedAt time.Time) error
	DeletePost(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) error

	UploadAttachment(ctx context.Context, userID *uuid.UUID, file io.ReadSeeker, size int64, filename string) (*dto.Attachment, error)
	OpenAttachment(ctx context.Context, userID *uuid.UUID, id uuid.UUID, rng *contentusecase.AttachmentRange) (*dto.Attachment, io.ReadCloser, error)

	LikePost(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) (int64, error)
	DeletePostLike(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) (int64, error)
}

func handleError(ctx context.Context, logger *slog.Logger, w http.ResponseWriter, action string, err error) {
	logError(ctx, logger, action, err)
	switch {
	case errors.Is(err, contentusecase.ErrConflict):
		response.ErrorConflict(w)
	case errors.Is(err, contentusecase.ErrForbidden):
		response.ErrorForbidden(w)
	case errors.Is(err, contentusecase.ErrUnauthorized):
		response.ErrorUnauthorized(w)
	case errors.Is(err, contentusecase.ErrNotFound):
		response.ErrorNotFound(w)
	case errors.Is(err, contentusecase.ErrMediaTooLarge):
		response.ErrorPayloadTooLarge(w)
	case errors.Is(err, contentusecase.ErrUnsupportedMediaType):
		response.ErrorUnsupportedMediaType(w)
	case errors.Is(err, contentusecase.ErrRangeNotSatisfiable):
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
	case errors.Is(err, validator.ErrValidation), errors.Is(err, contentusecase.ErrValidation):
		response.ErrorBadRequest(w)
	default:
		response.ErrorInternal(w)
	}
}

func logError(ctx context.Context, logger *slog.Logger, action string, err error) {
	if logger == nil {
		return
	}

	logger.ErrorContext(ctx, "content http error",
		slog.String("handler", action),
		slog.String("error", err.Error()),
	)
}
