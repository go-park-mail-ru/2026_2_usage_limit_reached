package posthandlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/dto"
	postusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type PostUsecase interface {
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
	UpdatePost(ctx context.Context, userID *uuid.UUID, postID uuid.UUID, p dto.PostRequest, lastUpdatedAt time.Time) error
	DeletePost(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) error

	LikePost(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) (int64, error)
	DeletePostLike(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) (int64, error)
}

type PostHandler struct {
	uc        PostUsecase
	validator *validator.Validator
	logger    *slog.Logger
}

func NewHandler(uc PostUsecase, v *validator.Validator, logger *slog.Logger) *PostHandler {
	return &PostHandler{uc: uc, validator: v, logger: logger}
}

func (h *PostHandler) RegisterRoutes(public *mux.Router, private *mux.Router) {
	public.HandleFunc("/feed", h.Feed).Methods(http.MethodGet, http.MethodOptions)
	public.HandleFunc("/posts/{id}", h.ViewPost).Methods(http.MethodGet, http.MethodOptions)

	private.HandleFunc("/profile/me/posts", h.MyPosts).Methods(http.MethodGet, http.MethodOptions)
	private.HandleFunc("/profile/me/drafts", h.MyDrafts).Methods(http.MethodGet, http.MethodOptions)
	private.HandleFunc("/posts", h.NewPost).Methods(http.MethodPost, http.MethodOptions)
	private.HandleFunc("/posts/{id}", h.EditPost).Methods(http.MethodPut, http.MethodOptions)
	private.HandleFunc("/posts/{id}", h.DeletePost).Methods(http.MethodDelete, http.MethodOptions)
	private.HandleFunc("/posts/{id}/like", h.LikePost).Methods(http.MethodPost, http.MethodOptions)
	private.HandleFunc("/posts/{id}/like", h.UnlikePost).Methods(http.MethodDelete, http.MethodOptions)
}

func (h *PostHandler) handleError(ctx context.Context, w http.ResponseWriter, handler string, logMessage string, err error) {
	h.logger.ErrorContext(ctx, logMessage,
		slog.String("handler", handler),
		slog.String("error", err.Error()),
	)
	switch {
	case errors.Is(err, postusecase.ErrConflict):
		response.ErrorPostConflict(w)
	case errors.Is(err, postusecase.ErrForbidden):
		response.ErrorForbidden(w)
	case errors.Is(err, postusecase.ErrUnauthorized):
		response.ErrorUnauthorized(w)
	case errors.Is(err, postusecase.ErrNotFound):
		response.ErrorNotFound(w)
	case errors.Is(err, validator.ErrValidation), errors.Is(err, postusecase.ErrValidation):
		response.ErrorBadRequest(w)
	default:
		response.ErrorInternal(w)
	}
}
