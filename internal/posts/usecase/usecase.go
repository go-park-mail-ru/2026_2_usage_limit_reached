package postusecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/models"
)

var (
	ErrForbidden    = errors.New("forbidden")
	ErrUnauthorized = errors.New("unauthorized")
	ErrValidation   = errors.New("validation error")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("post update conflict")
)

var repoErrors = map[error]error{
	models.ErrPostNotFound:    ErrNotFound,
	models.ErrInvalidAuthorID: ErrForbidden,
	models.ErrUnauthorized:    ErrUnauthorized,
	models.ErrPostValidation:  ErrValidation,
	models.ErrPostConflict:    ErrConflict,
}

func mapRepoError(op string, err error) error {
	to, ok := repoErrors[err]
	if ok {
		return fmt.Errorf("%s: %w", op, to)
	}
	return fmt.Errorf("%s: %w", op, err)
}

type PostRepository interface {
	CreatePost(ctx context.Context, userID *uuid.UUID, p models.Post) (uuid.UUID, error)
	GetPost(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) (*dto.PostInfo, error)
	GetMyNewPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.NewPostsCursor, limit int) ([]*dto.MyPostPreview, error)
	GetMyPopularPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.PopularPostsCursor, limit int) ([]*dto.MyPostPreview, error)
	GetMyDrafts(ctx context.Context, userID *uuid.UUID, cursor *dto.NewPostsCursor, limit int) ([]*dto.MyPostPreview, error)
	GetNewPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.NewPostsCursor, limit int) ([]*dto.PostPreview, error)
	GetPopularPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.PopularPostsCursor, limit int) ([]*dto.PostPreview, error)
	UpdatePost(ctx context.Context, userID *uuid.UUID, p models.Post, lastUpdatedAt time.Time) error
	DeletePost(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) error

	LikePost(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) (int64, error)
	DeletePostLike(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) (int64, error)
}

type PostUsecase struct { // TODO: добавить логгер
	repo PostRepository
}

func NewPostUsecase(r PostRepository) *PostUsecase {
	return &PostUsecase{repo: r}
}
