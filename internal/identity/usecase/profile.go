package usecase

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

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/mediaurl"
	"github.com/google/uuid"
	_ "golang.org/x/image/webp"
)

func (uc *Usecase) GetMyProfile(ctx context.Context, userID uuid.UUID) (*dto.ProfileResponse, error) {
	user, err := uc.getActiveUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get profile: %w", err)
	}

	return dto.ToProfileResponse(user, mediaurl.PublicPtr(uc.avatarBasePath, user.AvatarKey)), nil
}

func (uc *Usecase) GetPublicProfile(ctx context.Context, username string) (*dto.PublicProfileResponse, error) {
	user, err := uc.repo.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			return nil, fmt.Errorf("get public profile: %w", ErrProfileNotFound)
		}

		return nil, fmt.Errorf("get public profile: %w", err)
	}

	if !dto.IsActiveStatus(user.Status) {
		return nil, fmt.Errorf("get public profile: %w", ErrProfileNotFound)
	}

	return dto.ToPublicProfileResponse(user, mediaurl.PublicPtr(uc.avatarBasePath, user.AvatarKey)), nil
}

func (uc *Usecase) UpdateMyProfile(ctx context.Context, userID uuid.UUID, req dto.UpdateProfileRequest) (*dto.ProfileResponse, error) {
	user, err := uc.getActiveUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("update profile: %w", err)
	}

	nickname, err := uc.repo.UpdateNickname(ctx, userID, req.Nickname)
	if err != nil {
		return nil, fmt.Errorf("update profile: %w", err)
	}

	user.Nickname = nickname

	return dto.ToProfileResponse(user, mediaurl.PublicPtr(uc.avatarBasePath, user.AvatarKey)), nil
}

func (uc *Usecase) getActiveUser(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	user, err := uc.repo.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			return nil, ErrProfileNotFound
		}

		return nil, err
	}

	if !dto.IsActiveStatus(user.Status) {
		return nil, ErrUserInactive
	}

	return user, nil
}

func (uc *Usecase) UploadAvatar(ctx context.Context, userID uuid.UUID, file io.ReadSeeker, size int64) (string, error) {
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

	if err := uc.avatars.Put(ctx, key, file, size, format.contentType); err != nil {
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

	return mediaurl.Public(uc.avatarBasePath, key), nil
}

func (uc *Usecase) DeleteAvatar(ctx context.Context, userID uuid.UUID) error {
	oldKey, err := uc.updateAvatarKey(ctx, userID, nil)
	if err != nil {
		return fmt.Errorf("clear avatar key: %w", err)
	}

	uc.deleteOldAvatar(ctx, oldKey)

	return nil
}

func (uc *Usecase) updateAvatarKey(ctx context.Context, userID uuid.UUID, key *string) (*string, error) {
	oldKey, err := uc.repo.UpdateAvatarKey(ctx, userID, key)
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			return nil, ErrProfileNotFound
		}

		return nil, err
	}

	return oldKey, nil
}

func (uc *Usecase) deleteOldAvatar(ctx context.Context, key *string) {
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

func (uc *Usecase) deleteAvatarObject(ctx context.Context, key string) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), avatarDeleteTimeout)
	defer cancel()

	return uc.avatars.Delete(ctx, key)
}
