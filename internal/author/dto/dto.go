package dto

import (
	"fmt"
	"strings"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/author/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/google/uuid"
)

type AuthorInfo struct {
	ID       uuid.UUID
	Bio      string
	Category *string
}

type UpdateAuthorRequest struct {
	AuthorID uuid.UUID
	Bio      string
	Category *string
}

func (r UpdateAuthorRequest) Normalize() UpdateAuthorRequest {
	r.Bio = strings.TrimSpace(r.Bio)
	if r.Category != nil {
		category := strings.TrimSpace(*r.Category)
		r.Category = &category
	}

	return r
}

func (r UpdateAuthorRequest) Validate() error {
	if r.Bio == "" {
		return fmt.Errorf("%w: invalid bio", validator.ErrValidation)
	}

	if r.Category != nil && *r.Category == "" {
		return fmt.Errorf("%w: invalid category", validator.ErrValidation)
	}

	return nil
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
