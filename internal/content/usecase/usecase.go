package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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

	// ErrAdvancedFiltersDisabled — фильтр ленты по вложениям выключен для
	// пользователя: delivery отвечает 403 feed_advanced_filters_disabled.
	ErrAdvancedFiltersDisabled = errors.New("feed advanced filters disabled")
)

var repoErrors = []struct{ from, to error }{
	{models.ErrPostNotFound, ErrNotFound},
	{models.ErrInvalidAuthorID, ErrForbidden},
	{models.ErrUnauthorized, ErrUnauthorized},
	{models.ErrPostValidation, ErrValidation},
	{models.ErrPostConflict, ErrConflict},
	{models.ErrMediaNotFound, ErrNotFound},
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
	GetNewPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int, filters dto.FeedFilters) ([]*dto.PostPreview, error)
	GetOldPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int, filters dto.FeedFilters) ([]*dto.PostPreview, error)
	GetPopularPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.PopularPostsCursor, limit int, filters dto.FeedFilters) ([]*dto.PostPreview, error)
	GetUserNewPosts(ctx context.Context, userID *uuid.UUID, authorID uuid.UUID, cursor *dto.TimedPostsCursor, limit int) ([]*dto.PostPreview, error)
	GetUserOldPosts(ctx context.Context, userID *uuid.UUID, authorID uuid.UUID, cursor *dto.TimedPostsCursor, limit int) ([]*dto.PostPreview, error)
	GetUserPopularPosts(ctx context.Context, userID *uuid.UUID, authorID uuid.UUID, cursor *dto.PopularPostsCursor, limit int) ([]*dto.PostPreview, error)
	UpdatePost(ctx context.Context, userID *uuid.UUID, p models.Post, lastUpdatedAt time.Time) ([]string, error)
	DeletePost(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) ([]string, error)

	CreateMedia(ctx context.Context, m models.Media) (models.Media, error)
	CountPendingMedia(ctx context.Context, authorID uuid.UUID) (int64, error)
	GetMediaForAccess(ctx context.Context, userID *uuid.UUID, mediaID uuid.UUID) (models.Media, error)

	LikePost(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) (int64, error)
	DeletePostLike(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) (int64, error)
}

type IdentityClient interface {
	GetUsers(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*identitycontracts.UserInfo, error)
	GetUserByUsername(ctx context.Context, username string) (*identitycontracts.UserInfo, error)
}

type OnlineConfClient interface {
	Enabled(scope, key string, userID uuid.UUID) bool
}

// Проверка тела поста после санитайзинга. Интерфейс определен здесь, чтобы usecase не зависел от пакета validator целиком
type SanitizedBodyValidator interface {
	IsValidSanitizedPostBody(raw, sanitized string) bool
}

// Лимиты вложений. Интерфейс определён здесь, чтобы usecase не зависел от пакета validator целиком
type AttachmentLimiter interface {
	MaxImageSize() int64
	MaxVideoSize() int64
	MaxAudioSize() int64
	MaxFileSize() int64
	MaxPostAttachments() int
	MaxPendingMedia() int
}

type ContentUsecase struct {
	repo           Repository
	identity       IdentityClient
	onlineconf     OnlineConfClient
	sanitizer      *sanitizer.Sanitizer
	bodyCheck      SanitizedBodyValidator
	attLimits      AttachmentLimiter
	media          MediaStorage
	avatarBasePath string
	logger         *slog.Logger
}

func New(
	r Repository,
	ic IdentityClient,
	oc OnlineConfClient,
	s *sanitizer.Sanitizer,
	v SanitizedBodyValidator,
	abp string,
	limits AttachmentLimiter,
	media MediaStorage,
	logger *slog.Logger,
) *ContentUsecase {
	return &ContentUsecase{
		repo:           r,
		identity:       ic,
		onlineconf:     oc,
		sanitizer:      s,
		bodyCheck:      v,
		attLimits:      limits,
		media:          media,
		avatarBasePath: abp,
		logger:         logger,
	}
}
