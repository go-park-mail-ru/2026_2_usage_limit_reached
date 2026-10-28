package profileusecase

import (
	"context"
	"errors"
	"fmt"

	authordto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/author/dto"
	authorusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/author/usecase"
	postdto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/dto"
	userdto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/dto"
	userusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/usecase"
	"github.com/google/uuid"
)

var (
	ErrProfileNotFound = errors.New("profile not found")
)

type UserInfoProvider interface {
	FindUserByID(ctx context.Context, userID uuid.UUID) (*userdto.UserInfo, error)
}

type AuthorInfoProvider interface {
	FindAuthorByID(ctx context.Context, authorID uuid.UUID) (*authordto.AuthorInfo, error)
}

type PostsProvider interface {
	ListPostsByAuthor(ctx context.Context, authorID uuid.UUID) ([]*postdto.PostInfo, error)
}

type ProfileUsecase struct {
	usersRepo UserInfoProvider
	authors   AuthorInfoProvider
	posts     PostsProvider
}

func NewProfileUsecase(users UserInfoProvider, authors AuthorInfoProvider, posts PostsProvider) *ProfileUsecase {
	return &ProfileUsecase{usersRepo: users, authors: authors, posts: posts}
}

func (uc *ProfileUsecase) GetMyProfile(ctx context.Context, userID uuid.UUID) (*dto.ProfileResponse, error) {
	userProfile, err := uc.usersRepo.FindUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, userusecase.ErrUserNotFound) {
			return nil, fmt.Errorf("get user: %w", ErrProfileNotFound)
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	author, err := uc.authors.FindAuthorByID(ctx, userID)
	if errors.Is(err, authorusecase.ErrAuthorNotFound) {
		return dto.ToProfileResponse(userProfile, nil), nil
	}
	if err != nil {
		return nil, fmt.Errorf("get author: %w", err)
	}

	return dto.ToProfileResponse(userProfile, author), nil
}

func (uc *ProfileUsecase) GetMyPosts(ctx context.Context, userID uuid.UUID) (*dto.ProfilePostsResponse, error) {
	posts, err := uc.posts.ListPostsByAuthor(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list author posts: %w", err)
	}

	return dto.ToProfilePostsResponse(userID, posts), nil
}
