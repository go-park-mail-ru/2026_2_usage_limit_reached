package postusecase

import (
	"context"
	"fmt"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/sanitizer"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/models"
	"github.com/google/uuid"
)

func (uc *PostUsecase) CreatePost(ctx context.Context, userID *uuid.UUID, p dto.PostRequest) (uuid.UUID, error) {
	if userID == nil {
		return uuid.Nil, fmt.Errorf("%s: %w", "create post", ErrUnauthorized)
	}

	body, err := sanitizeBody(p.Body)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%s: %w", "create post", err)
	}

	post := models.Post{
		Title:  p.Title,
		Body:   body,
		Status: p.Status,
	}
	postID, err := uc.repo.CreatePost(ctx, userID, post)
	if err != nil {
		return uuid.Nil, mapRepoError("create post", err)
	}
	return postID, nil
}

func (uc *PostUsecase) GetPost(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) (*dto.PostInfo, error) {
	post, err := uc.repo.GetPost(ctx, userID, postID)
	if err != nil {
		return nil, mapRepoError("get post", err)
	}
	return post, nil
}

func (uc *PostUsecase) GetMyNewPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.NewPostsCursor, limit int) (*dto.NewPostsPage, error) {
	if userID == nil {
		return nil, fmt.Errorf("%s: %w", "get my new posts", ErrUnauthorized)
	}
	posts, err := uc.repo.GetMyNewPosts(ctx, userID, cursor, limit+1)
	if err != nil {
		return nil, mapRepoError("get my new posts", err)
	}
	items, next := paginate(posts, limit, func(p *dto.PostPreview) dto.NewPostsCursor {
		return dto.NewPostsCursor{PublishedOrUpdatedAt: *p.PublishedOrUpdatedAt, PostID: p.ID}
	})
	return &dto.NewPostsPage{Items: items, Next: next}, nil
}

func (uc *PostUsecase) GetMyPopularPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.PopularPostsCursor, limit int) (*dto.PopularPostsPage, error) {
	if userID == nil {
		return nil, fmt.Errorf("%s: %w", "get my popular posts", ErrUnauthorized)
	}
	posts, err := uc.repo.GetMyPopularPosts(ctx, userID, cursor, limit+1)
	if err != nil {
		return nil, mapRepoError("get my popular posts", err)
	}
	items, next := paginate(posts, limit, func(p *dto.PostPreview) dto.PopularPostsCursor {
		return dto.PopularPostsCursor{Likes: p.LikesCount, PublishedAt: *p.PublishedOrUpdatedAt, PostID: p.ID}
	})
	return &dto.PopularPostsPage{Items: items, Next: next}, nil
}

func (uc *PostUsecase) GetMyDrafts(ctx context.Context, userID *uuid.UUID, cursor *dto.NewPostsCursor, limit int) (*dto.NewPostsPage, error) {
	if userID == nil {
		return nil, fmt.Errorf("%s: %w", "get my drafts", ErrUnauthorized)
	}
	posts, err := uc.repo.GetMyDrafts(ctx, userID, cursor, limit+1)
	if err != nil {
		return nil, mapRepoError("get my drafts", err)
	}
	items, next := paginate(posts, limit, func(p *dto.PostPreview) dto.NewPostsCursor {
		return dto.NewPostsCursor{PublishedOrUpdatedAt: *p.PublishedOrUpdatedAt, PostID: p.ID}
	})
	return &dto.NewPostsPage{Items: items, Next: next}, nil
}

func (uc *PostUsecase) GetNewPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.NewPostsCursor, limit int) (*dto.NewPostsPage, error) {
	posts, err := uc.repo.GetNewPosts(ctx, userID, cursor, limit+1)
	if err != nil {
		return nil, mapRepoError("get new posts", err)
	}
	items, next := paginate(posts, limit, func(p *dto.PostPreview) dto.NewPostsCursor {
		return dto.NewPostsCursor{PublishedOrUpdatedAt: *p.PublishedOrUpdatedAt, PostID: p.ID}
	})
	return &dto.NewPostsPage{Items: items, Next: next}, nil
}

func (uc *PostUsecase) GetPopularPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.PopularPostsCursor, limit int) (*dto.PopularPostsPage, error) {
	posts, err := uc.repo.GetPopularPosts(ctx, userID, cursor, limit+1)
	if err != nil {
		return nil, mapRepoError("get popular posts", err)
	}
	items, next := paginate(posts, limit, func(p *dto.PostPreview) dto.PopularPostsCursor {
		return dto.PopularPostsCursor{Likes: p.LikesCount, PublishedAt: *p.PublishedOrUpdatedAt, PostID: p.ID}
	})
	return &dto.PopularPostsPage{Items: items, Next: next}, nil
}

func paginate[C any](items []*dto.PostPreview, limit int, cursorOf func(*dto.PostPreview) C) ([]*dto.PostPreview, *C) {
	if len(items) <= limit {
		return items, nil
	}
	items = items[:limit]
	next := cursorOf(items[limit-1])
	return items, &next
}

func (uc *PostUsecase) UpdatePost(ctx context.Context, userID *uuid.UUID, postID uuid.UUID, p dto.PostRequest, lastUpdatedAt time.Time) error {
	if userID == nil {
		return fmt.Errorf("%s: %w", "update post (unauthorized)", ErrNotFound)
	}

	body, err := sanitizeBody(p.Body)
	if err != nil {
		return fmt.Errorf("%s: %w", "update post", err)
	}

	post := models.Post{
		ID:     postID,
		Title:  p.Title,
		Body:   body,
		Status: p.Status,
	}
	if err := uc.repo.UpdatePost(ctx, userID, post, lastUpdatedAt); err != nil {
		return mapRepoError("update post", err)
	}
	return nil
}

func (uc *PostUsecase) DeletePost(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) error {
	if userID == nil {
		return fmt.Errorf("%s: %w", "delete post (unauthorized)", ErrNotFound)
	}
	if err := uc.repo.DeletePost(ctx, userID, postID); err != nil {
		return mapRepoError("delete post", err)
	}
	return nil
}

func (uc *PostUsecase) LikePost(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) (int64, error) {
	if userID == nil {
		return 0, fmt.Errorf("%s: %w", "like post (unauthorized)", ErrNotFound)
	}
	likes, err := uc.repo.LikePost(ctx, userID, postID)
	if err != nil {
		return 0, mapRepoError("like post", err)
	}
	return likes, nil
}

func (uc *PostUsecase) DeletePostLike(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) (int64, error) {
	if userID == nil {
		return 0, fmt.Errorf("%s: %w", "unlike post (unauthorized)", ErrNotFound)
	}
	likes, err := uc.repo.DeletePostLike(ctx, userID, postID)
	if err != nil {
		return 0, err
	}
	return likes, nil
}

func sanitizeBody(body string) (string, error) {
	clean, err := sanitizer.Sanitize(body)
	if err != nil {
		return "", fmt.Errorf("sanitize body: %w: %w", ErrValidation, err)
	}

	if !sanitizer.HasText(clean) {
		return "", ErrValidation
	}

	return clean, nil
}
