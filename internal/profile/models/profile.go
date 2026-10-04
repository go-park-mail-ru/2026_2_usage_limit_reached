package models

import (
	"time"

	"github.com/google/uuid"
)

type PostStatus string

const (
	PostStatusDraft     PostStatus = "draft"
	PostStatusPublished PostStatus = "published"
)

type Author struct {
	Bio      string
	Category string
}

type Post struct {
	ID          uuid.UUID
	AuthorID    uuid.UUID
	Title       string
	Body        string
	Status      PostStatus
	PublishedAt *time.Time
	CreatedAt   time.Time
}
