package dto

import (
	"fmt"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/feed"
	"github.com/google/uuid"
)

type PostRequest struct {
	Title         string     `json:"title" example:"Мой первый пост"`
	Body          string     `json:"body" example:"Текст поста"`
	Status        string     `json:"status" enums:"draft,published" example:"published"`
	LastUpdatedAt *time.Time `json:"last_updated_at,omitempty" example:"2026-11-02T12:00:00Z"`
}

func (r PostRequest) Validate(v *validator.Validator) error {
	var valError string
	switch {
	case !v.IsValidPostTitle(r.Title):
		valError += "invalid title:"
	case !v.IsValidPostBody(r.Body):
		valError += "invalid body: "
	case !v.IsValidPostStatus(r.Status):
		valError += "invalid status: "
	}
	if valError != "" {
		return fmt.Errorf("%s%w", valError, validator.ErrValidation)
	}
	return nil
}

type CreatePostResponse struct {
	ID uuid.UUID `json:"id" example:"00000000-0000-4000-8000-000000000101"`
}

type AuthorResponse struct {
	ID        uuid.UUID `json:"id" example:"00000000-0000-4000-8000-000000000001"`
	Username  string    `json:"username" example:"username123"`
	Nickname  string    `json:"nickname" example:"nickname_123"`
	AvatarUrl *string   `json:"avatar_url" example:"avatars/user-1.png"`
}

type PostInfo struct {
	ID          uuid.UUID      `json:"id" example:"00000000-0000-4000-8000-000000000101"`
	Author      AuthorResponse `json:"author"`
	Title       string         `json:"title" example:"Мой первый пост"`
	Body        string         `json:"body" example:"Текст поста"`
	Status      string         `json:"status" example:"published"`
	PublishedAt *time.Time     `json:"published_at" example:"2026-09-01T12:00:00Z"`
	CreatedAt   time.Time      `json:"created_at" example:"2026-09-01T11:00:00Z"`
	UpdatedAt   time.Time      `json:"updated_at" example:"2026-09-01T12:00:00Z"`
	Locked      bool           `json:"locked" example:"false"`
	LikesCount  int64          `json:"likes_count" example:"12"`
	LikedByUser bool           `json:"liked_by_user" example:"true"`
}

type PostPreview struct {
	ID                   uuid.UUID      `json:"id" example:"00000000-0000-4000-8000-000000000101"`
	Author               AuthorResponse `json:"author"`
	Title                string         `json:"title" example:"Мой первый пост"`
	BodyPreview          string         `json:"body_preview" example:"Первые 200 символов текста поста"`
	PublishedOrUpdatedAt *time.Time     `json:"published_or_updated_at" example:"2026-09-01T12:00:00Z"`
	Locked               bool           `json:"locked" example:"false"`
	LikesCount           int64          `json:"likes_count" example:"12"`
	LikedByUser          bool           `json:"liked_by_user" example:"true"`
}

type PostsPageResponse struct {
	Items      []*PostPreview `json:"items"`
	NextCursor *string        `json:"next_cursor" example:"eyJraW5kIjoibmV3X3Bvc3RzIn0"`
}

type TimedPostsCursor struct {
	PublishedOrUpdatedAt time.Time `json:"published_or_updated_at"`
	PostID               uuid.UUID `json:"post_id"`
}

type PopularPostsCursor struct {
	Likes       int64     `json:"likes"`
	PublishedAt time.Time `json:"published_at"`
	PostID      uuid.UUID `json:"post_id"`
}

type (
	TimedPostsPage   feed.Page[*PostPreview, TimedPostsCursor]
	PopularPostsPage feed.Page[*PostPreview, PopularPostsCursor]
)

type PostLikeResponse struct {
	LikesCount  int64 `json:"likes_count" example:"13"`
	LikedByUser bool  `json:"liked_by_user" example:"true"`
}
