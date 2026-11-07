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

	authordto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/author/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/models"
	userdto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/dto"
	userusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/usecase"
	"github.com/google/uuid"
	_ "golang.org/x/image/webp"
)

var (
	ErrProfileNotFound       = errors.New("profile not found")
	ErrNotAuthor             = errors.New("not an author")
	ErrUserInactive          = errors.New("user is not active")
	ErrUnsupportedAvatarType = errors.New("unsupported avatar type")
	ErrAvatarTooLarge        = errors.New("avatar too large")
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

type UserUsecase interface {
	UpdateNickname(ctx context.Context, userID uuid.UUID, nickname string) (string, error)
	UpdateAvatarKey(ctx context.Context, userID uuid.UUID, key *string) (*string, error)
}

type AuthorUsecase interface {
	UpdateAuthor(ctx context.Context, req authordto.UpdateAuthorRequest) (*authordto.AuthorInfo, error)
}

type ProfileRepository interface {
	GetProfileByID(ctx context.Context, userID uuid.UUID) (*models.Profile, error)
	GetProfileByUsername(ctx context.Context, username string) (*models.Profile, error)
}

type AvatarStorage interface {
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	Delete(ctx context.Context, key string) error
}

type ProfileUsecase struct {
	users          UserUsecase
	authors        AuthorUsecase
	profilesRepo   ProfileRepository
	avatarStorage  AvatarStorage
	avatarBasePath string
	logger         *slog.Logger
}

func NewProfileUsecase(users UserUsecase, authors AuthorUsecase, profiles ProfileRepository, avatars AvatarStorage, avatarBasePath string, logger *slog.Logger) *ProfileUsecase {
	return &ProfileUsecase{
		users:          users,
		authors:        authors,
		profilesRepo:   profiles,
		avatarStorage:  avatars,
		avatarBasePath: strings.TrimSuffix(avatarBasePath, "/"),
		logger:         logger,
	}
}

func (uc *ProfileUsecase) GetMyProfile(ctx context.Context, userID uuid.UUID) (*dto.ProfileResponse, error) {
	profile, err := uc.getActiveProfile(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get profile: %w", err)
	}

	return dto.ToProfileResponse(profile, uc.avatarURL(profile.AvatarKey)), nil
}

func (uc *ProfileUsecase) GetPublicProfile(ctx context.Context, username string) (*dto.PublicProfileResponse, error) {
	profile, err := uc.profilesRepo.GetProfileByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, models.ErrProfileNotFound) {
			return nil, fmt.Errorf("get public profile: %w", ErrProfileNotFound)
		}
		return nil, fmt.Errorf("get public profile: %w", err)
	}

	if !userdto.IsActiveStatus(profile.Status) {
		return nil, fmt.Errorf("get public profile: %w", ErrProfileNotFound)
	}

	return dto.ToPublicProfileResponse(profile, uc.avatarURL(profile.AvatarKey)), nil
}

func (uc *ProfileUsecase) UpdateMyProfile(ctx context.Context, userID uuid.UUID, req dto.UpdateProfileRequest) (*dto.ProfileResponse, error) {
	profile, err := uc.getActiveProfile(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("update profile: %w", err)
	}

	rollback := func() error {
		return nil
	}

	if req.Author != nil {
		if profile.Author == nil {
			return nil, fmt.Errorf("update profile: %w", ErrNotAuthor)
		}

		oldAuthor := *profile.Author

		author, err := uc.authors.UpdateAuthor(ctx, authordto.UpdateAuthorRequest{
			AuthorID: userID,
			Bio:      req.Author.Bio,
			Category: req.Author.Category,
		})
		if err != nil {
			return nil, fmt.Errorf("update profile: %w", err)
		}

		rollback = func() error {
			_, err := uc.authors.UpdateAuthor(ctx, authordto.UpdateAuthorRequest{
				AuthorID: userID,
				Bio:      oldAuthor.Bio,
				Category: oldAuthor.Category,
			})
			return err
		}

		profile.Author = &models.ProfileAuthor{Bio: author.Bio, Category: author.Category}
	}

	nickname, err := uc.users.UpdateNickname(ctx, userID, req.Nickname)
	if err != nil {
		if rbErr := rollback(); rbErr != nil {
			return nil, fmt.Errorf("update profile: %w", errors.Join(err, fmt.Errorf("rollback author: %w", rbErr)))
		}
		return nil, fmt.Errorf("update profile: %w", err)
	}

	profile.Nickname = nickname

	return dto.ToProfileResponse(profile, uc.avatarURL(profile.AvatarKey)), nil
}

func (uc *ProfileUsecase) getActiveProfile(ctx context.Context, userID uuid.UUID) (*models.Profile, error) {
	profile, err := uc.profilesRepo.GetProfileByID(ctx, userID)
	if err != nil {
		if errors.Is(err, models.ErrProfileNotFound) {
			return nil, ErrProfileNotFound
		}
		return nil, err
	}

	if !userdto.IsActiveStatus(profile.Status) {
		return nil, ErrUserInactive
	}

	return profile, nil
}

func (uc *ProfileUsecase) UploadAvatar(ctx context.Context, userID uuid.UUID, file io.ReadSeeker, size int64) (string, error) {
	if size > MaxAvatarSize {
		return "", fmt.Errorf("upload avatar: %w", ErrAvatarTooLarge)
	}

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

	key := path.Join(avatarKeyPrefix, userID.String(), uuid.NewString()+format.ext)

	if err := uc.avatarStorage.Put(ctx, key, file, size, format.contentType); err != nil {
		return "", fmt.Errorf("put avatar: %w", err)
	}

	oldKey, err := uc.updateAvatarKey(ctx, userID, &key)
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
	oldKey, err := uc.updateAvatarKey(ctx, userID, nil)
	if err != nil {
		return fmt.Errorf("clear avatar key: %w", err)
	}

	uc.deleteOldAvatar(ctx, oldKey)

	return nil
}

func (uc *ProfileUsecase) updateAvatarKey(ctx context.Context, userID uuid.UUID, key *string) (*string, error) {
	oldKey, err := uc.users.UpdateAvatarKey(ctx, userID, key)
	if err != nil {
		if errors.Is(err, userusecase.ErrUserNotFound) {
			return nil, ErrProfileNotFound
		}

		return nil, err
	}

	return oldKey, nil
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
	return uc.avatarBasePath + "/" + key
}

func (uc *ProfileUsecase) deleteAvatarObject(ctx context.Context, key string) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), avatarDeleteTimeout)
	defer cancel()

	return uc.avatarStorage.Delete(ctx, key)
}
