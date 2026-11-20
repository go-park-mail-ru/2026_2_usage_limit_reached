package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrProfileNotFound = errors.New("no profile found")

type Profile struct {
	UserID    uuid.UUID
	Username  string
	Nickname  string
	Email     string
	AvatarKey *string
	CreatedAt time.Time
	Status    string
	Author    *ProfileAuthor
}

type ProfileAuthor struct {
	Bio      string
	Category *string
}
