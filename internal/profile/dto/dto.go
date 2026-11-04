package dto

import (
	"fmt"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/google/uuid"
)

type AuthorResponse struct {
	Bio string `json:"bio" example:"i am author"`
	// null, если категории нет
	Category *string `json:"category" example:"Cars"`
}

type ProfileUserResponse struct {
	PublicProfileUserResponse
	Email string `json:"email" example:"user@example.com"`
}

type ProfileResponse struct {
	User ProfileUserResponse `json:"user"`
	// Нет, если пользователь не автор
	Author *AuthorResponse `json:"author,omitempty"`
}

type PublicProfileUserResponse struct {
	ID       uuid.UUID `json:"id" example:"00000000-0000-4000-8000-000000000001"`
	Username string    `json:"username" example:"username123"`
	Nickname string    `json:"nickname" example:"nickname_123"`
	// Нет, если аватара нет
	AvatarURL *string   `json:"avatar_url,omitempty" example:"http://localhost:8333/media/avatars/00000000-0000-4000-8000-000000000001/00000000-0000-4000-8000-000000000201.png"`
	CreatedAt time.Time `json:"created_at" example:"2026-08-01T12:00:00Z"`
}

type PublicProfileResponse struct {
	User PublicProfileUserResponse `json:"user"`
	// Нет, если пользователь не автор
	Author *AuthorResponse `json:"author,omitempty"`
}

type UpdateProfileRequest struct {
	Nickname string `json:"nickname" example:"nickname_123"`
	// Отправляет только автор. Если блока нет, данные автора не меняются
	Author *UpdateAuthorRequest `json:"author,omitempty"`
}

type UpdateAuthorRequest struct {
	Bio      string  `json:"bio" example:"i am author"`
	Category *string `json:"category" example:"Cars"`
}

func (r UpdateProfileRequest) Validate(v *validator.Validator) error {
	if !v.IsValidNickname(r.Nickname) {
		return fmt.Errorf("%w: invalid nickname", validator.ErrValidation)
	}

	if r.Author == nil {
		return nil
	}

	if !v.IsValidBio(r.Author.Bio) {
		return fmt.Errorf("%w: invalid bio", validator.ErrValidation)
	}

	if r.Author.Category != nil && !v.IsValidCategory(*r.Author.Category) {
		return fmt.Errorf("%w: invalid category", validator.ErrValidation)
	}

	return nil
}
