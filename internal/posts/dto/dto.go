package dto

import (
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/models"
)

type PostInfo struct {
	models.Post
	AuthorUsername  string
	AuthorNickname  string
	AuthorAvatarKey *string
	Locked          bool
	LikesCount      int
	LikedByUser     bool
}
