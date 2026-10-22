package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidAuthorID = errors.New("invalid author id")
	ErrPostNotFound    = errors.New("post not found")
)

type Post struct {
	ID          uuid.UUID
	AuthorID    uuid.UUID
	Title       string
	Body        string
	Status      string
	PublishedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
