package dto

import (
	"time"

	"github.com/google/uuid"
)

type PostResponse struct {
	ID          uuid.UUID  `json:"id"`
	Title       string     `json:"title"`
	Body        string     `json:"body"`
	Status      string     `json:"status"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
}

type AuthorResponse struct {
	Bio      string `json:"bio"`
	Category string `json:"category"`
}

type UserResponse struct {
	ID        uuid.UUID `json:"id"`
	Username  string    `json:"username"`
	Nickname  string    `json:"nickname"`
	Email     string    `json:"email"`
	AvatarKey string    `json:"avatar_key"`
	CreatedAt time.Time `json:"created_at"`
}

type ProfileResponse struct {
	User   UserResponse    `json:"user"`
	Author *AuthorResponse `json:"author,omitempty"`
	Posts  []PostResponse  `json:"posts"`
}
