package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrMediaNotFound = errors.New("media not found")
)

type Media struct {
	ID        uuid.UUID
	AuthorID  uuid.UUID
	MediaType string
	MimeType  string
	MediaKey  string
	Filename  string
	SizeBytes int64
	CreatedAt time.Time
	UpdatedAt time.Time
}
