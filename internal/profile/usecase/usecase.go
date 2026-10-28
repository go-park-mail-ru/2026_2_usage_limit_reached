package profileusecase

import (
	"context"
	"errors"
	"fmt"

	authordto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/author/dto"
	authorusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/author/usecase"
	postdto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/models"
	userdto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/dto"
	userusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/usecase"
	"github.com/google/uuid"
)

var (
	ErrProfileNotFound = errors.New("profile not found")
	ErrProfileConflict = errors.New("profile conflict")
	ErrInvalidProfile  = errors.New("invalid profile")
	ErrNotAuthor       = errors.New("not an author")
)

var repoErrors = map[error]error{
	models.ErrProfileNotFound:   ErrProfileNotFound,
	models.ErrProfileConflict:   ErrProfileConflict,
	models.ErrProfileValidation: ErrInvalidProfile,
	models.ErrNotAuthor:         ErrNotAuthor,
}

type UserInfoProvider interface {
	FindUserByID(ctx context.Context, userID uuid.UUID) (*userdto.UserInfo, error)
}

type AuthorInfoProvider interface {
	FindAuthorByID(ctx context.Context, authorID uuid.UUID) (*authordto.AuthorInfo, error)
}

type PostsProvider interface {
	ListPostsByAuthor(ctx context.Context, authorID uuid.UUID) ([]*postdto.PostInfo, error)
}

type ProfileRepository interface {
	UpdateProfile(ctx context.Context, p *models.ProfileUpdate) error
}

type ProfileUsecase struct {
	usersRepo    UserInfoProvider
	authors      AuthorInfoProvider
	posts        PostsProvider
	profilesRepo ProfileRepository
}

func NewProfileUsecase(users UserInfoProvider, authors AuthorInfoProvider, posts PostsProvider, profiles ProfileRepository) *ProfileUsecase {
	return &ProfileUsecase{usersRepo: users, authors: authors, posts: posts, profilesRepo: profiles}
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

func (uc *ProfileUsecase) UpdateMyProfile(ctx context.Context, userID uuid.UUID, req dto.UpdateProfileRequest) error {
	update := &models.ProfileUpdate{
		UserID:   userID,
		Username: userusecase.Normalize(req.User.Username),
		Nickname: req.User.Nickname,
		Email:    userusecase.Normalize(req.User.Email),
	}
	if req.Author != nil {
		update.Author = &models.AuthorUpdate{
			Bio:             req.Author.Bio,
			Category:        req.Author.Category,
			PayoutProvider:  req.Author.PayoutProvider,
			PayoutAccountID: req.Author.PayoutAccountID,
		}
	}

	if err := uc.profilesRepo.UpdateProfile(ctx, update); err != nil {
		if to, ok := repoErrors[err]; ok {
			return fmt.Errorf("update profile: %w: %w", to, err)
		}
		return fmt.Errorf("update profile: %w", err)
	}

	return nil
}
