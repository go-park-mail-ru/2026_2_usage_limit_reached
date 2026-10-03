package profileusecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/domain"
	"github.com/google/uuid"
)

var ErrProfileNotFound = errors.New("profile not found")

type UserRepository interface {
	GetUserByID(ctx context.Context, userID uuid.UUID) (*models.User, error)
}

type ProfileRepository interface {
	GetAuthorByUserID(ctx context.Context, userID uuid.UUID) (*domain.Author, bool, error)
	ListPostsByAuthorID(ctx context.Context, authorID uuid.UUID) ([]domain.Post, error)
}

type ProfileUsecase struct {
	users    UserRepository
	profiles ProfileRepository
}

func NewProfileUsecase(users UserRepository, profiles ProfileRepository) *ProfileUsecase {
	return &ProfileUsecase{users: users, profiles: profiles}
}

func (uc *ProfileUsecase) GetMyProfile(ctx context.Context, userID uuid.UUID) (*domain.Profile, error) {
	user, err := uc.users.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			return nil, fmt.Errorf("%w: %w", ErrProfileNotFound, err)
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	profile := &domain.Profile{
		User: domain.ProfileUser{
			ID:        user.ID,
			Username:  user.Username,
			Nickname:  user.Nickname,
			Email:     user.Email,
			AvatarKey: user.AvatarKey,
			CreatedAt: user.CreatedAt,
		},
		Posts: []domain.Post{},
	}
	author, found, err := uc.profiles.GetAuthorByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get author: %w", err)
	}

	if !found {
		return profile, nil
	}

	profile.Author = &domain.Author{Bio: author.Bio, Category: author.Category}
	posts, err := uc.profiles.ListPostsByAuthorID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list author posts: %w", err)
	}

	profile.Posts = posts
	return profile, nil
}
