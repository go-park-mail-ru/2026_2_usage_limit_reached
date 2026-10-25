package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidAuthorID = errors.New("invalid author id")
	ErrForbidden       = errors.New("forbidden")
	ErrPostNotFound    = errors.New("post not found")
	ErrPostConflict    = errors.New("post was already updated")
	ErrPostValidation  = errors.New("invalid post fields")
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
