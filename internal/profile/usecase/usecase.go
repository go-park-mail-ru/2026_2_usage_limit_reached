package profileusecase

import (
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"path"
	"strings"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/dto"
	authdto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/dto"
	authuc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/models"
	"github.com/google/uuid"
	_ "golang.org/x/image/webp"
)

var (
	ErrProfileNotFound       = errors.New("profile not found")
	ErrAuthorNotFound        = errors.New("author not found")
	ErrUnsupportedAvatarType = errors.New("unsupported avatar type")
)

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

type UserInfoProvider interface {
	FindUserByID(ctx context.Context, userID uuid.UUID) (*authdto.UserInfo, error)
}

type ProfileRepository interface {
	GetAuthorByUserID(ctx context.Context, userID uuid.UUID) (*models.Author, error)
	ListPostsByAuthorID(ctx context.Context, authorID uuid.UUID) ([]models.Post, error)
	UpdateAvatarKey(ctx context.Context, userID uuid.UUID, key *string) (*string, error)
}

type AvatarStorage interface {
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	Delete(ctx context.Context, key string) error
}

type ProfileUsecase struct {
	usersRepo     UserInfoProvider
	profilesRepo  ProfileRepository
	avatarStorage AvatarStorage
	avatarBaseURL string
	logger        *slog.Logger
}

func NewProfileUsecase(users UserInfoProvider, profiles ProfileRepository, avatars AvatarStorage, avatarBaseURL string, logger *slog.Logger) *ProfileUsecase {
	return &ProfileUsecase{
		usersRepo:     users,
		profilesRepo:  profiles,
		avatarStorage: avatars,
		avatarBaseURL: strings.TrimSuffix(avatarBaseURL, "/"),
		logger:        logger,
	}
}

func (uc *ProfileUsecase) GetMyProfile(ctx context.Context, userID uuid.UUID) (*dto.ProfileResponse, error) {
	userProfile, err := uc.usersRepo.FindUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, authuc.ErrUserNotFound) {
			return nil, fmt.Errorf("get user: %w", ErrProfileNotFound)
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	avatarURL := uc.avatarURL(userProfile.AvatarKey)

	author, err := uc.profilesRepo.GetAuthorByUserID(ctx, userID)
	if errors.Is(err, ErrAuthorNotFound) {
		return dto.ToProfileResponse(userProfile, nil, avatarURL), nil
	}
	if err != nil {
		return nil, fmt.Errorf("get author: %w", err)
	}

	return dto.ToProfileResponse(userProfile, author, avatarURL), nil
}

func (uc *ProfileUsecase) GetMyPosts(ctx context.Context, userID uuid.UUID) (*dto.ProfilePostsResponse, error) {
	posts, err := uc.profilesRepo.ListPostsByAuthorID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list author posts: %w", err)
	}

	return dto.ToProfilePostsResponse(userID, posts), nil
}

func (uc *ProfileUsecase) UploadAvatar(ctx context.Context, userID uuid.UUID, file io.ReadSeeker, size int64) (string, error) {
	_, formatName, err := image.DecodeConfig(file)
	if err != nil {
		return "", fmt.Errorf("upload avatar: %w: %w", ErrUnsupportedAvatarType, err)
	}
	format, ok := avatarFormats[formatName]
	if !ok {
		return "", fmt.Errorf("upload avatar: %w: %s", ErrUnsupportedAvatarType, formatName)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("rewind avatar: %w", err)
	}

	key := path.Join(avatarKeyPrefix, userID.String(), uuid.NewString() + format.ext)

	if err := uc.avatarStorage.Put(ctx, key, file, size, format.contentType); err != nil {
		return "", fmt.Errorf("put avatar: %w", err)
	}

	oldKey, err := uc.profilesRepo.UpdateAvatarKey(ctx, userID, &key)
	if err != nil {
		if delErr := uc.deleteAvatarObject(ctx, key); delErr != nil {
			err = errors.Join(err, delErr)
		}
		return "", fmt.Errorf("set avatar key: %w", err)
	}

	uc.deleteOldAvatar(ctx, oldKey)

	return uc.objectURL(key), nil
}

func (uc *ProfileUsecase) DeleteAvatar(ctx context.Context, userID uuid.UUID) error {
	oldKey, err := uc.profilesRepo.UpdateAvatarKey(ctx, userID, nil)
	if err != nil {
		return fmt.Errorf("clear avatar key: %w", err)
	}

	uc.deleteOldAvatar(ctx, oldKey)

	return nil
}

func (uc *ProfileUsecase) deleteOldAvatar(ctx context.Context, key *string) {
	if key == nil {
		return
	}
	if err := uc.deleteAvatarObject(ctx, *key); err != nil {
		uc.logger.ErrorContext(ctx, "delete old avatar error",
			slog.String("key", *key),
			slog.String("error", err.Error()),
		)
	}
}

func (uc *ProfileUsecase) avatarURL(key *string) *string {
	if key == nil {
		return nil
	}
	u := uc.objectURL(*key)
	return &u
}

func (uc *ProfileUsecase) objectURL(key string) string {
	return uc.avatarBaseURL + "/" + key
}

func (uc *ProfileUsecase) deleteAvatarObject(ctx context.Context, key string) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), avatarDeleteTimeout)
	defer cancel()
	return uc.avatarStorage.Delete(ctx, key)
}
