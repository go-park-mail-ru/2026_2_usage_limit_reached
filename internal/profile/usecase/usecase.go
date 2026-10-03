package profileusecase

import (
	"context"
	"fmt"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/domain"
	"github.com/google/uuid"
)

type UserRepository interface {
	GetUserByID(ctx context.Context, userID uuid.UUID) (models.User, error)
}

type ProfileRepository interface {
	GetAuthorByUserID(ctx context.Context, userID uuid.UUID) (domain.Author, bool, error)
	ListPostsByAuthorID(ctx context.Context, authorID uuid.UUID) ([]domain.Post, error)
}

type ProfileUsecase struct {
	users    UserRepository
	profiles ProfileRepository
}

func NewProfileUsecase(users UserRepository, profiles ProfileRepository) *ProfileUsecase {
	return &ProfileUsecase{users: users, profiles: profiles}
}

func (uc *ProfileUsecase) GetMyProfile(ctx context.Context, userID uuid.UUID) (domain.Profile, error) {
	user, err := uc.users.GetUserByID(ctx, userID)
	if err != nil {
		return domain.Profile{}, fmt.Errorf("get user: %w", err)
	}

	profile := domain.Profile{User: user, Posts: []domain.Post{}}
	author, found, err := uc.profiles.GetAuthorByUserID(ctx, userID)
	if err != nil {
		return domain.Profile{}, fmt.Errorf("get author: %w", err)
	}

	if !found {
		return profile, nil
	}

	profile.Author = &author
	posts, err := uc.profiles.ListPostsByAuthorID(ctx, author.ID)
	if err != nil {
		return domain.Profile{}, fmt.Errorf("list author posts: %w", err)
	}

	profile.Posts = posts
	return profile, nil
}
