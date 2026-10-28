package postusecase

import (
	"context"
	"fmt"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/models"
	"github.com/google/uuid"
)

func (uc *PostUsecase) CreatePost(ctx context.Context, userID uuid.UUID, p dto.PostRequest) error {
	post := models.Post{
		Title:  p.Title,
		Body:   p.Body,
		Status: p.Status,
	}
	if err := uc.repo.CreatePost(ctx, userID, post); err != nil {
		return mapRepoError("create post", err)
	}
	return nil
}

func (uc *PostUsecase) GetPost(ctx context.Context, userID, postID uuid.UUID) (*dto.PostInfo, error) {
	post, err := uc.repo.GetPost(ctx, userID, postID)
	if err != nil {
		return nil, mapRepoError("get post", err)
	}
	return post, nil
}

func (uc *PostUsecase) GetNewPosts(ctx context.Context, userID uuid.UUID, cursor *dto.NewPostsCursor, limit int) (*dto.NewPostsPage, error) {
	posts, err := uc.repo.GetNewPosts(ctx, userID, cursor, limit+1)
	if err != nil {
		return nil, fmt.Errorf("failed to get new posts: %w", err)
	}
	items, next := paginate(posts, limit, func(p *dto.PostPreview) dto.NewPostsCursor {
		return dto.NewPostsCursor{PublishedAt: *p.PublishedAt, PostID: p.ID}
	})
	return &dto.NewPostsPage{Items: items, Next: next}, nil
}

func (uc *PostUsecase) GetPopularPosts(ctx context.Context, userID uuid.UUID, cursor *dto.PopularPostsCursor, limit int) (*dto.PopularPostsPage, error) {
	posts, err := uc.repo.GetPopularPosts(ctx, userID, cursor, limit+1)
	if err != nil {
		return nil, fmt.Errorf("failed to get popular posts: %w", err)
	}
	items, next := paginate(posts, limit, func(p *dto.PostPreview) dto.PopularPostsCursor {
		return dto.PopularPostsCursor{Likes: p.LikesCount, PublishedAt: *p.PublishedAt, PostID: p.ID}
	})
	return &dto.PopularPostsPage{Items: items, Next: next}, nil
}

func (uc *PostUsecase) ListPostsByAuthor(ctx context.Context, authorID uuid.UUID) ([]*dto.PostInfo, error) {
	posts, err := uc.repo.ListPostsByAuthor(ctx, authorID)
	if err != nil {
		return nil, mapRepoError("list author posts", err)
	}
	return posts, nil
}

func paginate[C any](items []*dto.PostPreview, limit int, cursorOf func(*dto.PostPreview) C) ([]*dto.PostPreview, *C) {
	if len(items) <= limit {
		return items, nil
	}
	items = items[:limit]
	next := cursorOf(items[limit-1])
	return items, &next
}

func (uc *PostUsecase) UpdatePost(ctx context.Context, userID, postID uuid.UUID, p dto.PostRequest, lastUpdatedAt time.Time) error {
	post := models.Post{
		ID:     postID,
		Title:  p.Title,
		Body:   p.Body,
		Status: p.Status,
	}
	if err := uc.repo.UpdatePost(ctx, userID, post, lastUpdatedAt); err != nil {
		return mapRepoError("update post", err)
	}
	return nil
}

func (uc *PostUsecase) DeletePost(ctx context.Context, userID, postID uuid.UUID) error {
	if err := uc.repo.DeletePost(ctx, userID, postID); err != nil {
		return mapRepoError("delete post", err)
	}
	return nil
}

func (uc *PostUsecase) LikePost(ctx context.Context, userID, postID uuid.UUID) (int64, error) {
	likes, err := uc.repo.LikePost(ctx, userID, postID)
	if err != nil {
		return 0, mapRepoError("like post", err)
	}
	return likes, nil
}

func (uc *PostUsecase) DeletePostLike(ctx context.Context, userID, postID uuid.UUID) (int64, error) {
	likes, err := uc.repo.DeletePostLike(ctx, userID, postID)
	if err != nil {
		return 0, err
	}
	return likes, nil
}
