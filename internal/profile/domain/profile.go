package domain

import (
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/models"
	"github.com/google/uuid"
)

type Author struct {
	ID              uuid.UUID
	Bio             string
	Category        string
	PayoutProvider  string
	PayoutAccountID string
	CreatedAt       time.Time
}

type PostStatus string

const (
	PostStatusDraft     PostStatus = "draft"
	PostStatusPublished PostStatus = "published"
)

type Post struct {
	ID          uuid.UUID
	AuthorID    uuid.UUID
	Title       string
	Body        string
	Status      PostStatus
	PublishedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Profile struct {
	User   models.User
	Author *Author
	Posts  []Post
}
