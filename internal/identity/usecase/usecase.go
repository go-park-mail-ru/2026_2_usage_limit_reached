package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/jwt"
	"github.com/google/uuid"
)

var (
	ErrUserNotFound          = errors.New("user not found")
	ErrUserExists            = errors.New("user already exists")
	ErrInvalidNickname       = errors.New("invalid nickname")
	ErrProfileNotFound       = errors.New("profile not found")
	ErrUserInactive          = errors.New("user is not active")
	ErrAuthorNotFound        = errors.New("author not found")
	ErrUnsupportedAvatarType = errors.New("unsupported avatar type")
	ErrAvatarTooLarge        = errors.New("avatar too large")
	ErrRegistrationFailed    = errors.New("registration failed")
	ErrLoginFailed           = errors.New("login failed")
	ErrInvalidToken          = errors.New("invalid token")
)

const MaxAvatarSize = 5 << 20

const (
	avatarKeyPrefix     = "avatars"
	avatarDeleteTimeout = 5 * time.Second
)

type avatarFormat struct {
	contentType string
	ext         string
}

var avatarFormats = map[string]avatarFormat{
	"jpeg": {contentType: "image/jpeg", ext: ".jpg"},
	"png":  {contentType: "image/png", ext: ".png"},
	"webp": {contentType: "image/webp", ext: ".webp"},
}

type Repository interface {
	CreateUser(ctx context.Context, user models.User) (*models.User, error)
	UpdateNickname(ctx context.Context, id uuid.UUID, nickname string) (string, error)
	UpdateAvatarKey(ctx context.Context, id uuid.UUID, key *string) (*string, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	GetUserByUsername(ctx context.Context, username string) (*models.User, error)
	GetUsersByIDs(ctx context.Context, ids []uuid.UUID) ([]models.User, error)

	SearchAuthors(ctx context.Context, query string, categories []string, cursor *dto.AuthorSearchCursor, limit int) ([]*models.Author, error)
	CreateAuthor(ctx context.Context, id uuid.UUID, bio string, category *string) (*models.Author, error)
	GetAuthorByID(ctx context.Context, id uuid.UUID) (*models.Author, error)
	UpdateAuthorDetails(ctx context.Context, id uuid.UUID, bio string, category *string) (*models.Author, error)
}

type TokenManager interface {
	Generate(ID uuid.UUID, payload jwt.Payload) (string, error)
	TTL() time.Duration
	Verify(token string) (json.RawMessage, error)
}

type AvatarStorage interface {
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	Delete(ctx context.Context, key string) error
}

type Usecase struct {
	repo           Repository
	tokens         TokenManager
	avatars        AvatarStorage
	avatarBasePath string
	logger         *slog.Logger
}

func New(repo Repository, tokens TokenManager, avatars AvatarStorage, avatarBasePath string, logger *slog.Logger) *Usecase {
	return &Usecase{
		repo:           repo,
		tokens:         tokens,
		avatars:        avatars,
		avatarBasePath: avatarBasePath,
		logger:         logger,
	}
}

func (uc *Usecase) GetUsers(ctx context.Context, ids []uuid.UUID) ([]*dto.UserInfo, error) {
	return uc.FindUsersByIDs(ctx, ids)
}

func (uc *Usecase) GetUserByUsername(ctx context.Context, username string) (*dto.UserInfo, error) {
	return uc.FindUserByUsername(ctx, username)
}
