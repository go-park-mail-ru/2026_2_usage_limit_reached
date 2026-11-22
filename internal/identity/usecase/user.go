package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/mediaurl"
	"github.com/google/uuid"
)

func (uc *Usecase) createUser(ctx context.Context, req dto.CreateUserRequest) (*dto.UserInfo, error) {
	user, err := uc.repo.CreateUser(ctx, models.User{
		Email:        normalizeLogin(req.Email),
		Username:     normalizeLogin(req.Username),
		Nickname:     strings.TrimSpace(req.Nickname),
		PasswordHash: req.PasswordHash,
		Status:       models.StatusActive,
	})
	if err != nil {
		if errors.Is(err, models.ErrUserExists) {
			return nil, fmt.Errorf("create user: %w", ErrUserExists)
		}

		return nil, fmt.Errorf("create user: %w", err)
	}

	return dto.ToUserInfo(user, mediaurl.PublicPtr(uc.avatarBasePath, user.AvatarKey)), nil
}

func (uc *Usecase) UpdateNickname(ctx context.Context, userID uuid.UUID, nickname string) (string, error) {
	nickname = strings.TrimSpace(nickname)
	if nickname == "" {
		return "", fmt.Errorf("update nickname: %w", ErrInvalidNickname)
	}

	updated, err := uc.repo.UpdateNickname(ctx, userID, nickname)
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			return "", fmt.Errorf("update nickname: %w", ErrUserNotFound)
		}

		return "", fmt.Errorf("update nickname: %w", err)
	}

	return updated, nil
}

func (uc *Usecase) UpdateAvatarKey(ctx context.Context, userID uuid.UUID, key *string) (*string, error) {
	oldKey, err := uc.repo.UpdateAvatarKey(ctx, userID, key)
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			return nil, fmt.Errorf("update avatar key: %w", ErrUserNotFound)
		}

		return nil, fmt.Errorf("update avatar key: %w", err)
	}

	return oldKey, nil
}

func (uc *Usecase) FindUserByID(ctx context.Context, userID uuid.UUID) (*dto.UserInfo, error) {
	user, err := uc.repo.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			return nil, fmt.Errorf("find user: %w", ErrUserNotFound)
		}

		return nil, fmt.Errorf("find user: %w", err)
	}

	return dto.ToUserInfo(user, mediaurl.PublicPtr(uc.avatarBasePath, user.AvatarKey)), nil
}

func (uc *Usecase) FindUsersByIDs(ctx context.Context, ids []uuid.UUID) ([]*dto.UserInfo, error) {
	if len(ids) == 0 {
		return []*dto.UserInfo{}, nil
	}

	users, err := uc.repo.GetUsersByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("find users: %w", err)
	}

	infos := make([]*dto.UserInfo, 0, len(users))
	for i := range users {
		infos = append(infos, dto.ToUserInfo(&users[i], mediaurl.PublicPtr(uc.avatarBasePath, users[i].AvatarKey)))
	}

	return infos, nil
}

func (uc *Usecase) FindUserByUsername(ctx context.Context, username string) (*dto.UserInfo, error) {
	user, err := uc.repo.GetUserByUsername(ctx, normalizeLogin(username))
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			return nil, fmt.Errorf("find user by username: %w", ErrUserNotFound)
		}

		return nil, fmt.Errorf("find user by username: %w", err)
	}

	return dto.ToUserInfo(user, mediaurl.PublicPtr(uc.avatarBasePath, user.AvatarKey)), nil
}

func (uc *Usecase) findCredentialsByLogin(ctx context.Context, login string) (*dto.UserCredentials, error) {
	var (
		user *models.User
		err  error
	)

	if strings.ContainsRune(login, '@') {
		user, err = uc.repo.GetUserByEmail(ctx, login)
	} else {
		user, err = uc.repo.GetUserByUsername(ctx, login)
	}
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			return nil, fmt.Errorf("find user: %w", ErrUserNotFound)
		}

		return nil, fmt.Errorf("find user: %w", err)
	}

	return dto.ToUserCredentials(user, mediaurl.PublicPtr(uc.avatarBasePath, user.AvatarKey)), nil
}

func normalizeLogin(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
