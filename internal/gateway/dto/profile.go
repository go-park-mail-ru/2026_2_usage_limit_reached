package dto

import (
	"time"

	"github.com/google/uuid"
)

type AuthorResponse struct {
	Bio      string  `json:"bio" example:"i am author"`
	Category *string `json:"category,omitempty" example:"Cars"`
}

type PublicProfileUserResponse struct {
	ID        uuid.UUID `json:"id" example:"00000000-0000-4000-8000-000000000001"`
	Username  string    `json:"username" example:"username123"`
	Nickname  string    `json:"nickname" example:"nickname_123"`
	AvatarURL *string   `json:"avatar_url,omitempty" example:"/media/avatars/00000000-0000-4000-8000-000000000001/00000000-0000-4000-8000-000000000201.png"`
	CreatedAt time.Time `json:"created_at" example:"2026-08-01T12:00:00Z"`
}

type ProfileUserResponse struct {
	PublicProfileUserResponse
	Email string `json:"email" example:"user@example.com"`
}

type ProfileResponse struct {
	User   ProfileUserResponse `json:"user"`
	Author *AuthorResponse     `json:"author,omitempty"`
}

type PublicProfileResponse struct {
	User   PublicProfileUserResponse `json:"user"`
	Author *AuthorResponse           `json:"author,omitempty"`
}

type UpdateProfileRequest struct {
	Nickname string               `json:"nickname" example:"nickname_123"`
	Author   *UpdateAuthorRequest `json:"author,omitempty"`
}

type UpdateAuthorRequest struct {
	Bio      string  `json:"bio" example:"i am author"`
	Category *string `json:"category" example:"Cars"`
}

type AvatarResponse struct {
	AvatarURL string `json:"avatar_url" example:"/media/avatars/00000000-0000-4000-8000-000000000001/00000000-0000-4000-8000-000000000201.png"`
}
