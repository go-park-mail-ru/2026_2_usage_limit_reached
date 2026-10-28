package dto

import (
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/author/models"
	"github.com/google/uuid"
)

type AuthorInfo struct {
	ID              uuid.UUID
	Bio             string
	Category        *string
	PayoutProvider  *string
	PayoutAccountID *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func ToAuthorInfo(author *models.Author) *AuthorInfo {
	if author == nil {
		return nil
	}
	return &AuthorInfo{
		ID:              author.ID,
		Bio:             author.Bio,
		Category:        author.Category,
		PayoutProvider:  author.PayoutProvider,
		PayoutAccountID: author.PayoutAccountID,
		CreatedAt:       author.CreatedAt,
		UpdatedAt:       author.UpdatedAt,
	}
}
