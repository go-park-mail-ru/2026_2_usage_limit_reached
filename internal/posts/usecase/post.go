package postusecase

import (
	"context"
	"fmt"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/feed"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/mediaurl"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/sanitizer"
	"github.com/google/uuid"
)

type (
	getTimedPostsFunc       func(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) ([]*dto.PostPreview, error)
	getPopularPostsFunc     func(ctx context.Context, userID *uuid.UUID, cursor *dto.PopularPostsCursor, limit int) ([]*dto.PostPreview, error)
	getUserTimedPostsFunc   func(ctx context.Context, userID *uuid.UUID, username string, cursor *dto.TimedPostsCursor, limit int) ([]*dto.PostPreview, error)
	getUserPopularPostsFunc func(ctx context.Context, userID *uuid.UUID, username string, cursor *dto.PopularPostsCursor, limit int) ([]*dto.PostPreview, error)
)

func (uc *PostUsecase) CreatePost(ctx context.Context, userID *uuid.UUID, p dto.PostRequest) (uuid.UUID, error) {
	if userID == nil {
		return uuid.Nil, fmt.Errorf("%s: %w", "create post", ErrUnauthorized)
	}

	body, err := uc.sanitizeBody(p.Body)
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
	post.Author.AvatarUrl = mediaurl.PublicPtr(uc.avatarBasePath, post.Author.AvatarUrl)
	return post, nil
}

func (uc *PostUsecase) GetMyNewPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) (*dto.TimedPostsPage, error) {
	if userID == nil {
		return nil, fmt.Errorf("%s: %w", "get my new posts", ErrUnauthorized)
	}
	page, err := uc.getTimedPosts(ctx, uc.repo.GetMyNewPosts, userID, cursor, limit)
	if err != nil {
		return nil, mapRepoError("get my new posts", err)
	}
	return page, nil
}

func (uc *PostUsecase) GetMyOldPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) (*dto.TimedPostsPage, error) {
	if userID == nil {
		return nil, fmt.Errorf("%s: %w", "get my old posts", ErrUnauthorized)
	}
	page, err := uc.getTimedPosts(ctx, uc.repo.GetMyOldPosts, userID, cursor, limit)
	if err != nil {
		return nil, mapRepoError("get my old posts", err)
	}
	return page, nil
}

func (uc *PostUsecase) GetMyPopularPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.PopularPostsCursor, limit int) (*dto.PopularPostsPage, error) {
	if userID == nil {
		return nil, fmt.Errorf("%s: %w", "get my popular posts", ErrUnauthorized)
	}
	page, err := uc.getPopularPosts(ctx, uc.repo.GetMyPopularPosts, userID, cursor, limit)
	if err != nil {
		return nil, mapRepoError("get my popular posts", err)
	}
	return page, nil
}

func (uc *PostUsecase) GetMyNewDrafts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) (*dto.TimedPostsPage, error) {
	if userID == nil {
		return nil, fmt.Errorf("%s: %w", "get my new drafts", ErrUnauthorized)
	}
	page, err := uc.getTimedPosts(ctx, uc.repo.GetMyNewDrafts, userID, cursor, limit)
	if err != nil {
		return nil, mapRepoError("get my new drafts", err)
	}
	return page, nil
}

func (uc *PostUsecase) GetMyOldDrafts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) (*dto.TimedPostsPage, error) {
	if userID == nil {
		return nil, fmt.Errorf("%s: %w", "get my old drafts", ErrUnauthorized)
	}
	page, err := uc.getTimedPosts(ctx, uc.repo.GetMyOldDrafts, userID, cursor, limit)
	if err != nil {
		return nil, mapRepoError("get my old drafts", err)
	}
	return page, nil
}

func (uc *PostUsecase) GetNewPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) (*dto.TimedPostsPage, error) {
	page, err := uc.getTimedPosts(ctx, uc.repo.GetNewPosts, userID, cursor, limit)
	if err != nil {
		return nil, mapRepoError("get new posts", err)
	}
	return page, nil
}

func (uc *PostUsecase) GetOldPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) (*dto.TimedPostsPage, error) {
	page, err := uc.getTimedPosts(ctx, uc.repo.GetOldPosts, userID, cursor, limit)
	if err != nil {
		return nil, mapRepoError("get old posts", err)
	}
	return page, nil
}

func (uc *PostUsecase) GetPopularPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.PopularPostsCursor, limit int) (*dto.PopularPostsPage, error) {
	page, err := uc.getPopularPosts(ctx, uc.repo.GetPopularPosts, userID, cursor, limit)
	if err != nil {
		return nil, mapRepoError("get popular posts", err)
	}
	return page, nil
}

func (uc *PostUsecase) GetUserNewPosts(ctx context.Context, userID *uuid.UUID, authorUsername string, cursor *dto.TimedPostsCursor, limit int) (*dto.TimedPostsPage, error) {
	page, err := uc.getUserTimedPosts(ctx, uc.repo.GetUserNewPosts, userID, authorUsername, cursor, limit)
	if err != nil {
		return nil, mapRepoError("get user new posts", err)
	}
	return page, nil
}

func (uc *PostUsecase) GetUserOldPosts(ctx context.Context, userID *uuid.UUID, authorUsername string, cursor *dto.TimedPostsCursor, limit int) (*dto.TimedPostsPage, error) {
	page, err := uc.getUserTimedPosts(ctx, uc.repo.GetUserOldPosts, userID, authorUsername, cursor, limit)
	if err != nil {
		return nil, mapRepoError("get user old posts", err)
	}
	return page, nil
}

func (uc *PostUsecase) GetUserPopularPosts(ctx context.Context, userID *uuid.UUID, authorUsername string, cursor *dto.PopularPostsCursor, limit int) (*dto.PopularPostsPage, error) {
	page, err := uc.getUserPopularPosts(ctx, uc.repo.GetUserPopularPosts, userID, authorUsername, cursor, limit)
	if err != nil {
		return nil, mapRepoError("get user popular posts", err)
	}
	return page, nil
}

func (uc *PostUsecase) getUserTimedPosts(ctx context.Context, method getUserTimedPostsFunc, userID *uuid.UUID, username string, cursor *dto.TimedPostsCursor, limit int) (*dto.TimedPostsPage, error) {
	posts, err := method(ctx, userID, username, cursor, limit+1)
	if err != nil {
		return nil, err
	}
	page := feed.Paginate(posts, limit, func(p *dto.PostPreview) dto.TimedPostsCursor {
		return dto.TimedPostsCursor{PublishedOrUpdatedAt: *p.PublishedOrUpdatedAt, PostID: p.ID}
	})
	return (*dto.TimedPostsPage)(page), nil
}

func (uc *PostUsecase) getUserPopularPosts(ctx context.Context, method getUserPopularPostsFunc, userID *uuid.UUID, username string, cursor *dto.PopularPostsCursor, limit int) (*dto.PopularPostsPage, error) {
	posts, err := method(ctx, userID, username, cursor, limit+1)
	if err != nil {
		return nil, err
	}
	page := feed.Paginate(posts, limit, func(p *dto.PostPreview) dto.PopularPostsCursor {
		return dto.PopularPostsCursor{Likes: p.LikesCount, PublishedAt: *p.PublishedOrUpdatedAt, PostID: p.ID}
	})
	return (*dto.PopularPostsPage)(page), nil
}

func (uc *PostUsecase) getTimedPosts(ctx context.Context, method getTimedPostsFunc, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) (*dto.TimedPostsPage, error) {
	posts, err := method(ctx, userID, cursor, limit+1)
	if err != nil {
		return nil, err
	}
	for i := range posts {
		posts[i].Author.AvatarUrl = mediaurl.PublicPtr(uc.avatarBasePath, posts[i].Author.AvatarUrl)
	}
	page := feed.Paginate(posts, limit, func(p *dto.PostPreview) dto.TimedPostsCursor {
		return dto.TimedPostsCursor{PublishedOrUpdatedAt: *p.PublishedOrUpdatedAt, PostID: p.ID}
	})
	return (*dto.TimedPostsPage)(page), nil
}

func (uc *PostUsecase) getPopularPosts(ctx context.Context, method getPopularPostsFunc, userID *uuid.UUID, cursor *dto.PopularPostsCursor, limit int) (*dto.PopularPostsPage, error) {
	posts, err := method(ctx, userID, cursor, limit+1)
	if err != nil {
		return nil, err
	}
	for i := range posts {
		posts[i].Author.AvatarUrl = mediaurl.PublicPtr(uc.avatarBasePath, posts[i].Author.AvatarUrl)
	}
	page := feed.Paginate(posts, limit, func(p *dto.PostPreview) dto.PopularPostsCursor {
		return dto.PopularPostsCursor{Likes: p.LikesCount, PublishedAt: *p.PublishedOrUpdatedAt, PostID: p.ID}
	})
	return (*dto.PopularPostsPage)(page), nil
}

func (uc *PostUsecase) UpdatePost(ctx context.Context, userID *uuid.UUID, postID uuid.UUID, p dto.PostRequest, lastUpdatedAt time.Time) error {
	if userID == nil {
		return fmt.Errorf("%s: %w", "update post (unauthorized)", ErrUnauthorized)
	}

	body, err := uc.sanitizeBody(p.Body)
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
		return fmt.Errorf("%s: %w", "delete post (unauthorized)", ErrUnauthorized)
	}
	if err := uc.repo.DeletePost(ctx, userID, postID); err != nil {
		return mapRepoError("delete post", err)
	}
	return nil
}

func (uc *PostUsecase) LikePost(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) (int64, error) {
	if userID == nil {
		return 0, fmt.Errorf("%s: %w", "like post (unauthorized)", ErrUnauthorized)
	}
	likes, err := uc.repo.LikePost(ctx, userID, postID)
	if err != nil {
		return 0, mapRepoError("like post", err)
	}
	return likes, nil
}

func (uc *PostUsecase) DeletePostLike(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) (int64, error) {
	if userID == nil {
		return 0, fmt.Errorf("%s: %w", "unlike post (unauthorized)", ErrUnauthorized)
	}
	likes, err := uc.repo.DeletePostLike(ctx, userID, postID)
	if err != nil {
		return 0, mapRepoError("delete like", err)
	}
	return likes, nil
}

func (uc *PostUsecase) sanitizeBody(body string) (string, error) {
	clean, err := uc.sanitizer.Sanitize(body)
	if err != nil {
		return "", fmt.Errorf("sanitize body: %w: %w", ErrValidation, err)
	}

	if !uc.bodyCheck.IsValidSanitizedPostBody(body, clean) {
		return "", fmt.Errorf("sanitize body: %w: sanitized body is too large", ErrValidation)
	}

	if !sanitizer.HasText(clean) {
		return "", fmt.Errorf("sanitize body: %w: no text", ErrValidation)
	}

	return clean, nil
}
