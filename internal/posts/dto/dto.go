package dto

import (
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/validator"
	"github.com/google/uuid"
)

type PostRequest struct {
	Title         string     `json:"title"`
	Body          string     `json:"body"`
	Status        string     `json:"status"`
	LastUpdatedAt *time.Time `json:"last_updated_at,omitempty"`
}

func (r PostRequest) Validate(v *validator.Validator) error {
	if !v.IsValidPostTitle(r.Title) || !v.IsValidPostBody(r.Body) || !v.IsValidPostStatus(r.Status) {
		return validator.ErrValidation
	}
	return nil
}

type AuthorResponse struct {
	ID        uuid.UUID `json:"id"`
	Username  string    `json:"username"`
	Nickname  string    `json:"nickname"`
	AvatarKey *string   `json:"avatar_key"`
}

type PostInfo struct {
	ID          uuid.UUID      `json:"id"`
	Author      AuthorResponse `json:"author"`
	Title       string         `json:"title"`
	Body        string         `json:"body"`
	Status      string         `json:"status"`
	PublishedAt *time.Time     `json:"published_at"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	Locked      bool           `json:"locked"`
	LikesCount  int64          `json:"likes_count"`
	LikedByUser bool           `json:"liked_by_user"`
}

type PostPreview struct {
	ID          uuid.UUID      `json:"id"`
	Author      AuthorResponse `json:"author"`
	Title       string         `json:"title"`
	BodyPreview string         `json:"body_preview"`
	PublishedAt *time.Time     `json:"published_at"`
	Locked      bool           `json:"locked"`
	LikesCount  int64          `json:"likes_count"`
	LikedByUser bool           `json:"liked_by_user"`
}

type PostsPageResponse struct {
	Items      []*PostPreview `json:"items"`
	NextCursor *string        `json:"next_cursor"`
}

type NewPostsCursor struct {
	PublishedAt time.Time `json:"published_at"`
	PostID      uuid.UUID `json:"post_id"`
}

type PopularPostsCursor struct {
	Likes       int64     `json:"likes"`
	PublishedAt time.Time `json:"published_at"`
	PostID      uuid.UUID `json:"post_id"`
}

type NewPostsPage struct {
	Items []*PostPreview
	Next  *NewPostsCursor
}

type PopularPostsPage struct {
	Items []*PostPreview
	Next  *PopularPostsCursor
}

type PostLikeResponse struct {
	LikesCount  int64 `json:"likes_count"`
	LikedByUser bool  `json:"liked_by_user"`
}
