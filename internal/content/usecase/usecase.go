package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/models"
	identitycontracts "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/contracts/identity"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/sanitizer"
)

var (
	ErrForbidden    = errors.New("forbidden")
	ErrUnauthorized = errors.New("unauthorized")
	ErrValidation   = errors.New("validation error")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("post update conflict")
)

var repoErrors = []struct{ from, to error }{
	{models.ErrPostNotFound, ErrNotFound},
	{models.ErrInvalidAuthorID, ErrForbidden},
	{models.ErrUnauthorized, ErrUnauthorized},
	{models.ErrPostValidation, ErrValidation},
	{models.ErrPostConflict, ErrConflict},
}

func mapRepoError(op string, err error) error {
	for _, e := range repoErrors {
		if errors.Is(err, e.from) {
			return fmt.Errorf("%s: %w", op, e.to)
		}
	}
	return fmt.Errorf("%s: %w", op, err)
}

type Repository interface {
	CreatePost(ctx context.Context, userID *uuid.UUID, p models.Post) (uuid.UUID, error)
	GetPost(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) (*dto.PostInfo, error)
	GetMyNewPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) ([]*dto.PostPreview, error)
	GetMyOldPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) ([]*dto.PostPreview, error)
	GetMyPopularPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.PopularPostsCursor, limit int) ([]*dto.PostPreview, error)
	GetMyNewDrafts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) ([]*dto.PostPreview, error)
	GetMyOldDrafts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) ([]*dto.PostPreview, error)
	GetNewPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) ([]*dto.PostPreview, error)
	GetOldPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) ([]*dto.PostPreview, error)
	GetPopularPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.PopularPostsCursor, limit int) ([]*dto.PostPreview, error)
	GetUserNewPosts(ctx context.Context, userID *uuid.UUID, authorID uuid.UUID, cursor *dto.TimedPostsCursor, limit int) ([]*dto.PostPreview, error)
	GetUserOldPosts(ctx context.Context, userID *uuid.UUID, authorID uuid.UUID, cursor *dto.TimedPostsCursor, limit int) ([]*dto.PostPreview, error)
	GetUserPopularPosts(ctx context.Context, userID *uuid.UUID, authorID uuid.UUID, cursor *dto.PopularPostsCursor, limit int) ([]*dto.PostPreview, error)
	UpdatePost(ctx context.Context, userID *uuid.UUID, p models.Post, lastUpdatedAt time.Time) error
	DeletePost(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) error

	LikePost(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) (int64, error)
	DeletePostLike(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) (int64, error)
}

type IdentityClient interface {
	GetUsers(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*identitycontracts.UserInfo, error)
	GetUserByUsername(ctx context.Context, username string) (*identitycontracts.UserInfo, error)
}

// Проверка тела поста после санитайзинга. Интерфейс определен здесь, чтобы usecase не зависел от пакета validator целиком
type SanitizedBodyValidator interface {
	IsValidSanitizedPostBody(raw, sanitized string) bool
}

type ContentUsecase struct {
	repo           Repository
	identity       IdentityClient
	sanitizer      *sanitizer.Sanitizer
	bodyCheck      SanitizedBodyValidator
	avatarBasePath string
}

func New(r Repository, ic IdentityClient, s *sanitizer.Sanitizer, v SanitizedBodyValidator, abp string) *ContentUsecase {
	return &ContentUsecase{repo: r, identity: ic, sanitizer: s, bodyCheck: v, avatarBasePath: abp}
}
