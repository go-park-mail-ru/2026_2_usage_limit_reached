package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrProfileNotFound = errors.New("no profile found")

type ProfileReadModel struct {
	UserID    uuid.UUID
	Username  string
	Nickname  string
	Email     string
	AvatarKey *string
	CreatedAt time.Time
	Status    string
	Author    *ProfileAuthorReadModel
}

type ProfileAuthorReadModel struct {
	Bio      string
	Category *string
}
