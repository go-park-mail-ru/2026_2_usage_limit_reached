package models

import (
	"time"

	"github.com/google/uuid"
)

type PostLike struct {
	ID        uuid.UUID
	PostID    uuid.UUID
	UserID    uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}
