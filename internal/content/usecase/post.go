package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/models"
	identitycontracts "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/contracts/identity"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/feed"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/sanitizer"
	"github.com/google/uuid"
)

type (
	getTimedPostsFunc       func(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) ([]*dto.PostPreview, error)
	getPopularPostsFunc     func(ctx context.Context, userID *uuid.UUID, cursor *dto.PopularPostsCursor, limit int) ([]*dto.PostPreview, error)
	getUserTimedPostsFunc   func(ctx context.Context, userID *uuid.UUID, authorID uuid.UUID, cursor *dto.TimedPostsCursor, limit int) ([]*dto.PostPreview, error)
	getUserPopularPostsFunc func(ctx context.Context, userID *uuid.UUID, authorID uuid.UUID, cursor *dto.PopularPostsCursor, limit int) ([]*dto.PostPreview, error)
)

func (uc *ContentUsecase) CreatePost(ctx context.Context, userID *uuid.UUID, p dto.PostRequest) (uuid.UUID, error) {
	if userID == nil {
		return uuid.Nil, fmt.Errorf("%s: %w", "create post", ErrUnauthorized)
	}

	body, searchText, err := uc.prepareBody(p.Body)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%s: %w", "create post", err)
	}

	attachments, err := uc.extractAttachmentIDs(body)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%s: %w", "create post", err)
	}

	post := models.Post{
		Title:       p.Title,
		Body:        body,
		SearchText:  searchText,
		Status:      p.Status,
		Attachments: attachments,
	}
	postID, err := uc.repo.CreatePost(ctx, userID, post)
	if err != nil {
		return uuid.Nil, mapRepoError("create post", err)
	}
	return postID, nil
}

func (uc *ContentUsecase) GetPost(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) (*dto.PostInfo, error) {
	post, err := uc.repo.GetPost(ctx, userID, postID)
	if err != nil {
		return nil, mapRepoError("get post", err)
	}
	authors, err := uc.getPostAuthorsInfo(ctx, post.Author.ID)
	if err != nil {
		return nil, err
	}
	author, ok := authors[post.Author.ID]
	if !ok {
		return nil, fmt.Errorf("%w: author user %s not found", ErrNotFound, post.Author.ID)
	}
	post.Author = *author
	applyAttachmentURLs(post.Attachments)

	return post, nil
}

func (uc *ContentUsecase) GetMyNewPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) (*dto.TimedPostsPage, error) {
	if userID == nil {
		return nil, fmt.Errorf("%s: %w", "get my new posts", ErrUnauthorized)
	}
	page, err := uc.getTimedPosts(ctx, uc.repo.GetMyNewPosts, userID, cursor, limit)
	if err != nil {
		return nil, mapRepoError("get my new posts", err)
	}
	return page, nil
}

func (uc *ContentUsecase) GetMyOldPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) (*dto.TimedPostsPage, error) {
	if userID == nil {
		return nil, fmt.Errorf("%s: %w", "get my old posts", ErrUnauthorized)
	}
	page, err := uc.getTimedPosts(ctx, uc.repo.GetMyOldPosts, userID, cursor, limit)
	if err != nil {
		return nil, mapRepoError("get my old posts", err)
	}
	return page, nil
}

func (uc *ContentUsecase) GetMyPopularPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.PopularPostsCursor, limit int) (*dto.PopularPostsPage, error) {
	if userID == nil {
		return nil, fmt.Errorf("%s: %w", "get my popular posts", ErrUnauthorized)
	}
	page, err := uc.getPopularPosts(ctx, uc.repo.GetMyPopularPosts, userID, cursor, limit)
	if err != nil {
		return nil, mapRepoError("get my popular posts", err)
	}
	return page, nil
}

func (uc *ContentUsecase) GetMyNewDrafts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) (*dto.TimedPostsPage, error) {
	if userID == nil {
		return nil, fmt.Errorf("%s: %w", "get my new drafts", ErrUnauthorized)
	}
	page, err := uc.getTimedPosts(ctx, uc.repo.GetMyNewDrafts, userID, cursor, limit)
	if err != nil {
		return nil, mapRepoError("get my new drafts", err)
	}
	return page, nil
}

func (uc *ContentUsecase) GetMyOldDrafts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) (*dto.TimedPostsPage, error) {
	if userID == nil {
		return nil, fmt.Errorf("%s: %w", "get my old drafts", ErrUnauthorized)
	}
	page, err := uc.getTimedPosts(ctx, uc.repo.GetMyOldDrafts, userID, cursor, limit)
	if err != nil {
		return nil, mapRepoError("get my old drafts", err)
	}
	return page, nil
}

func (uc *ContentUsecase) GetNewPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int, filters dto.FeedFilters) (*dto.TimedPostsPage, error) {
	page, err := uc.getTimedPosts(ctx, func(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) ([]*dto.PostPreview, error) {
		return uc.repo.GetNewPosts(ctx, userID, cursor, limit, filters)
	}, userID, cursor, limit)
	if err != nil {
		return nil, mapRepoError("get new posts", err)
	}
	return page, nil
}

func (uc *ContentUsecase) GetOldPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int, filters dto.FeedFilters) (*dto.TimedPostsPage, error) {
	page, err := uc.getTimedPosts(ctx, func(ctx context.Context, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) ([]*dto.PostPreview, error) {
		return uc.repo.GetOldPosts(ctx, userID, cursor, limit, filters)
	}, userID, cursor, limit)
	if err != nil {
		return nil, mapRepoError("get old posts", err)
	}
	return page, nil
}

func (uc *ContentUsecase) GetPopularPosts(ctx context.Context, userID *uuid.UUID, cursor *dto.PopularPostsCursor, limit int, filters dto.FeedFilters) (*dto.PopularPostsPage, error) {
	page, err := uc.getPopularPosts(ctx, func(ctx context.Context, userID *uuid.UUID, cursor *dto.PopularPostsCursor, limit int) ([]*dto.PostPreview, error) {
		return uc.repo.GetPopularPosts(ctx, userID, cursor, limit, filters)
	}, userID, cursor, limit)
	if err != nil {
		return nil, mapRepoError("get popular posts", err)
	}
	return page, nil
}

func (uc *ContentUsecase) GetUserNewPosts(ctx context.Context, userID *uuid.UUID, authorUsername string, cursor *dto.TimedPostsCursor, limit int) (*dto.TimedPostsPage, error) {
	authorID, err := uc.getAuthorIDByUsername(ctx, authorUsername)
	if err != nil {
		return nil, err
	}

	page, err := uc.getUserTimedPosts(
		ctx,
		uc.repo.GetUserNewPosts,
		userID,
		authorID,
		cursor,
		limit,
	)
	if err != nil {
		return nil, mapRepoError("get user new posts", err)
	}

	return page, nil
}

func (uc *ContentUsecase) GetUserOldPosts(ctx context.Context, userID *uuid.UUID, authorUsername string, cursor *dto.TimedPostsCursor, limit int) (*dto.TimedPostsPage, error) {
	authorID, err := uc.getAuthorIDByUsername(ctx, authorUsername)
	if err != nil {
		return nil, err
	}

	page, err := uc.getUserTimedPosts(
		ctx,
		uc.repo.GetUserOldPosts,
		userID,
		authorID,
		cursor,
		limit,
	)
	if err != nil {
		return nil, mapRepoError("get user old posts", err)
	}

	return page, nil
}

func (uc *ContentUsecase) GetUserPopularPosts(ctx context.Context, userID *uuid.UUID, authorUsername string, cursor *dto.PopularPostsCursor, limit int) (*dto.PopularPostsPage, error) {
	authorID, err := uc.getAuthorIDByUsername(ctx, authorUsername)
	if err != nil {
		return nil, err
	}

	page, err := uc.getUserPopularPosts(
		ctx,
		uc.repo.GetUserPopularPosts,
		userID,
		authorID,
		cursor,
		limit,
	)
	if err != nil {
		return nil, mapRepoError("get user popular posts", err)
	}

	return page, nil
}

func (uc *ContentUsecase) getUserTimedPosts(ctx context.Context, method getUserTimedPostsFunc, userID *uuid.UUID, authorID uuid.UUID, cursor *dto.TimedPostsCursor, limit int) (*dto.TimedPostsPage, error) {
	posts, err := method(ctx, userID, authorID, cursor, limit+1)
	if err != nil {
		return nil, err
	}

	filteredPosts, err := uc.enrichPostsAuthorsInfo(ctx, posts)
	if err != nil {
		return nil, err
	}

	cursorOf := func(p *dto.PostPreview) dto.TimedPostsCursor {
		return dto.TimedPostsCursor{
			PublishedOrUpdatedAt: *p.PublishedOrUpdatedAt,
			PostID:               p.ID,
		}
	}

	page := feed.Paginate(
		filteredPosts,
		limit,
		cursorOf,
	)
	if len(posts) > limit && len(filteredPosts) <= limit {
		next := cursorOf(posts[len(posts)-1])
		page.Next = &next
	}

	return (*dto.TimedPostsPage)(page), nil
}

func (uc *ContentUsecase) getUserPopularPosts(ctx context.Context, method getUserPopularPostsFunc, userID *uuid.UUID, authorID uuid.UUID, cursor *dto.PopularPostsCursor, limit int) (*dto.PopularPostsPage, error) {
	posts, err := method(ctx, userID, authorID, cursor, limit+1)
	if err != nil {
		return nil, err
	}

	filteredPosts, err := uc.enrichPostsAuthorsInfo(ctx, posts)
	if err != nil {
		return nil, err
	}

	cursorOf := func(p *dto.PostPreview) dto.PopularPostsCursor {
		return dto.PopularPostsCursor{
			Likes:       p.LikesCount,
			PublishedAt: *p.PublishedOrUpdatedAt,
			PostID:      p.ID,
		}
	}

	page := feed.Paginate(
		filteredPosts,
		limit,
		cursorOf,
	)
	if len(posts) > limit && len(filteredPosts) <= limit {
		next := cursorOf(posts[len(posts)-1])
		page.Next = &next
	}

	return (*dto.PopularPostsPage)(page), nil
}

func (uc *ContentUsecase) getTimedPosts(ctx context.Context, method getTimedPostsFunc, userID *uuid.UUID, cursor *dto.TimedPostsCursor, limit int) (*dto.TimedPostsPage, error) {
	posts, err := method(ctx, userID, cursor, limit+1)
	if err != nil {
		return nil, err
	}

	filteredPosts, err := uc.enrichPostsAuthorsInfo(ctx, posts)
	if err != nil {
		return nil, err
	}

	cursorOf := func(p *dto.PostPreview) dto.TimedPostsCursor {
		return dto.TimedPostsCursor{
			PublishedOrUpdatedAt: *p.PublishedOrUpdatedAt,
			PostID:               p.ID,
		}
	}

	page := feed.Paginate(
		filteredPosts,
		limit,
		cursorOf,
	)
	if len(posts) > limit && len(filteredPosts) <= limit {
		next := cursorOf(posts[len(posts)-1])
		page.Next = &next
	}

	return (*dto.TimedPostsPage)(page), nil
}

func (uc *ContentUsecase) getPopularPosts(ctx context.Context, method getPopularPostsFunc, userID *uuid.UUID, cursor *dto.PopularPostsCursor, limit int) (*dto.PopularPostsPage, error) {
	posts, err := method(ctx, userID, cursor, limit+1)
	if err != nil {
		return nil, err
	}

	filteredPosts, err := uc.enrichPostsAuthorsInfo(ctx, posts)
	if err != nil {
		return nil, err
	}

	cursorOf := func(p *dto.PostPreview) dto.PopularPostsCursor {
		return dto.PopularPostsCursor{
			Likes:       p.LikesCount,
			PublishedAt: *p.PublishedOrUpdatedAt,
			PostID:      p.ID,
		}
	}

	page := feed.Paginate(
		filteredPosts,
		limit,
		cursorOf,
	)
	if len(posts) > limit && len(filteredPosts) <= limit {
		next := cursorOf(posts[len(posts)-1])
		page.Next = &next
	}

	return (*dto.PopularPostsPage)(page), nil
}

func (uc *ContentUsecase) UpdatePost(ctx context.Context, userID *uuid.UUID, postID uuid.UUID, p dto.PostRequest, lastUpdatedAt time.Time) error {
	if userID == nil {
		return fmt.Errorf("%s: %w", "update post (unauthorized)", ErrUnauthorized)
	}

	body, searchText, err := uc.prepareBody(p.Body)
	if err != nil {
		return fmt.Errorf("%s: %w", "update post", err)
	}

	attachments, err := uc.extractAttachmentIDs(body)
	if err != nil {
		return fmt.Errorf("%s: %w", "update post", err)
	}

	post := models.Post{
		ID:          postID,
		Title:       p.Title,
		Body:        body,
		SearchText:  searchText,
		Status:      p.Status,
		Attachments: attachments,
	}
	keys, err := uc.repo.UpdatePost(ctx, userID, post, lastUpdatedAt)
	if err != nil {
		return mapRepoError("update post", err)
	}
	uc.deleteDetachedMedia(ctx, keys)

	return nil
}

func (uc *ContentUsecase) DeletePost(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) error {
	if userID == nil {
		return fmt.Errorf("%s: %w", "delete post (unauthorized)", ErrUnauthorized)
	}
	keys, err := uc.repo.DeletePost(ctx, userID, postID)
	if err != nil {
		return mapRepoError("delete post", err)
	}
	uc.deleteDetachedMedia(ctx, keys)

	return nil
}

func (uc *ContentUsecase) LikePost(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) (int64, error) {
	if userID == nil {
		return 0, fmt.Errorf("%s: %w", "like post (unauthorized)", ErrUnauthorized)
	}
	likes, err := uc.repo.LikePost(ctx, userID, postID)
	if err != nil {
		return 0, mapRepoError("like post", err)
	}
	return likes, nil
}

func (uc *ContentUsecase) DeletePostLike(ctx context.Context, userID *uuid.UUID, postID uuid.UUID) (int64, error) {
	if userID == nil {
		return 0, fmt.Errorf("%s: %w", "unlike post (unauthorized)", ErrUnauthorized)
	}
	likes, err := uc.repo.DeletePostLike(ctx, userID, postID)
	if err != nil {
		return 0, mapRepoError("delete like", err)
	}
	return likes, nil
}

func (uc *ContentUsecase) prepareBody(raw string) (string, string, error) {
	body, err := uc.sanitizeBody(raw)
	if err != nil {
		return "", "", err
	}

	searchText, err := uc.sanitizer.PlainText(body)
	if err != nil {
		return "", "", fmt.Errorf("sanitize body: %w", ErrValidation)
	}

	return body, searchText, nil
}

func (uc *ContentUsecase) sanitizeBody(body string) (string, error) {
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

// extractAttachmentIDs вынимает id вложений из маркеров санитизированного body
func (uc *ContentUsecase) extractAttachmentIDs(body string) ([]uuid.UUID, error) {
	attachments, err := uc.sanitizer.AttachmentIDs(body)
	if err != nil {
		return nil, fmt.Errorf("extract attachments: %w: %w", ErrValidation, err)
	}

	if len(attachments) > uc.attLimits.MaxPostAttachments() {
		return nil, fmt.Errorf("extract attachments: %w: too many attachments", ErrValidation)
	}

	return attachments, nil
}

// applyAttachmentURLs проставляет вложениям маршрут выдачи файла
func applyAttachmentURLs(attachments []*dto.Attachment) {
	for _, attachment := range attachments {
		attachment.URL = attachmentURLPrefix + "/" + attachment.ID.String()
	}
}

func (uc *ContentUsecase) enrichPostsAuthorsInfo(ctx context.Context, posts []*dto.PostPreview) ([]*dto.PostPreview, error) {
	if len(posts) == 0 {
		return []*dto.PostPreview{}, nil
	}

	authorIDs := make([]uuid.UUID, 0, len(posts))
	filteredPosts := make([]*dto.PostPreview, 0, len(posts))
	seen := make(map[uuid.UUID]struct{}, len(posts))

	for _, post := range posts {
		if post == nil {
			uc.logger.WarnContext(ctx, "enrich posts authors: nil post")
			continue
		}

		authorID := post.Author.ID

		if authorID == uuid.Nil {
			uc.logger.WarnContext(ctx, "enrich posts authors: empty author ID", "post_id", post.ID)
			continue
		}

		filteredPosts = append(filteredPosts, post)

		if _, exists := seen[authorID]; exists {
			continue
		}

		seen[authorID] = struct{}{}
		authorIDs = append(authorIDs, authorID)
	}

	authors, err := uc.getPostAuthorsInfo(ctx, authorIDs...)
	if err != nil {
		return nil, fmt.Errorf("enrich posts authors: %w", err)
	}

	result := make([]*dto.PostPreview, 0, len(filteredPosts))
	for _, post := range filteredPosts {
		author, ok := authors[post.Author.ID]
		if !ok {
			uc.logger.WarnContext(ctx, "enrich posts authors: author not found", "author_id", post.Author.ID)
			continue
		}

		post.Author = *author
		result = append(result, post)
	}

	return result, nil
}

func (uc *ContentUsecase) getPostAuthorsInfo(ctx context.Context, ids ...uuid.UUID) (map[uuid.UUID]*dto.AuthorResponse, error) {
	result := make(map[uuid.UUID]*dto.AuthorResponse, len(ids))
	if len(ids) == 0 {
		return result, nil
	}

	users, err := uc.identity.GetUsers(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("get post authors user info: %w", err)
	}

	for _, id := range ids {
		user, ok := users[id]
		if !ok {
			continue
		}

		result[id] = &dto.AuthorResponse{
			ID:        user.ID,
			Username:  user.Username,
			Nickname:  user.Nickname,
			AvatarUrl: emptyStringToNil(user.AvatarURL),
		}
	}

	return result, nil
}

func (uc *ContentUsecase) getAuthorIDByUsername(ctx context.Context, username string) (uuid.UUID, error) {
	user, err := uc.identity.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, identitycontracts.ErrUserNotFound) {
			return uuid.Nil, fmt.Errorf("get user by username: %w", ErrNotFound)
		}
		if errors.Is(err, identitycontracts.ErrInvalidUsername) {
			return uuid.Nil, fmt.Errorf("get user by username: %w", ErrValidation)
		}

		return uuid.Nil, fmt.Errorf("get user by username: %w", err)
	}

	if user.ID == uuid.Nil {
		return uuid.Nil, fmt.Errorf("get user by username: identity returned empty user ID for username %s", username)
	}

	return user.ID, nil
}

func emptyStringToNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
