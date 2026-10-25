package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/models"
)

type PostInfo struct {
	models.Post
	AuthorUsername  string
	AuthorNickname  string
	AuthorAvatarKey *string
	Locked          bool
	LikesCount      int64
	LikedByUser     bool
}

type NewPostsCursor struct {
	PublishedAt time.Time
	PostID      uuid.UUID
}

type PopularPostsCursor struct {
	Likes       int64
	PublishedAt time.Time
	PostID      uuid.UUID
}
