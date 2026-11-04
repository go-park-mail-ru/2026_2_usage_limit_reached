package profileusecase

import (
	"context"
	"errors"
	"fmt"

	authordto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/author/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/models"
	userdto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/dto"
	"github.com/google/uuid"
)

var (
	ErrProfileNotFound = models.ErrProfileNotFound
	ErrNotAuthor       = errors.New("not an author")
	ErrUserInactive    = errors.New("user is not active")
)

type UserUpdater interface {
	UpdateNickname(ctx context.Context, userID uuid.UUID, nickname string) (string, error)
}

type AuthorUpdater interface {
	UpdateAuthor(ctx context.Context, authorID uuid.UUID, bio string, category *string) (*authordto.AuthorInfo, error)
}

type ProfileRepository interface {
	GetProfileByID(ctx context.Context, userID uuid.UUID) (*models.ProfileReadModel, error)
	GetProfileByUsername(ctx context.Context, username string) (*models.ProfileReadModel, error)
}

type ProfileUsecase struct {
	users        UserUpdater
	authors      AuthorUpdater
	profilesRepo ProfileRepository
}

func NewProfileUsecase(users UserUpdater, authors AuthorUpdater, profiles ProfileRepository) *ProfileUsecase {
	return &ProfileUsecase{users: users, authors: authors, profilesRepo: profiles}
}

func (uc *ProfileUsecase) GetMyProfile(ctx context.Context, userID uuid.UUID) (*dto.ProfileResponse, error) {
	profile, err := uc.getActiveProfile(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get profile: %w", err)
	}

	return dto.ToProfileResponse(profile), nil
}

func (uc *ProfileUsecase) GetPublicProfile(ctx context.Context, username string) (*dto.PublicProfileResponse, error) {
	profile, err := uc.profilesRepo.GetProfileByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("get public profile: %w", err)
	}

	if !userdto.IsActiveStatus(profile.Status) {
		return nil, fmt.Errorf("get public profile: %w", ErrProfileNotFound)
	}

	return dto.ToPublicProfileResponse(profile), nil
}

func (uc *ProfileUsecase) UpdateMyProfile(ctx context.Context, userID uuid.UUID, req dto.UpdateProfileRequest) (*dto.ProfileResponse, error) {
	profile, err := uc.getActiveProfile(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("update profile: %w", err)
	}

	if req.Author != nil {
		if profile.Author == nil {
			return nil, fmt.Errorf("update profile: %w", ErrNotAuthor)
		}

		author, err := uc.authors.UpdateAuthor(ctx, userID, req.Author.Bio, req.Author.Category)
		if err != nil {
			return nil, fmt.Errorf("update profile: %w", err)
		}

		profile.Author = &models.ProfileAuthorReadModel{Bio: author.Bio, Category: author.Category}
	}

	nickname, err := uc.users.UpdateNickname(ctx, userID, req.Nickname)
	if err != nil {
		return nil, fmt.Errorf("update profile: %w", err)
	}

	profile.Nickname = nickname

	return dto.ToProfileResponse(profile), nil
}

func (uc *ProfileUsecase) getActiveProfile(ctx context.Context, userID uuid.UUID) (*models.ProfileReadModel, error) {
	profile, err := uc.profilesRepo.GetProfileByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if !userdto.IsActiveStatus(profile.Status) {
		return nil, ErrUserInactive
	}

	return profile, nil
}
