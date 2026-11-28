package models

import (
	"errors"

	"github.com/google/uuid"
)

var ErrAuthorNotFound = errors.New("author not found")

type Author struct {
	ID        uuid.UUID
	Username  string
	Nickname  string
	AvatarKey *string
	Bio       string
	Category  *string
}
