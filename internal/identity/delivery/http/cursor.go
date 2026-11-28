package handlers

import (
	"github.com/google/uuid"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/cursor"
)

const typeAuthorSearch cursor.CursorType = "author_search"

var AuthorSearchCursor = cursor.Codec[dto.AuthorSearchCursor]{
	Kind: typeAuthorSearch,
	Valid: func(c *dto.AuthorSearchCursor) bool {
		return c.ID != uuid.Nil && c.Nickname != ""
	},
}
