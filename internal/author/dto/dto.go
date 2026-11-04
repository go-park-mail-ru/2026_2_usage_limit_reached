package dto

import (
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/author/models"
	"github.com/google/uuid"
)

type AuthorInfo struct {
	ID       uuid.UUID
	Bio      string
	Category *string
}

func ToAuthorInfo(author *models.Author) *AuthorInfo {
	if author == nil {
		return nil
	}

	return &AuthorInfo{
		ID:       author.ID,
		Bio:      author.Bio,
		Category: author.Category,
	}
}
