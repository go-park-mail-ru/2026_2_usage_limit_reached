package dto

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/identity/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/google/uuid"
)

type AuthorSearchItem struct {
	ID        uuid.UUID `json:"-"`
	Username  string    `json:"username" example:"username123"`
	Nickname  string    `json:"nickname" example:"nickname_123"`
	AvatarURL *string   `json:"avatar_url" example:"/media/avatars/00000000-0000-4000-8000-000000000001/00000000-0000-4000-8000-000000000201.png"`
	Bio       string    `json:"bio" example:"i am author"`
}

type AuthorSearchRequest struct {
	Query      string
	Categories []string
	Cursor     *AuthorSearchCursor
}

func (r AuthorSearchRequest) Validate(v *validator.Validator) error {
	errs := make([]error, 0)
	query := strings.TrimSpace(r.Query)

	if query == "" && len(r.Categories) == 0 {
		errs = append(errs, fmt.Errorf("query or categories required"))
	}
	if !v.IsValidSearchQuery(query) {
		errs = append(errs, fmt.Errorf("invalid query"))
	}
	if len(r.Categories) > v.MaxSearchCategories() {
		errs = append(errs, fmt.Errorf("too many categories"))
	}
	for _, category := range r.Categories {
		if !v.IsValidCategory(category) || !v.IsAllowedCategory(category) {
			errs = append(errs, fmt.Errorf("invalid category"))
		}
	}

	if len(errs) != 0 {
		errs = append(errs, validator.ErrValidation)
		return errors.Join(errs...)
	}

	return nil
}

type AuthorSearchCursor struct {
	Nickname string    `json:"nickname"`
	ID       uuid.UUID `json:"id"`
}

type AuthorSearchPage struct {
	Items []*AuthorSearchItem
	Next  *AuthorSearchCursor
}

type AuthorSearchResponse struct {
	Items      []*AuthorSearchItem `json:"items"`
	NextCursor *string             `json:"next_cursor"`
}

type AuthorCreateRequest struct {
	Bio      string  `json:"bio" example:"Пишу о книгах"`
	Category *string `json:"category,omitempty" example:"Творчество"`
}

func (r AuthorCreateRequest) Validate(v *validator.Validator) error {
	errs := make([]error, 0)
	if !v.IsValidBio(r.Bio) {
		errs = append(errs, fmt.Errorf("invalid bio"))
	}
	if r.Category != nil && !v.IsAllowedCategory(*r.Category) {
		errs = append(errs, fmt.Errorf("invalid category"))
	}

	if len(errs) != 0 {
		errs = append(errs, validator.ErrValidation)
		return errors.Join(errs...)
	}

	return nil
}

type AuthorMeResponse struct {
	ID       uuid.UUID `json:"id" example:"00000000-0000-4000-8000-000000000001"`
	Username string    `json:"username" example:"username123"`
	Nickname string    `json:"nickname" example:"nickname_123"`
	Bio      string    `json:"bio" example:"Пишу о книгах"`
	Category *string   `json:"category" example:"Творчество"`
}

func ToAuthorSearchItem(author *models.Author, avatarURL *string) *AuthorSearchItem {
	if author == nil {
		return nil
	}

	return &AuthorSearchItem{
		ID:        author.ID,
		Username:  author.Username,
		Nickname:  author.Nickname,
		AvatarURL: avatarURL,
		Bio:       author.Bio,
	}
}

func ToAuthorMeResponse(author *models.Author) *AuthorMeResponse {
	if author == nil {
		return nil
	}

	return &AuthorMeResponse{
		ID:       author.ID,
		Username: author.Username,
		Nickname: author.Nickname,
		Bio:      author.Bio,
		Category: author.Category,
	}
}
