package profileusecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/models"
	"github.com/google/uuid"
)

var (
	ErrProfileNotFound     = errors.New("profile not found")
	ErrProfileUserNotFound = errors.New("profile user not found")
	ErrAuthorNotFound      = errors.New("author not found")
)

type UserInfoProvider interface {
	GetProfileUserByID(ctx context.Context, userID uuid.UUID) (*models.ProfileUser, error)
}

type ProfileRepository interface {
	GetAuthorByUserID(ctx context.Context, userID uuid.UUID) (*models.Author, error)
	ListPostsByAuthorID(ctx context.Context, authorID uuid.UUID) ([]models.Post, error)
}

type ProfileUsecase struct {
	users    UserInfoProvider
	profiles ProfileRepository
}

func NewProfileUsecase(users UserInfoProvider, profiles ProfileRepository) *ProfileUsecase {
	return &ProfileUsecase{users: users, profiles: profiles}
}

func (uc *ProfileUsecase) GetMyProfile(ctx context.Context, userID uuid.UUID) (*dto.ProfileResponse, error) {
	userProfile, err := uc.users.GetProfileUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrProfileUserNotFound) {
			return nil, fmt.Errorf("get user: %w", ErrProfileNotFound)
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	author, err := uc.profiles.GetAuthorByUserID(ctx, userID)
	if errors.Is(err, ErrAuthorNotFound) {
		return dto.ToProfileResponse(userProfile, nil), nil
	}
	if err != nil {
		return nil, fmt.Errorf("get author: %w", err)
	}

	return dto.ToProfileResponse(userProfile, author), nil
}

func (uc *ProfileUsecase) GetMyPosts(ctx context.Context, userID uuid.UUID) (*dto.ProfilePostsResponse, error) {
	posts, err := uc.profiles.ListPostsByAuthorID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list author posts: %w", err)
	}

	return dto.ToProfilePostsResponse(userID, posts), nil
}
