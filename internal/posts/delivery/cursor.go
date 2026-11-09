package posthandlers

import (
	"github.com/google/uuid"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/cursor"
)

const (
	typeTimedPosts   cursor.CursorType = "timed_posts"
	typePopularPosts cursor.CursorType = "popular_posts"
)

var (
	TimedPostsCursor = cursor.Codec[dto.TimedPostsCursor]{
		Kind: typeTimedPosts,
		Valid: func(c *dto.TimedPostsCursor) bool {
			return !c.PublishedOrUpdatedAt.IsZero() && c.PostID != uuid.Nil
		},
	}
	PopularPostsCursor = cursor.Codec[dto.PopularPostsCursor]{
		Kind: typePopularPosts,
		Valid: func(c *dto.PopularPostsCursor) bool {
			return c.Likes >= 0 && !c.PublishedAt.IsZero() && c.PostID != uuid.Nil
		},
	}
)
