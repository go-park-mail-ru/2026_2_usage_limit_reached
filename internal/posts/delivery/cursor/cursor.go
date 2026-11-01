package cursor

import (
	"github.com/google/uuid"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/cursor"
)

const (
	typeNewPosts     cursor.CursorType = "new_posts"
	typePopularPosts cursor.CursorType = "popular_posts"
)

var (
	NewPosts = cursor.Codec[dto.NewPostsCursor]{
		Kind: typeNewPosts,
		Valid: func(c *dto.NewPostsCursor) bool {
			return !c.PublishedOrUpdatedAt.IsZero() && c.PostID != uuid.Nil
		},
	}
	PopularPosts = cursor.Codec[dto.PopularPostsCursor]{
		Kind: typePopularPosts,
		Valid: func(c *dto.PopularPostsCursor) bool {
			return c.Likes >= 0 && !c.PublishedAt.IsZero() && c.PostID != uuid.Nil
		},
	}
)
