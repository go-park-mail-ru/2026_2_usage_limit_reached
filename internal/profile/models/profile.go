package models

import (
	"errors"

	"github.com/google/uuid"
)

var (
	ErrProfileNotFound   = errors.New("no profile found")
	ErrProfileConflict   = errors.New("profile data already in use")
	ErrProfileValidation = errors.New("invalid profile fields")
	ErrNotAuthor         = errors.New("user is not an author")
)

type ProfileUpdate struct {
	UserID   uuid.UUID
	Username string
	Nickname string
	Email    string
	Author   *AuthorUpdate
}

type AuthorUpdate struct {
	Bio             string
	Category        *string
	PayoutProvider  *string
	PayoutAccountID *string
}
