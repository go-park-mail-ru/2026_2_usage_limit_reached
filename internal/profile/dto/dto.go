package dto

import (
	"time"

	"github.com/google/uuid"
)

type PostResponse struct {
	ID          uuid.UUID  `json:"id" example:"00000000-0000-4000-8000-000000000101"`
	Title       string     `json:"title" example:"my title"`
	Body        string     `json:"body" example:"smth in body"`
	Status      string     `json:"status" example:"published"`
	PublishedAt *time.Time `json:"published_at,omitempty" example:"2026-09-01T12:00:00Z"`
}

type AuthorResponse struct {
	Bio      string `json:"bio" example:"i am author"`
	Category string `json:"category" example:"Cars"`
}

type ProfileUserResponse struct {
	ID        uuid.UUID `json:"id" example:"00000000-0000-4000-8000-000000000001"`
	Username  string    `json:"username" example:"username123"`
	Nickname  string    `json:"nickname" example:"nickname_123"`
	Email     string    `json:"email" example:"user@example.com"`
	AvatarKey string    `json:"avatar_key" example:"avatars/user-1.png"`
	CreatedAt time.Time `json:"created_at" example:"2026-08-01T12:00:00Z"`
}

type ProfileResponse struct {
	User   ProfileUserResponse `json:"user"`
	Author *AuthorResponse     `json:"author,omitempty"`
	Posts  []PostResponse      `json:"posts"`
}
