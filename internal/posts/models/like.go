package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrLikeNotFound = errors.New("like not found")

type PostLike struct {
	ID        uuid.UUID
	PostID    uuid.UUID
	UserID    uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}
